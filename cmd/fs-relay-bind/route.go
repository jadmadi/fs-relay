package main

import (
	"context"
	"errors"
	"syscall"

	"fs-relay/internal/client"
	"fs-relay/internal/wire"

	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hanwen/go-fuse/v2/splice"
)

type path int

const (
	viaEnvelope path = iota
	viaRawPipe
	viaRawBuf
	viaRawWrite
)

// route picks the connection for one read or write of n bytes.
// min and max are the server's raw window. min <= 0 means the raw path is off.
func route(write bool, n, min, max, spliceMax int) path {
	if min <= 0 || n < min || n > max {
		return viaEnvelope
	}
	if write {
		return viaRawWrite
	}
	if spliceMax > 0 && n <= spliceMax {
		return viaRawPipe
	}
	return viaRawBuf
}

func spliceCeiling() int {
	if !splice.Resizable() {
		return 0
	}
	n := splice.MaxPipeSize() - wire.RawSpare
	if n < 0 {
		return 0
	}
	return n
}

func (h *handle) rawWindow() (int, int) {
	raw := h.rawConn()
	if raw == nil {
		return 0, 0
	}
	return raw.Min(), raw.Max()
}

func (h *handle) rawConn() *client.Raw {
	if h.n == nil {
		return nil
	}
	return h.n.r
}

func (h *handle) spliceMax() int {
	if h.n == nil {
		return 0
	}
	return h.n.spliceMax
}

func isTransport(err error) bool {
	var w *wire.Error
	return err != nil && !errors.As(err, &w)
}

func (h *handle) readPipe(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno, bool) {
	pair, err := splice.Get()
	if err != nil {
		return nil, 0, false
	}
	if err := pair.Grow(len(dest) + wire.RawSpare); err != nil {
		splice.Done(pair)
		return nil, 0, false
	}
	m, err := h.n.r.ReadToPipe(ctx, h.ino, h.fh, off, len(dest), int(pair.WriteFd()))
	if err != nil {
		splice.Drop(pair)
		if isTransport(err) {
			return nil, 0, false
		}
		return nil, toErrno(err), true
	}
	if m == 0 {
		splice.Done(pair)
		return fuse.ReadResultData(nil), 0, true
	}
	return fuse.ReadResultPipe(pair, m), 0, true
}

func (h *handle) readBuf(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno, bool) {
	m, err := h.n.r.ReadInto(ctx, h.ino, h.fh, off, dest)
	if err != nil {
		if isTransport(err) {
			return nil, 0, false
		}
		return nil, toErrno(err), true
	}
	return fuse.ReadResultData(dest[:m]), 0, true
}
