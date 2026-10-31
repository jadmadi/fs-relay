package main

import (
	"context"
	"sync"
	"syscall"

	"fs-relay/internal/wire"

	"github.com/hanwen/go-fuse/v2/fuse"
)

// writeBehindMin is the send threshold before a raw connection exists.
// It is 32 times a 4096-byte block.
const writeBehindMin = 32 * 4096

var bufPool = sync.Pool{New: func() any { return make([]byte, 0, wire.MaxWrite) }}

// pending is contiguous bytes written to one handle and not yet sent.
// mu orders the handle's writes; it is held across the send on purpose.
type pending struct {
	mu  sync.Mutex
	off int64
	buf []byte
	err syscall.Errno
}

type wbAction int

const (
	wbAppend wbAction = iota
	wbSendThenBuf
	wbSendThenPut
	wbPut
	wbStart
)

// planWrite chooses how one write meets bytes already held.
// heldEnd is the first offset past the held bytes.
func planWrite(held int, heldEnd, off int64, n, min int) wbAction {
	if held > 0 && off == heldEnd && held+n <= wire.MaxWrite {
		return wbAppend
	}
	if held > 0 {
		if n >= min {
			return wbSendThenPut
		}
		return wbSendThenBuf
	}
	if n >= min {
		return wbPut
	}
	return wbStart
}

func takeBuf() []byte { return bufPool.Get().([]byte)[:0] }

func (h *handle) sendWrite(ctx context.Context, off int64, data []byte) (uint32, syscall.Errno) {
	min, max := h.rawWindow()
	if route(true, len(data), min, max, h.spliceMax()) == viaRawWrite {
		n, err := h.n.r.Write(ctx, h.fh, off, data)
		if err == nil {
			if n < len(data) {
				return uint32(n), syscall.EIO
			}
			return uint32(n), 0
		}
		if !isTransport(err) {
			return 0, toErrno(err)
		}
	}
	res, errno := h.call(ctx, wire.Write, wire.Params{Offset: off, Data: data})
	if errno != 0 {
		return 0, errno
	}
	if int(res.Written) < len(data) {
		return res.Written, syscall.EIO
	}
	return res.Written, 0
}

func (h *handle) Write(ctx context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	if h.fh == 0 {
		_, errno := h.sendWrite(ctx, off, data)
		return uint32(len(data)), errno
	}
	h.wb.mu.Lock()
	defer h.wb.mu.Unlock()
	if h.wb.err != 0 {
		return 0, h.wb.err
	}
	end := h.wb.off + int64(len(h.wb.buf))
	switch planWrite(len(h.wb.buf), end, off, len(data), h.writeMin()) {
	case wbAppend:
		h.appendLocked(data)
		if len(h.wb.buf) == wire.MaxWrite {
			if errno := h.flushLocked(ctx); errno != 0 {
				return 0, errno
			}
		}
	case wbSendThenBuf:
		if errno := h.flushLocked(ctx); errno != 0 {
			return 0, errno
		}
		h.startLocked(off, data)
	case wbSendThenPut:
		if errno := h.flushLocked(ctx); errno != 0 {
			return 0, errno
		}
		if errno := h.putLocked(ctx, off, data); errno != 0 {
			return 0, errno
		}
	case wbPut:
		if errno := h.putLocked(ctx, off, data); errno != 0 {
			return 0, errno
		}
	case wbStart:
		h.startLocked(off, data)
	}
	h.noteDirty()
	h.dropCache()
	return uint32(len(data)), 0
}

func (h *handle) writeMin() int {
	if h.n != nil && h.n.rawMin > 0 {
		return h.n.rawMin
	}
	return writeBehindMin
}

func (h *handle) appendLocked(data []byte) {
	if h.wb.buf == nil {
		h.wb.buf = takeBuf()
	}
	h.wb.buf = append(h.wb.buf, data...)
}

func (h *handle) startLocked(off int64, data []byte) {
	if h.wb.buf == nil {
		h.wb.buf = takeBuf()
	}
	h.wb.buf = append(h.wb.buf[:0], data...)
	h.wb.off = off
}

func (h *handle) flushLocked(ctx context.Context) syscall.Errno {
	if len(h.wb.buf) == 0 {
		return 0
	}
	errno := h.putLocked(ctx, h.wb.off, h.wb.buf)
	if errno != 0 {
		return errno
	}
	h.wb.buf = h.wb.buf[:0]
	h.clearDirty()
	return 0
}

func (h *handle) putLocked(ctx context.Context, off int64, data []byte) syscall.Errno {
	_, errno := h.sendWrite(ctx, off, data)
	if errno != 0 {
		h.fail(errno)
		return errno
	}
	return 0
}

func (h *handle) fail(errno syscall.Errno) {
	h.wb.err = errno
	if h.wb.buf != nil {
		h.wb.buf = h.wb.buf[:0]
	}
	h.clearDirty()
}

func (h *handle) drain(ctx context.Context) syscall.Errno {
	h.wb.mu.Lock()
	defer h.wb.mu.Unlock()
	if h.wb.err != 0 {
		return h.wb.err
	}
	return h.flushLocked(ctx)
}

func (h *handle) takeErr() syscall.Errno {
	e := h.wb.err
	h.wb.err = 0
	return e
}

func (n *node) drain(ctx context.Context) syscall.Errno {
	if n == nil {
		return 0
	}
	n.mu.Lock()
	handles := make([]*handle, 0, len(n.dirty))
	for h := range n.dirty {
		handles = append(handles, h)
	}
	n.mu.Unlock()
	var errno syscall.Errno
	for _, h := range handles {
		if e := h.drain(ctx); e != 0 && errno == 0 {
			errno = e
		}
	}
	return errno
}

func (h *handle) noteDirty() {
	if h.n == nil {
		return
	}
	h.n.mu.Lock()
	defer h.n.mu.Unlock()
	if len(h.wb.buf) == 0 {
		delete(h.n.dirty, h)
		return
	}
	if h.n.dirty == nil {
		h.n.dirty = map[*handle]int64{}
	}
	h.n.dirty[h] = h.wb.off + int64(len(h.wb.buf))
}

func (h *handle) clearDirty() {
	if h.n == nil {
		return
	}
	h.n.mu.Lock()
	delete(h.n.dirty, h)
	h.n.mu.Unlock()
}

func (h *handle) dropCache() {
	if h.n == nil {
		return
	}
	h.n.forgetAttr()
	if h.n.k != nil {
		h.n.k.forgetIno(h.ino)
	}
}

func (h *handle) releaseBuf() {
	h.wb.mu.Lock()
	defer h.wb.mu.Unlock()
	if h.wb.buf == nil {
		return
	}
	bufPool.Put(h.wb.buf[:0])
	h.wb.buf = nil
}

func (n *node) fill(out *fuse.Attr, a wire.Attr) {
	fillAttr(out, a)
	if n == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, end := range n.dirty {
		if end > 0 && uint64(end) > out.Size {
			out.Size = uint64(end)
		}
	}
	out.Blocks = (out.Size + 511) / 512
}
