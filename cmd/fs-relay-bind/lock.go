package main

import (
	"context"
	"syscall"
	"time"

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

// Setlkw tries the lock, then waits on the mount's release stream and tries
// again. A dropped stream cannot lose the wait: the release is replayed after
// a rejoin, and a one-second retry bounds anything that is missed.
func (n *node) Setlkw(ctx context.Context, f fs.FileHandle, owner uint64, lk *fuse.FileLock, flags uint32) syscall.Errno {
	for {
		released := n.c.Releases()
		_, errno := n.lock(ctx, wire.Setlkw, f, owner, lk, flags)
		if errno != syscall.EAGAIN {
			return errno
		}
		select {
		case <-released:
		case <-ctx.Done():
			return syscall.EINTR
		case <-time.After(time.Second):
		}
	}
}

func (n *node) lock(ctx context.Context, method string, f fs.FileHandle, owner uint64, lk *fuse.FileLock, flags uint32) (wire.Result, syscall.Errno) {
	p, errno := lockParams(n, f, owner, lk, flags)
	if errno != 0 {
		return wire.Result{}, errno
	}
	h, _ := f.(*handle)
	grant := method != wire.Getlk && p.Lock != wire.LockUnlock
	if h != nil && grant {
		if !h.beginOp() {
			return wire.Result{}, syscall.EINTR
		}
		defer h.endOp()
	}
	if h != nil && h.fh == 0 && grant {
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
