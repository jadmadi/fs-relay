package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fs-relay/internal/wire"

	"github.com/coder/websocket"
)

// Socket is one websocket to /ws/<namespace>. Calls overlap; the id matches each reply.
type Socket struct {
	conn     *websocket.Conn
	endpoint string
	url      string
	ns       string
	token    string
	closed   atomic.Bool
	mu       sync.Mutex
	wmu      sync.Mutex
	next     uint64
	waiters  map[uint64]chan wire.Reply
	evicts   chan wire.Change
	dead     chan struct{}
	stopped  chan struct{}
	err      error
}

// Dial opens the websocket. Close stops the reader.
func Dial(ctx context.Context, baseURL, ns, token string) (*Socket, error) {
	if err := wire.CheckNS(ns); err != nil {
		return nil, err
	}
	if err := wire.CheckToken(token); err != nil {
		return nil, err
	}
	endpoint, err := wsEndpoint(baseURL, ns)
	if err != nil {
		return nil, err
	}
	conn, err := dialConn(ctx, endpoint, token)
	if err != nil {
		return nil, err
	}
	s := &Socket{
		conn:     conn,
		endpoint: endpoint,
		url:      baseURL,
		ns:       ns,
		token:    token,
		waiters:  map[uint64]chan wire.Reply{},
		evicts:   make(chan wire.Change, 32),
		dead:     make(chan struct{}),
		stopped:  make(chan struct{}),
	}
	go s.serve()
	return s, nil
}

// Events receives cache evictions pushed on this socket.
func (s *Socket) Events() <-chan wire.Change {
	return s.evicts
}

// Close closes the socket and waits for the reader to finish.
func (s *Socket) Close() error {
	s.closed.Store(true)
	s.kill()
	conn := s.takeConn()
	var err error
	if conn != nil {
		err = conn.Close(websocket.StatusNormalClosure, "")
	}
	<-s.stopped
	return err
}

// Call sends one method and waits for its reply. A filesystem failure is *wire.Error.
func (s *Socket) Call(ctx context.Context, method string, p wire.Params) (wire.Result, error) {
	if p.Namespace == "" {
		p.Namespace = s.ns
	}
	id, ch, dead, err := s.begin()
	if err != nil {
		return wire.Result{}, err
	}
	defer s.end(id)
	body, err := wire.MarshalCall(id, method, p)
	if err != nil {
		return wire.Result{}, err
	}
	if err := s.send(ctx, body); err != nil {
		return wire.Result{}, fmt.Errorf("ws %s: %w", method, err)
	}
	return s.wait(ctx, method, ch, dead)
}

func wsEndpoint(base, ns string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("url: %w", err)
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	default:
		return "", fmt.Errorf("url scheme %q", u.Scheme)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/ws/" + url.PathEscape(ns)
	return u.String(), nil
}

func (s *Socket) begin() (uint64, <-chan wire.Reply, <-chan struct{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return 0, nil, nil, s.err
	}
	s.next++
	ch := make(chan wire.Reply, 2)
	s.waiters[s.next] = ch
	return s.next, ch, s.dead, nil
}

func (s *Socket) end(id uint64) {
	s.mu.Lock()
	delete(s.waiters, id)
	s.mu.Unlock()
}

func (s *Socket) send(ctx context.Context, b []byte) error {
	conn := s.current()
	if conn == nil {
		return errors.New("connection lost")
	}
	s.wmu.Lock()
	defer s.wmu.Unlock()
	return conn.Write(ctx, websocket.MessageBinary, b)
}

func (s *Socket) wait(ctx context.Context, method string, ch <-chan wire.Reply, dead <-chan struct{}) (wire.Result, error) {
	for {
		select {
		case <-ctx.Done():
			return wire.Result{}, ctx.Err()
		case <-dead:
			return wire.Result{}, fmt.Errorf("ws %s: connection lost", method)
		case rp := <-ch:
			res, err, more := finish(rp)
			if more {
				continue
			}
			return res, err
		}
	}
}

