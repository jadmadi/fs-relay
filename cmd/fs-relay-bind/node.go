package main

import (
	"context"
	"sync"
	"syscall"

	"fs-relay/internal/wire"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// caller is the mount's transport. HTTP posts and the websocket both implement it.
type caller interface {
	Call(ctx context.Context, method string, p wire.Params) (wire.Result, error)
}

// node is one protocol inode. Every method is one call.
type node struct {
	fs.Inode
	c   caller
	k   *kernel
	ino uint64

	mu    sync.Mutex
	attr  wire.Attr
	known bool
}

var (
	_ fs.NodeGetattrer      = (*node)(nil)
	_ fs.NodeSetattrer      = (*node)(nil)
	_ fs.NodeLookuper       = (*node)(nil)
	_ fs.NodeOpendirHandler = (*node)(nil)
	_ fs.NodeMkdirer        = (*node)(nil)
	_ fs.NodeCreater        = (*node)(nil)
	_ fs.NodeUnlinker       = (*node)(nil)
	_ fs.NodeRmdirer        = (*node)(nil)
	_ fs.NodeRenamer        = (*node)(nil)
	_ fs.NodeSymlinker      = (*node)(nil)
	_ fs.NodeReadlinker     = (*node)(nil)
	_ fs.NodeOpener         = (*node)(nil)
	_ fs.NodeAccesser       = (*node)(nil)
	_ fs.NodeStatfser       = (*node)(nil)
	_ fs.NodeOnAdder        = (*node)(nil)
	_ fs.NodeOnForgetter    = (*node)(nil)
)

func (n *node) OnAdd(context.Context) {
	if n.k != nil {
		n.k.track(n.ino, n.EmbeddedInode())
	}
}

func (n *node) OnForget() {
	if n.k != nil {
		n.k.untrack(n.ino)
	}
}

func (n *node) call(ctx context.Context, method string, p wire.Params) (wire.Result, syscall.Errno) {
	if p.Inode == 0 {
		p.Inode = n.ino
	}
	res, err := n.c.Call(ctx, method, p)
	return res, toErrno(err)
}

func (n *node) spawn(ctx context.Context, a wire.Attr, out *fuse.EntryOut) (*node, *fs.Inode) {
	fillAttr(&out.Attr, a)
	ch := &node{c: n.c, k: n.k, ino: a.Ino}
	mode := a.Mode & syscall.S_IFMT
	ino := n.NewInode(ctx, ch, fs.StableAttr{Mode: mode, Ino: a.Ino})
	live, ok := ino.Operations().(*node)
	if !ok {
		live = ch
	}
	live.remember(a)
	return live, ino
}

func (n *node) remember(a wire.Attr) {
	n.mu.Lock()
	n.attr = a
	n.known = true
	n.mu.Unlock()
}

func (n *node) forgetAttr() {
	n.mu.Lock()
	n.known = false
	n.mu.Unlock()
}

func (n *node) cached() (wire.Attr, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.attr, n.known
}

func (n *node) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	if h, ok := f.(*handle); ok {
		return h.getattr(ctx, out)
	}
	if a, ok := n.cached(); ok {
		fillAttr(&out.Attr, a)
		return 0
	}
	res, errno := n.call(ctx, wire.Getattr, wire.Params{})
	if errno != 0 {
		return errno
	}
	n.remember(*res.Attr)
	fillAttr(&out.Attr, *res.Attr)
	return 0
}

func (n *node) Setattr(ctx context.Context, f fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	if h, ok := f.(*handle); ok {
		return h.setattr(ctx, in, out)
	}
	return n.setattr(ctx, in, out)
}

func (n *node) setattr(ctx context.Context, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	res, errno := n.call(ctx, wire.Setattr, setattrParams(in))
	if errno != 0 {
		return errno
	}
	n.remember(*res.Attr)
	fillAttr(&out.Attr, *res.Attr)
	return 0
}

func (n *node) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	if child := n.GetChild(name); child != nil {
		if cn, ok := child.Operations().(*node); ok {
			if a, hit := cn.cached(); hit {
				fillAttr(&out.Attr, a)
				return child, 0
			}
		}
	}
	res, errno := n.call(ctx, wire.Lookup, wire.Params{Name: name})
	if errno != 0 {
		return nil, errno
	}
	_, ino := n.spawn(ctx, *res.Attr, out)
	return ino, 0
}

func (n *node) OpendirHandle(context.Context, uint32) (fs.FileHandle, uint32, syscall.Errno) {
	return &dirHandle{parent: n, c: n.c, ino: n.ino}, 0, 0
}

