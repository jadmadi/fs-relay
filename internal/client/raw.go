package client

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"syscall"
	"time"

	"fs-relay/internal/wire"

	"golang.org/x/sys/unix"
)

const rawConns = 8

// Raw carries large reads and writes on upgraded connections, one transfer per connection at a time.
type Raw struct {
	host  string
	path  string
	min   int
	max   int
	idle  chan *net.TCPConn
	slots chan struct{}
	done  chan struct{}
	once  sync.Once
}

// DialRaw opens one raw connection and learns the server's size window.
func DialRaw(ctx context.Context, baseURL, ns string) (*Raw, error) {
	if err := wire.CheckNS(ns); err != nil {
		return nil, err
	}
	host, path, err := rawEndpoint(baseURL, ns)
	if err != nil {
		return nil, err
	}
	r := &Raw{
		host:  host,
		path:  path,
		idle:  make(chan *net.TCPConn, rawConns),
		slots: make(chan struct{}, rawConns),
		done:  make(chan struct{}),
	}
	c, min, max, err := r.handshake(ctx)
	if err != nil {
		return nil, err
	}
	r.min, r.max = min, max
	r.slots <- struct{}{}
	r.idle <- c
	return r, nil
}

// Min is the smallest transfer the mount should send on this path.
func (r *Raw) Min() int { return r.min }

// Max is the largest transfer one frame may carry.
func (r *Raw) Max() int { return r.max }

// Close closes idle connections. A transfer already running finishes on its own.
func (r *Raw) Close() error {
	r.once.Do(func() { close(r.done) })
	for {
		select {
		case c := <-r.idle:
			c.Close()
			<-r.slots
		default:
			return nil
		}
	}
}

// ReadToPipe splices a read reply into the write end of a pipe.
func (r *Raw) ReadToPipe(ctx context.Context, ino, fh uint64, off int64, n, pipeW int) (int, error) {
	return r.exchange(ctx, wire.RawRead, ino, fh, off, n, nil, pipeW)
}

// ReadInto reads a reply into dest. dest must hold the requested length.
func (r *Raw) ReadInto(ctx context.Context, ino, fh uint64, off int64, dest []byte) (int, error) {
	return r.exchange(ctx, wire.RawRead, ino, fh, off, len(dest), dest, 0)
}

// Write sends data at off and returns the count the server stored.
func (r *Raw) Write(ctx context.Context, fh uint64, off int64, data []byte) (int, error) {
	return r.exchange(ctx, wire.RawWrite, 0, fh, off, len(data), data, 0)
}

func (r *Raw) exchange(ctx context.Context, op uint16, ino, fh uint64, off int64, n int, buf []byte, pipeW int) (int, error) {
	c, err := r.get(ctx)
	if err != nil {
		return 0, err
	}
	stop := context.AfterFunc(ctx, func() { _ = c.SetDeadline(time.Unix(1, 0)) })
	got, err := transfer(ctx, c, op, ino, fh, off, n, buf, pipeW)
	fired := !stop()
	_ = c.SetDeadline(time.Time{})
	r.put(c, !fired && (err == nil || isWire(err)))
	return got, err
}

func transfer(ctx context.Context, c *net.TCPConn, op uint16, ino, fh uint64, off int64, n int, buf []byte, pipeW int) (int, error) {
	if err := writeReq(c, op, ino, fh, off, n, buf); err != nil {
		return 0, err
	}
	rep, err := readRep(c)
	if err != nil {
		return 0, err
	}
	if rep.Errno != 0 {
		e := syscall.Errno(rep.Errno)
		return 0, &wire.Error{Code: rep.Errno, Message: e.Error()}
	}
	if op == wire.RawWrite {
		return int(rep.Length), nil
	}
	return readBody(ctx, c, int(rep.Length), buf, pipeW)
}

func writeReq(c *net.TCPConn, op uint16, ino, fh uint64, off int64, n int, payload []byte) error {
	var hdr [wire.RawReqLen]byte
	if err := wire.PutRawRequest(hdr[:], wire.RawRequest{Op: op, Inode: ino, Fh: fh, Offset: off, Length: uint32(n)}); err != nil {
		return err
	}
	if op != wire.RawWrite {
		_, err := c.Write(hdr[:])
		return err
	}
	bufs := net.Buffers{hdr[:], payload}
	_, err := bufs.WriteTo(c)
	return err
}

func readRep(c *net.TCPConn) (wire.RawReply, error) {
	var buf [wire.RawRepLen]byte
	if _, err := io.ReadFull(c, buf[:]); err != nil {
		return wire.RawReply{}, err
	}
	return wire.ParseRawReply(buf[:])
}

func readBody(ctx context.Context, c *net.TCPConn, n int, dest []byte, pipeW int) (int, error) {
	if pipeW != 0 {
		return spliceToPipe(ctx, c, pipeW, n)
	}
	if n > len(dest) {
		return 0, fmt.Errorf("raw read: short buffer")
	}
	if _, err := io.ReadFull(c, dest[:n]); err != nil {
		return 0, err
	}
	return n, nil
}

