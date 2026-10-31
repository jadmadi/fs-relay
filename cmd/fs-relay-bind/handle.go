package main

import (
	"context"
	"syscall"

	"fs-relay/internal/wire"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

type handle struct {
	c     caller
	n     *node
	fh    uint64
	ino   uint64
	local bool
	wb    pending
}

var (
	_ fs.FileReader    = (*handle)(nil)
	_ fs.FileWriter    = (*handle)(nil)
	_ fs.FileFlusher   = (*handle)(nil)
	_ fs.FileFsyncer   = (*handle)(nil)
	_ fs.FileReleaser  = (*handle)(nil)
	_ fs.FileGetattrer = (*handle)(nil)
	_ fs.FileSetattrer = (*handle)(nil)
)

func (h *handle) call(ctx context.Context, method string, p wire.Params) (wire.Result, syscall.Errno) {
	if h.fh != 0 {
		p.Fh = h.fh
	}
	if p.Inode == 0 {
		p.Inode = h.ino
	}
	res, err := h.c.Call(ctx, method, p)
	return res, toErrno(err)
}

func (h *handle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	if h.local && h.n != nil {
		if a, ok := h.n.cached(); ok && a.Size == 0 {
			return fuse.ReadResultData(nil), 0
		}
		if h.n.k != nil {
			if b, ok := h.n.k.blob(h.ino, off, len(dest)); ok {
				return fuse.ReadResultData(b), 0
			}
		}
	}
	if h.n != nil {
		if errno := h.n.drain(ctx); errno != 0 {
			return nil, errno
		}
	}
	min, max := h.rawWindow()
	switch route(false, len(dest), min, max, h.spliceMax()) {
	case viaRawPipe:
		if res, errno, ok := h.readPipe(ctx, dest, off); ok {
			return res, errno
		}
	case viaRawBuf:
		if res, errno, ok := h.readBuf(ctx, dest, off); ok {
			return res, errno
		}
	}
	res, errno := h.call(ctx, wire.Read, wire.Params{Offset: off, Size: uint64(len(dest))})
	if errno != 0 {
		return nil, errno
	}
	return fuse.ReadResultData(res.Data), 0
}

func (h *handle) Flush(ctx context.Context) syscall.Errno {
	h.wb.mu.Lock()
	defer h.wb.mu.Unlock()
	return h.takeErr()
}

func (h *handle) Fsync(ctx context.Context, _ uint32) syscall.Errno {
	h.wb.mu.Lock()
	errno := h.flushLocked(ctx)
	sticky := h.takeErr()
	h.wb.mu.Unlock()
	if errno != 0 {
		return errno
	}
	if sticky != 0 {
		return sticky
	}
	if h.local || h.fh == 0 {
		return 0
	}
	_, errno = h.call(ctx, wire.Fsync, wire.Params{})
	return errno
}

func (h *handle) Release(ctx context.Context) syscall.Errno {
	h.wb.mu.Lock()
	sticky := h.takeErr()
	off := h.wb.off
	var data []byte
	if len(h.wb.buf) > 0 {
		data = append([]byte(nil), h.wb.buf...)
		h.wb.buf = h.wb.buf[:0]
	}
	h.clearDirty()
	h.wb.mu.Unlock()
	h.releaseBuf()
	if h.local || h.fh == 0 {
		if sticky != 0 {
			return sticky
		}
		return 0
	}
	parent, name := uint64(0), ""
	if len(data) > 0 && h.n != nil {
		parent, name = h.n.ownPlace()
	}
	_, errno := h.call(ctx, wire.Release, wire.Params{Inode: h.ino, Offset: off, Data: data})
	if errno != 0 && h.n != nil {
		h.n.disownPlace(parent, name)
	}
	if sticky != 0 {
		return sticky
	}
	return errno
}

func (h *handle) getattr(ctx context.Context, out *fuse.AttrOut) syscall.Errno {
	if h.n != nil {
		if a, ok := h.n.cached(); ok {
			h.n.fill(&out.Attr, a)
			return 0
		}
	}
	res, errno := h.call(ctx, wire.Getattr, wire.Params{})
	if errno != 0 {
		return errno
	}
	if h.n != nil {
		h.n.remember(*res.Attr)
		h.n.fill(&out.Attr, *res.Attr)
		return 0
	}
	fillAttr(&out.Attr, *res.Attr)
	return 0
}

func (h *handle) Getattr(ctx context.Context, out *fuse.AttrOut) syscall.Errno {
	return h.getattr(ctx, out)
}

func (h *handle) setattr(ctx context.Context, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	if h.n != nil {
		if errno := h.n.drain(ctx); errno != 0 {
			return errno
		}
	}
	p := setattrParams(in)
	parent, name := uint64(0), ""
	if h.n != nil {
		parent, name = h.n.ownPlace()
	}
	res, errno := h.call(ctx, wire.Setattr, p)
	if errno != 0 {
		if h.n != nil {
			h.n.disownPlace(parent, name)
		}
		return errno
	}
	if h.n != nil {
		h.n.remember(*res.Attr)
		h.n.fill(&out.Attr, *res.Attr)
		return 0
	}
	fillAttr(&out.Attr, *res.Attr)
	return 0
}

func (h *handle) Setattr(ctx context.Context, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	return h.setattr(ctx, in, out)
}

// dirHandle lists a directory. go-fuse only reads the handle, so the
// listing has to come from Readdirent. A handle that only releases
// makes the kernel see an empty directory.
type dirHandle struct {
	parent *node
	c      caller
	ino    uint64
	ents   []fuse.DirEntry
	attrs  map[string]wire.Attr
	i      int
	loaded bool
}

var (
	_ fs.FileReaddirenter = (*dirHandle)(nil)
	_ fs.FileLookuper     = (*dirHandle)(nil)
	_ fs.FileReleasedirer = (*dirHandle)(nil)
	_ fs.FileSeekdirer    = (*dirHandle)(nil)
)

func (d *dirHandle) Readdirent(ctx context.Context) (*fuse.DirEntry, syscall.Errno) {
	if !d.loaded {
		if errno := d.load(ctx); errno != 0 {
			return nil, errno
		}
	}
	if d.i >= len(d.ents) {
		return nil, 0
	}
	de := d.ents[d.i]
	d.i++
	return &de, 0
}

func (d *dirHandle) load(ctx context.Context) syscall.Errno {
	if d.parent != nil && d.parent.k != nil {
		if ents, ok := d.parent.k.dir(d.ino); ok {
			d.store(ents)
			d.prefetch(ctx, ents)
			d.prefetchDirs(ctx, ents)
			return 0
		}
	}
	res, err := d.c.Call(ctx, wire.Readdir, wire.Params{Inode: d.ino})
	if err != nil {
		return toErrno(err)
	}
	d.store(res.Entries)
	d.prefetch(ctx, res.Entries)
	d.prefetchDirs(ctx, res.Entries)
	return 0
}

func (d *dirHandle) store(entries []wire.Dirent) {
	d.ents = make([]fuse.DirEntry, len(entries))
	d.attrs = make(map[string]wire.Attr, len(entries))
	for i, e := range entries {
		d.ents[i] = fuse.DirEntry{Mode: e.Mode, Name: e.Name, Ino: e.Ino}
		d.attrs[e.Name] = e.Attr
	}
	d.loaded = true
}

func (d *dirHandle) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	a, ok := d.attrs[name]
	if !ok {
		return d.parent.Lookup(ctx, name, out)
	}
	if child := d.parent.GetChild(name); child != nil {
		if cn, ok := child.Operations().(*node); ok {
			cn.remember(a)
			cn.fill(&out.Attr, a)
		} else {
			fillAttr(&out.Attr, a)
		}
		return child, 0
	}
	_, ino := d.parent.spawn(ctx, a, out)
	return ino, 0
}

func (d *dirHandle) Seekdir(_ context.Context, off uint64) syscall.Errno {
	if off != 0 {
		return syscall.ENOTSUP
	}
	d.i = 0
	return 0
}

func (d *dirHandle) Releasedir(context.Context, uint32) {}
