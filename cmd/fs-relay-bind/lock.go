package main

import (
	"context"
	"syscall"

	"fs-relay/internal/wire"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

const maxOffset = int64(^uint64(0) >> 1)

var (
	_ fs.NodeGetlker  = (*node)(nil)
	_ fs.NodeSetlker  = (*node)(nil)
	_ fs.NodeSetlkwer = (*node)(nil)
)

// Getlk reports the first conflicting lock, or F_UNLCK when the range is clear.
func (n *node) Getlk(ctx context.Context, f fs.FileHandle, owner uint64, lk *fuse.FileLock, flags uint32, out *fuse.FileLock) syscall.Errno {
	res, errno := n.lock(ctx, wire.Getlk, f, owner, lk, flags)
	if errno != 0 {
		return errno
	}
	if res.Lock == nil {
		return syscall.EIO
	}
	*out = fuse.FileLock{
		Start: res.Lock.Start,
		End:   res.Lock.End,
		Typ:   res.Lock.Type,
		Pid:   res.Lock.Pid,
	}
	return 0
}

// Setlk takes a lock or fails with EAGAIN when it conflicts.
func (n *node) Setlk(ctx context.Context, f fs.FileHandle, owner uint64, lk *fuse.FileLock, flags uint32) syscall.Errno {
	_, errno := n.lock(ctx, wire.Setlk, f, owner, lk, flags)
	return errno
}

// Setlkw waits in the server until the lock can be granted or the call is canceled.
func (n *node) Setlkw(ctx context.Context, f fs.FileHandle, owner uint64, lk *fuse.FileLock, flags uint32) syscall.Errno {
	_, errno := n.lock(ctx, wire.Setlkw, f, owner, lk, flags)
	return errno
}

func (n *node) lock(ctx context.Context, method string, f fs.FileHandle, owner uint64, lk *fuse.FileLock, flags uint32) (wire.Result, syscall.Errno) {
	p, errno := lockParams(n, f, owner, lk, flags)
	if errno != 0 {
		return wire.Result{}, errno
	}
	if h, ok := f.(*handle); ok && h != nil && h.fh == 0 &&
		method != wire.Getlk && p.Lock != wire.LockUnlock {
		h.addOwner(owner)
	}
	res, err := n.c.Call(ctx, method, p)
	return res, toErrno(err)
}

func lockParams(n *node, f fs.FileHandle, owner uint64, lk *fuse.FileLock, flags uint32) (wire.Params, syscall.Errno) {
	if lk.Start > uint64(maxOffset) {
		return wire.Params{}, syscall.EINVAL
	}
	p := wire.Params{
		Inode:  n.ino,
		Owner:  owner,
		Pid:    lk.Pid,
		Lock:   lk.Typ,
		Offset: int64(lk.Start),
	}
	switch {
	case lk.End == ^uint64(0), lk.End < lk.Start:
		p.Size = 0
	default:
		p.Size = lk.End - lk.Start + 1
	}
	if flags&fuse.LK_FLOCK != 0 {
		p.Flags = wire.FlagFlock
	}
	if h, ok := f.(*handle); ok && h != nil && h.fh != 0 {
		p.Fh = h.fh
	}
	return p, 0
}