func spliceToPipe(ctx context.Context, c *net.TCPConn, pipeW, n int) (int, error) {
	fd, err := dupFD(c)
	if err != nil {
		return 0, err
	}
	defer unix.Close(fd)
	if err := spliceMove(ctx, fd, pipeW, n, fd, unix.POLLIN); err != nil {
		return 0, err
	}
	return n, nil
}

func dupFD(c syscall.Conn) (int, error) {
	sc, err := c.SyscallConn()
	if err != nil {
		return -1, err
	}
	var fd int
	var opErr error
	err = sc.Control(func(s uintptr) {
		fd, opErr = unix.Dup(int(s))
	})
	if err != nil {
		return -1, err
	}
	return fd, opErr
}

func spliceMove(ctx context.Context, rfd, wfd, n, waitFd int, waitEvents int16) error {
	got := 0
	stall := 0
	for got < n {
		if err := ctx.Err(); err != nil {
			return err
		}
		c, err := unix.Splice(rfd, nil, wfd, nil, n-got, unix.SPLICE_F_MOVE|unix.SPLICE_F_NONBLOCK)
		if c > 0 {
			got += int(c)
			stall = 0
		}
		if got >= n {
			return nil
		}
		if err == nil {
			if c == 0 {
				return io.ErrUnexpectedEOF
			}
			continue
		}
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if !errors.Is(err, syscall.EAGAIN) && !errors.Is(err, syscall.EWOULDBLOCK) {
			return err
		}
		ready, err := waitSocket(ctx, waitFd, waitEvents)
		if err != nil {
			return err
		}
		if !ready {
			stall = 0
			continue
		}
		stall++
		if stall > 8 {
			return fmt.Errorf("splice: stalled")
		}
	}
	return nil
}

func waitSocket(ctx context.Context, fd int, ev int16) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	p := []unix.PollFd{{Fd: int32(fd), Events: ev | unix.POLLERR | unix.POLLHUP}}
	_, err := unix.Poll(p, 100)
	if errors.Is(err, syscall.EINTR) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	re := p[0].Revents
	if re&(unix.POLLIN|unix.POLLOUT) != 0 {
		return true, nil
	}
	if re&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 {
		return false, io.ErrUnexpectedEOF
	}
	return false, nil
}

func isWire(err error) bool {
	var w *wire.Error
	return errors.As(err, &w)
}

func (r *Raw) get(ctx context.Context) (*net.TCPConn, error) {
	select {
	case <-r.done:
		return nil, net.ErrClosed
	case c := <-r.idle:
		return c, nil
	default:
	}
	select {
	case <-r.done:
		return nil, net.ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	case c := <-r.idle:
		return c, nil
	case r.slots <- struct{}{}:
		c, _, _, err := r.handshake(ctx)
		if err != nil {
			<-r.slots
			return nil, err
		}
		return c, nil
	}
}

func (r *Raw) put(c *net.TCPConn, keep bool) {
	if !keep {
		c.Close()
		<-r.slots
		return
	}
	select {
	case <-r.done:
		c.Close()
		<-r.slots
	case r.idle <- c:
	}
}

func (r *Raw) handshake(ctx context.Context) (*net.TCPConn, int, int, error) {
	var d net.Dialer
	c, err := d.DialContext(ctx, "tcp", r.host)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("raw dial: %w", err)
	}
	tcp := c.(*net.TCPConn)
	if err := writeUpgrade(tcp, r.host, r.path); err != nil {
		tcp.Close()
		return nil, 0, 0, err
	}
	min, max, err := readSwitch(tcp)
	if err != nil {
		tcp.Close()
		return nil, 0, 0, err
	}
	return tcp, min, max, nil
}

func writeUpgrade(c net.Conn, host, path string) error {
	_, err := fmt.Fprintf(c, "GET %s HTTP/1.1\r\nHost: %s\r\nConnection: Upgrade\r\nUpgrade: %s\r\n\r\n", path, host, wire.RawProtocol)
	return err
}

func readSwitch(c net.Conn) (int, int, error) {
	br := bufio.NewReader(c)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("raw upgrade: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		return 0, 0, fmt.Errorf("raw upgrade: http %d", resp.StatusCode)
	}
	if br.Buffered() != 0 {
		return 0, 0, fmt.Errorf("raw upgrade: buffered body")
	}
	min, err := atoiHeader(resp, "Fsr-Raw-Min")
	if err != nil {
		return 0, 0, err
	}
	max, err := atoiHeader(resp, "Fsr-Raw-Max")
	if err != nil {
		return 0, 0, err
	}
	return min, max, nil
}

func atoiHeader(resp *http.Response, name string) (int, error) {
	var n int
	if _, err := fmt.Sscan(resp.Header.Get(name), &n); err != nil || n <= 0 {
		return 0, fmt.Errorf("raw upgrade: %s", name)
	}
	return n, nil
}

func rawEndpoint(base, ns string) (string, string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", "", fmt.Errorf("url: %w", err)
	}
	if u.Scheme != "http" || u.Host == "" {
		return "", "", fmt.Errorf("url scheme %q", u.Scheme)
	}
	return u.Host, "/raw/" + url.PathEscape(ns), nil
}