func finish(rp wire.Reply) (wire.Result, error, bool) {
	if rp.Fault != "" {
		return wire.Result{}, errors.New(rp.Fault), false
	}
	if rp.Err != nil {
		return wire.Result{}, rp.Err, false
	}
	if rp.Result != nil {
		return *rp.Result, nil, false
	}
	if rp.Pending != "" {
		return wire.Result{}, nil, true
	}
	return wire.Result{}, errors.New("empty reply"), false
}

func (s *Socket) serve() {
	defer close(s.stopped)
	drops := 0
	again := false
	for !s.closed.Load() {
		if drops > 0 && !waitRejoin(context.Background()) {
			return
		}
		if s.closed.Load() {
			return
		}
		if s.current() == nil {
			conn, err := dialConn(context.Background(), s.endpoint, s.token)
			if err != nil {
				drops = nextDrops(drops, false, 0)
				continue
			}
			s.setConn(conn)
		}
		started := time.Now()
		established, err := s.loop(again)
		s.dropConn()
		if s.closed.Load() {
			return
		}
		logLost(s.url)
		s.kill()
		again = true
		drops = nextDrops(drops, established, time.Since(started))
		if err == nil {
			return
		}
	}
}

func dialConn(ctx context.Context, endpoint, token string) (*websocket.Conn, error) {
	conn, resp, err := websocket.Dial(ctx, endpoint, &websocket.DialOptions{
		CompressionMode: websocket.CompressionDisabled,
		HTTPHeader: http.Header{
			wire.TokenHeader: {wire.TokenScheme + " " + token},
		},
	})
	if err != nil {
		return nil, dialErr(endpoint, resp, err)
	}
	conn.SetReadLimit(wire.MaxBody)
	return conn, nil
}

func dialErr(endpoint string, resp *http.Response, err error) error {
	if resp == nil || resp.Body == nil || resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("dial %s: %w", endpoint, err)
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if why := tokenWhy(raw); why != "" {
		return fmt.Errorf("dial %s: %s", endpoint, why)
	}
	return fmt.Errorf("dial %s: %w", endpoint, err)
}

func (s *Socket) loop(again bool) (bool, error) {
	named := false
	for {
		conn := s.current()
		if conn == nil {
			return named, errors.New("connection lost")
		}
		readCtx, cancel := context.WithTimeout(context.Background(), wire.Stall)
		_, data, err := conn.Read(readCtx)
		cancel()
		if err != nil {
			return named, err
		}
		rp, err := wire.DecodeReply(data)
		if err != nil {
			return named, err
		}
		if id, ok := sessionID(rp); ok {
			if !named {
				named = true
				noteSession(s.url, id, again)
			}
			continue
		}
		if rp.Evict != nil {
			select {
			case s.evicts <- *rp.Evict:
			case <-s.dead:
				return named, errors.New("connection lost")
			}
			continue
		}
		s.deliver(rp)
	}
}

func noteSession(url, id string, again bool) {
	if again {
		logReconnected(url, id)
		return
	}
	logConnected(url, id)
}

func sessionID(rp wire.Reply) (string, bool) {
	if rp.Client == "" || rp.ID != 0 || rp.Evict != nil || rp.Result != nil || rp.Err != nil {
		return "", false
	}
	if rp.Fault != "" || rp.Pending != "" || rp.Release != "" {
		return "", false
	}
	return rp.Client, true
}

func (s *Socket) deliver(rp wire.Reply) {
	s.mu.Lock()
	ch := s.waiters[rp.ID]
	s.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- rp:
	default:
	}
}

func (s *Socket) current() *websocket.Conn {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn
}

func (s *Socket) setConn(conn *websocket.Conn) {
	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()
}

func (s *Socket) takeConn() *websocket.Conn {
	s.mu.Lock()
	defer s.mu.Unlock()
	conn := s.conn
	s.conn = nil
	return conn
}

func (s *Socket) dropConn() {
	conn := s.takeConn()
	if conn != nil {
		_ = conn.Close(websocket.StatusGoingAway, "")
	}
}

func (s *Socket) kill() {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.dead:
	default:
		close(s.dead)
	}
	s.dead = make(chan struct{})
}
