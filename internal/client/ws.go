package client

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"fs-relay/internal/wire"

	"github.com/coder/websocket"
)

// Socket is one websocket to /ws/<namespace>. Calls overlap; the id matches each reply.
type Socket struct {
	conn    *websocket.Conn
	url     string
	ns      string
	closed  atomic.Bool
	mu      sync.Mutex
	wmu     sync.Mutex
	once    sync.Once
	next    uint64
	waiters map[uint64]chan wire.Reply
	evicts  chan wire.Change
	done    chan struct{}
	stopped chan struct{}
	err     error
}

// Dial opens the websocket. Close stops the reader.
func Dial(ctx context.Context, baseURL, ns string) (*Socket, error) {
	if err := wire.CheckNS(ns); err != nil {
		return nil, err
	}
	endpoint, err := wsEndpoint(baseURL, ns)
	if err != nil {
		return nil, err
	}
	conn, _, err := websocket.Dial(ctx, endpoint, &websocket.DialOptions{
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", endpoint, err)
	}
	conn.SetReadLimit(wire.MaxBody)
	s := &Socket{
		conn:    conn,
		url:     baseURL,
		ns:      ns,
		waiters: map[uint64]chan wire.Reply{},
		evicts:  make(chan wire.Change, 32),
		done:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
	go s.read()
	return s, nil
}

// Events receives cache evictions pushed on this socket.
func (s *Socket) Events() <-chan wire.Change {
	return s.evicts
}

// Close closes the socket and waits for the reader to finish.
func (s *Socket) Close() error {
	s.closed.Store(true)
	err := s.conn.Close(websocket.StatusNormalClosure, "")
	<-s.stopped
	return err
}

// Call sends one method and waits for its reply. A filesystem failure is *wire.Error.
func (s *Socket) Call(ctx context.Context, method string, p wire.Params) (wire.Result, error) {
	if p.Namespace == "" {
		p.Namespace = s.ns
	}
	id, ch, err := s.begin()
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
	return s.wait(ctx, method, ch)
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

func (s *Socket) begin() (uint64, <-chan wire.Reply, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return 0, nil, s.err
	}
	s.next++
	ch := make(chan wire.Reply, 2)
	s.waiters[s.next] = ch
	return s.next, ch, nil
}

func (s *Socket) end(id uint64) {
	s.mu.Lock()
	delete(s.waiters, id)
	s.mu.Unlock()
}

func (s *Socket) send(ctx context.Context, b []byte) error {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	return s.conn.Write(ctx, websocket.MessageBinary, b)
}

func (s *Socket) wait(ctx context.Context, method string, ch <-chan wire.Reply) (wire.Result, error) {
	for {
		select {
		case <-ctx.Done():
			return wire.Result{}, ctx.Err()
		case <-s.done:
			return wire.Result{}, fmt.Errorf("ws %s: %w", method, s.cause())
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

func (s *Socket) read() {
	defer close(s.stopped)
	err := s.loop()
	if err != nil && !s.closed.Load() {
		logLost(s.url)
	}
	s.fail(err)
}

func (s *Socket) loop() error {
	named := false
	for {
		_, data, err := s.conn.Read(context.Background())
		if err != nil {
			return err
		}
		rp, err := wire.DecodeReply(data)
		if err != nil {
			return err
		}
		if id, ok := sessionID(rp); ok {
			if !named {
				named = true
				logConnected(s.url, id)
			}
			continue
		}
		if rp.Evict != nil {
			select {
			case s.evicts <- *rp.Evict:
			case <-s.done:
				return err
			}
			continue
		}
		s.deliver(rp)
	}
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

func (s *Socket) fail(err error) {
	s.once.Do(func() {
		s.mu.Lock()
		s.err = err
		s.mu.Unlock()
		close(s.done)
	})
}

func (s *Socket) cause() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}
