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
	res, errno := h.call(ctx, wire.Read, wire.Params{Offset: off, Size: uint64(len(dest))})
	if errno != 0 {
		return nil, errno
	}
	return fuse.ReadResultData(res.Data), 0
}

func (h *handle) Write(ctx context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	res, errno := h.call(ctx, wire.Write, wire.Params{Offset: off, Data: data})
	if errno != 0 {
		return 0, errno
	}
	if h.n != nil {
		h.n.forgetAttr()
		if h.n.k != nil {
			h.n.k.forgetIno(h.ino)
		}
	}
	return res.Written, 0
}

func (h *handle) Flush(context.Context) syscall.Errno { return 0 }

func (h *handle) Fsync(ctx context.Context, _ uint32) syscall.Errno {
	if h.local {
		return 0
	}
	_, errno := h.call(ctx, wire.Fsync, wire.Params{})
	return errno
}

func (h *handle) Release(ctx context.Context) syscall.Errno {
	if h.local {
		return 0
	}
	_, errno := h.call(ctx, wire.Release, wire.Params{})
	return errno
}

func (h *handle) getattr(ctx context.Context, out *fuse.AttrOut) syscall.Errno {
	if h.n != nil {
		if a, ok := h.n.cached(); ok {
			fillAttr(&out.Attr, a)
			return 0
		}
	}
	res, errno := h.call(ctx, wire.Getattr, wire.Params{})
	if errno != 0 {
		return errno
	}
	if h.n != nil {
		h.n.remember(*res.Attr)
	}
	fillAttr(&out.Attr, *res.Attr)
	return 0
}

func (h *handle) Getattr(ctx context.Context, out *fuse.AttrOut) syscall.Errno {
	return h.getattr(ctx, out)
}

func (h *handle) setattr(ctx context.Context, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	p := setattrParams(in)
	res, errno := h.call(ctx, wire.Setattr, p)
	if errno != 0 {
		return errno
	}
	if h.n != nil {
		h.n.remember(*res.Attr)
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
	res, err := d.c.Call(ctx, wire.Readdir, wire.Params{Inode: d.ino})
	if err != nil {
		return toErrno(err)
	}
	d.ents = make([]fuse.DirEntry, len(res.Entries))
	d.attrs = make(map[string]wire.Attr, len(res.Entries))
	for i, e := range res.Entries {
		d.ents[i] = fuse.DirEntry{Mode: e.Mode, Name: e.Name, Ino: e.Ino}
		d.attrs[e.Name] = e.Attr
	}
	d.loaded = true
	d.prefetch(ctx, res.Entries)
	return 0
}

func (d *dirHandle) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	a, ok := d.attrs[name]
	if !ok {
		return d.parent.Lookup(ctx, name, out)
	}
	if child := d.parent.GetChild(name); child != nil {
		fillAttr(&out.Attr, a)
		if cn, ok := child.Operations().(*node); ok {
			cn.remember(a)
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
