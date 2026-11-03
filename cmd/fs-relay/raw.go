package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"syscall"
	"time"

	"fs-relay/internal/local"
	"fs-relay/internal/wire"

	"golang.org/x/sys/unix"
)

type rawSet struct {
	mu   sync.Mutex
	live map[net.Conn]struct{}
}

func newRawSet() *rawSet {
	return &rawSet{live: map[net.Conn]struct{}{}}
}

func (s *rawSet) add(c net.Conn) {
	s.mu.Lock()
	s.live[c] = struct{}{}
	s.mu.Unlock()
}

func (s *rawSet) remove(c net.Conn) {
	s.mu.Lock()
	delete(s.live, c)
	s.mu.Unlock()
}

func (s *rawSet) closeAll() {
	s.mu.Lock()
	all := s.live
	s.live = map[net.Conn]struct{}{}
	s.mu.Unlock()
	for c := range all {
		c.Close()
	}
}

func rawLimits(tree *local.Tree) (int, int) {
	blk, err := tree.BlockSize()
	if err != nil || blk <= 0 {
		blk = 4096
	}
	return wire.RawLimits(blk, local.PipeCapacity(wire.MaxWrite))
}

func serveRaw(h *hub, tree *local.Tree, set *rawSet, min, max int, w http.ResponseWriter, r *http.Request) {
	ns := r.PathValue("ns")
	if err := wire.CheckNS(ns); err != nil {
		http.Error(w, "bad namespace", http.StatusBadRequest)
		return
	}
	if err := tree.OpenNamespace(ns); err != nil {
		writeNamespaceErr(w, ns, err)
		return
	}
	if r.Header.Get("Upgrade") != wire.RawProtocol {
		http.Error(w, "upgrade", http.StatusBadRequest)
		return
	}
	conn, rc, ok := hijackRaw(w, min, max)
	if !ok {
		return
	}
	defer conn.Close()
	pipe, err := local.NewPipe(max + wire.RawSpare)
	if err != nil {
		return
	}
	defer pipe.Close()
	set.add(conn)
	defer set.remove(conn)
	rawLoop(h, tree, ns, conn, rc, pipe, max)
}

func hijackRaw(w http.ResponseWriter, min, max int) (net.Conn, syscall.RawConn, bool) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return nil, nil, false
	}
	conn, brw, err := hj.Hijack()
	if err != nil {
		return nil, nil, false
	}
	if brw.Reader.Buffered() != 0 {
		conn.Close()
		return nil, nil, false
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		conn.Close()
		return nil, nil, false
	}
	sc, ok := conn.(syscall.Conn)
	if !ok {
		fmt.Fprintf(conn, "HTTP/1.1 501 Not Implemented\r\nConnection: close\r\nContent-Length: 0\r\n\r\n")
		conn.Close()
		return nil, nil, false
	}
	rc, err := sc.SyscallConn()
	if err != nil {
		conn.Close()
		return nil, nil, false
	}
	_, err = fmt.Fprintf(conn, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: %s\r\nFsr-Raw-Min: %d\r\nFsr-Raw-Max: %d\r\n\r\n", wire.RawProtocol, min, max)
	if err != nil {
		conn.Close()
		return nil, nil, false
	}
	return conn, rc, true
}

func rawLoop(h *hub, tree *local.Tree, ns string, conn net.Conn, rc syscall.RawConn, pipe *local.Pipe, max int) {
	hdr := make([]byte, wire.RawReqLen)
	for {
		if _, err := io.ReadFull(conn, hdr); err != nil {
			return
		}
		req, err := wire.ParseRawRequest(hdr)
		if err != nil || int(req.Length) > max {
			return
		}
		if rawOne(h, tree, ns, rc, pipe, req) != nil {
			return
		}
	}
}

func rawOne(h *hub, tree *local.Tree, ns string, rc syscall.RawConn, pipe *local.Pipe, req wire.RawRequest) error {
	switch req.Op {
	case wire.RawRead:
		return rawRead(tree, ns, rc, pipe, req)
	case wire.RawWrite:
		return rawWrite(h, tree, ns, rc, pipe, req)
	default:
		return wire.ErrParams
	}
}

func rawRead(tree *local.Tree, ns string, rc syscall.RawConn, pipe *local.Pipe, req wire.RawRequest) error {
	n, err := tree.SpliceRead(context.Background(), ns, req.Inode, req.Fh, req.Offset, int(req.Length), pipe)
	if err != nil {
		_ = pipe.Reset()
		return rawStatus(rc, err)
	}
	if err := rawReply(rc, 0, n, n > 0); err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	return spliceOut(rc, pipe.ReadFd(), n)
}

func rawWrite(h *hub, tree *local.Tree, ns string, rc syscall.RawConn, pipe *local.Pipe, req wire.RawRequest) error {
	if err := spliceIn(rc, pipe.WriteFd(), int(req.Length)); err != nil {
		return err
	}
	n, err := tree.SpliceWrite(context.Background(), ns, req.Fh, req.Offset, pipe, int(req.Length))
	if err != nil {
		_ = pipe.Reset()
		return rawStatus(rc, err)
	}
	publishCall(h, tree, ns, wire.Write, wire.Params{Namespace: ns, Fh: req.Fh}, nil)
	return rawReply(rc, 0, n, false)
}

func rawStatus(rc syscall.RawConn, err error) error {
	return rawReply(rc, errnoCode(err), 0, false)
}

func errnoCode(err error) syscall.Errno {
	var n syscall.Errno
	if errors.As(err, &n) && n != 0 {
		return n
	}
	return syscall.EIO
}

func rawReply(rc syscall.RawConn, errno syscall.Errno, n int, more bool) error {
	var buf [wire.RawRepLen]byte
	if err := wire.PutRawReply(buf[:], wire.RawReply{Errno: uint32(errno), Length: uint32(n)}); err != nil {
		return err
	}
	flags := 0
	if more {
		flags = unix.MSG_MORE
	}
	return sendAll(rc, buf[:], flags)
}

func sendAll(rc syscall.RawConn, b []byte, flags int) error {
	off := 0
	var opErr error
	err := rc.Write(func(fd uintptr) bool {
		for off < len(b) {
			n, e := unix.SendmsgN(int(fd), b[off:], nil, nil, flags)
			if e == syscall.EAGAIN || e == syscall.EWOULDBLOCK {
				return false
			}
			if e != nil {
				opErr = e
				return true
			}
			if n == 0 {
				opErr = io.ErrShortWrite
				return true
			}
			off += n
		}
		return true
	})
	if err != nil {
		return err
	}
	return opErr
}

func spliceIn(rc syscall.RawConn, wfd, n int) error {
	fd, err := dupRaw(rc)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	return local.SpliceMove(context.Background(), fd, wfd, n, fd, unix.POLLIN)
}

func spliceOut(rc syscall.RawConn, rfd, n int) error {
	fd, err := dupRaw(rc)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	return local.SpliceMove(context.Background(), rfd, fd, n, fd, unix.POLLOUT)
}

func dupRaw(rc syscall.RawConn) (int, error) {
	var fd int
	var opErr error
	err := rc.Control(func(s uintptr) {
		fd, opErr = unix.Dup(int(s))
	})
	if err != nil {
		return -1, err
	}
	return fd, opErr
}