func (n *node) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	res, errno := n.call(ctx, wire.Mkdir, wire.Params{Name: name, Mode: mode})
	if errno != 0 {
		return nil, errno
	}
	_, ino := n.spawn(ctx, *res.Attr, out)
	return ino, 0
}

func (n *node) Create(ctx context.Context, name string, flags, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	res, errno := n.call(ctx, wire.Create, wire.Params{Name: name, Mode: mode, Flags: flags})
	if errno != 0 {
		return nil, nil, 0, errno
	}
	ch, ino := n.spawn(ctx, *res.Attr, out)
	h := &handle{c: n.c, n: ch, fh: res.Fh, ino: ch.ino}
	return ino, h, fuse.FOPEN_DIRECT_IO, 0
}

func (n *node) Unlink(ctx context.Context, name string) syscall.Errno {
	_, errno := n.call(ctx, wire.Unlink, wire.Params{Name: name})
	return errno
}

func (n *node) Rmdir(ctx context.Context, name string) syscall.Errno {
	_, errno := n.call(ctx, wire.Rmdir, wire.Params{Name: name})
	return errno
}

func (n *node) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	parent, ok := newParent.(*node)
	if !ok {
		return syscall.EXDEV
	}
	_, errno := n.call(ctx, wire.Rename, wire.Params{
		NewInode: parent.ino, Name: name, NewName: newName, Flags: flags,
	})
	return errno
}

func (n *node) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	res, errno := n.call(ctx, wire.Symlink, wire.Params{Name: name, Target: target})
	if errno != 0 {
		return nil, errno
	}
	_, ino := n.spawn(ctx, *res.Attr, out)
	return ino, 0
}

func (n *node) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	res, errno := n.call(ctx, wire.Readlink, wire.Params{})
	if errno != 0 {
		return nil, errno
	}
	return []byte(res.Target), 0
}

func (n *node) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	if readOnly(flags) {
		return &handle{c: n.c, n: n, ino: n.ino, local: true}, fuse.FOPEN_KEEP_CACHE, 0
	}
	res, errno := n.call(ctx, wire.Open, wire.Params{Flags: flags})
	if errno != 0 {
		return nil, 0, errno
	}
	return &handle{c: n.c, n: n, fh: res.Fh, ino: n.ino}, fuse.FOPEN_DIRECT_IO, 0
}

func readOnly(flags uint32) bool {
	return flags&syscall.O_ACCMODE == syscall.O_RDONLY && flags&syscall.O_TRUNC == 0
}

func (n *node) Access(ctx context.Context, mask uint32) syscall.Errno {
	_, errno := n.call(ctx, wire.Access, wire.Params{Mask: mask})
	return errno
}

func (n *node) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
	res, errno := n.call(ctx, wire.Statfs, wire.Params{})
	if errno != 0 {
		return errno
	}
	out.Blocks = res.Stat.Blocks
	out.Bfree = res.Stat.Bfree
	out.Bavail = res.Stat.Bavail
	out.Files = res.Stat.Files
	out.Ffree = res.Stat.Ffree
	out.Bsize = res.Stat.Bsize
	out.NameLen = res.Stat.NameLen
	out.Frsize = res.Stat.Frsize
	return 0
}

func fillAttr(out *fuse.Attr, a wire.Attr) {
	out.Ino = a.Ino
	out.Size = a.Size
	out.Blocks = (a.Size + 511) / 512
	out.Mode = a.Mode
	out.Nlink = a.Nlink
	if out.Nlink == 0 {
		out.Nlink = 1
	}
	out.Uid = a.Uid
	out.Gid = a.Gid
	out.Atime, out.Atimensec = splitTime(a.Atime)
	out.Mtime, out.Mtimensec = splitTime(a.Mtime)
	out.Ctime, out.Ctimensec = splitTime(a.Ctime)
}

func splitTime(ns int64) (uint64, uint32) {
	if ns < 0 {
		return 0, 0
	}
	return uint64(ns / 1e9), uint32(ns % 1e9)
}

func setattrParams(in *fuse.SetAttrIn) wire.Params {
	p := wire.Params{Valid: in.Valid, Mode: in.Mode, Size: in.Size, UID: in.Uid, GID: in.Gid}
	p.Atime = int64(in.Atime)*1e9 + int64(in.Atimensec)
	p.Mtime = int64(in.Mtime)*1e9 + int64(in.Mtimensec)
	if fh, ok := in.GetFh(); ok {
		p.Fh = fh
	}
	return p
}
