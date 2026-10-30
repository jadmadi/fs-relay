package local

import (
	"context"
	"io"
	"os"
	"syscall"

	"fs-relay/internal/wire"
	"golang.org/x/sys/unix"
)

func create(ctx context.Context, t *Tree, p wire.Params) (wire.Result, error) {
	if err := ctx.Err(); err != nil {
		return wire.Result{}, err
	}
	if len(p.Data) > wire.MaxWrite {
		return wire.Result{}, wire.ErrParams
	}
	return t.withW(func() (wire.Result, error) {
		rel, abs, err := child(t, p.Namespace, p.Inode, p.Name)
		if err != nil {
			return wire.Result{}, wrap("create", err)
		}
		mode := os.FileMode(p.Mode & 0o777)
		if mode == 0 {
			mode = 0o644
		}
		f, err := os.OpenFile(abs, openFlags(p.Flags)|os.O_CREATE, mode)
		if err != nil {
			return wire.Result{}, wrap("create", err)
		}
		ino := t.assign(p.Namespace, rel)
		a, err := statAttr(abs, ino)
		if err != nil {
			f.Close()
			return wire.Result{}, wrap("create", err)
		}
		return wire.Result{Attr: &a, Fh: t.add(f, ino)}, nil
	})
}

func open(ctx context.Context, t *Tree, p wire.Params) (wire.Result, error) {
	if err := ctx.Err(); err != nil {
		return wire.Result{}, err
	}
	return t.withW(func() (wire.Result, error) {
		rel, err := t.relOf(p.Namespace, p.Inode)
		if err != nil {
			return wire.Result{}, wrap("open", err)
		}
		abs, err := t.abs(p.Namespace, rel)
		if err != nil {
			return wire.Result{}, wrap("open", err)
		}
		var st unix.Stat_t
		if err := unix.Lstat(abs, &st); err != nil {
			return wire.Result{}, wrap("open", err)
		}
		switch st.Mode & syscall.S_IFMT {
		case syscall.S_IFDIR:
			return wire.Result{}, wrap("open", syscall.EISDIR)
		case syscall.S_IFLNK:
			return wire.Result{}, wrap("open", syscall.EINVAL)
		}
		f, err := os.OpenFile(abs, openFlags(p.Flags), 0)
		if err != nil {
			return wire.Result{}, wrap("open", err)
		}
		return wire.Result{Fh: t.add(f, p.Inode)}, nil
	})
}

func openFlags(flags uint32) int {
	out := int(flags & syscall.O_ACCMODE)
	if flags&syscall.O_APPEND != 0 {
		out |= os.O_APPEND
	}
	if flags&syscall.O_TRUNC != 0 {
		out |= os.O_TRUNC
	}
	if flags&syscall.O_EXCL != 0 {
		out |= os.O_EXCL
	}
	if flags&syscall.O_SYNC != 0 {
		out |= os.O_SYNC
	}
	return out
}

func read(ctx context.Context, t *Tree, p wire.Params) (wire.Result, error) {
	if err := ctx.Err(); err != nil {
		return wire.Result{}, err
	}
	if p.Size > wire.MaxWrite {
		return wire.Result{}, wire.ErrParams
	}
	if p.Fh != 0 {
		return readFH(t, p)
	}
	return readInode(t, p)
}

func readFH(t *Tree, p wire.Params) (wire.Result, error) {
	h, err := t.use(p.Fh)
	if err != nil {
		return wire.Result{}, wrap("read", err)
	}
	defer t.done(h)
	return readFile(h.f, p)
}

func readInode(t *Tree, p wire.Params) (wire.Result, error) {
	rel, err := t.relOf(p.Namespace, p.Inode)
	if err != nil {
		return wire.Result{}, wrap("read", err)
	}
	abs, err := t.abs(p.Namespace, rel)
	if err != nil {
		return wire.Result{}, wrap("read", err)
	}
	f, err := os.OpenFile(abs, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return wire.Result{}, wrap("read", err)
	}
	defer f.Close()
	return readFile(f, p)
}

func reads(ctx context.Context, t *Tree, p wire.Params) (wire.Result, error) {
	if err := ctx.Err(); err != nil {
		return wire.Result{}, err
	}
	if len(p.Inodes) == 0 || len(p.Inodes) > wire.MaxReads {
		return wire.Result{}, wire.ErrParams
	}
	blobs := make([]wire.Blob, 0, len(p.Inodes))
	total := 0
	for _, ino := range p.Inodes {
		if err := ctx.Err(); err != nil {
			return wire.Result{}, err
		}
		b := readCapped(t, p.Namespace, ino)
		if b == nil {
			continue
		}
		if total+len(b) > wire.MaxWrite {
			break
		}
		total += len(b)
		blobs = append(blobs, wire.Blob{Inode: ino, Data: b})
	}
	return wire.Result{Blobs: blobs}, nil
}

func readCapped(t *Tree, ns string, ino uint64) []byte {
	rel, err := t.relOf(ns, ino)
	if err != nil {
		return nil
	}
	abs, err := t.abs(ns, rel)
	if err != nil {
		return nil
	}
	var st unix.Stat_t
	if err := unix.Lstat(abs, &st); err != nil {
		return nil
	}
	if st.Mode&syscall.S_IFMT != syscall.S_IFREG || st.Size == 0 || st.Size > int64(wire.MaxPrefetch) {
		return nil
	}
	f, err := os.OpenFile(abs, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil
	}
	defer f.Close()
	buf := make([]byte, st.Size)
	if _, err := io.ReadFull(f, buf); err != nil {
		return nil
	}
	return buf
}

func readFile(f *os.File, p wire.Params) (wire.Result, error) {
	buf := make([]byte, p.Size)
	n, err := f.ReadAt(buf, p.Offset)
	if err != nil && err != io.EOF {
		return wire.Result{}, wrap("read", err)
	}
	return wire.Result{Data: buf[:n]}, nil
}

func write(ctx context.Context, t *Tree, p wire.Params) (wire.Result, error) {
	if err := ctx.Err(); err != nil {
		return wire.Result{}, err
	}
	if len(p.Data) > wire.MaxWrite {
		return wire.Result{}, wire.ErrParams
	}
	h, err := t.use(p.Fh)
	if err != nil {
		return wire.Result{}, wrap("write", err)
	}
	defer t.done(h)
	n, err := h.f.WriteAt(p.Data, p.Offset)
	if err != nil {
		return wire.Result{}, wrap("write", err)
	}
	return wire.Result{Written: uint32(n)}, nil
}

func flush(ctx context.Context, t *Tree, p wire.Params) (wire.Result, error) {
	if err := ctx.Err(); err != nil {
		return wire.Result{}, err
	}
	h, err := t.use(p.Fh)
	if err != nil {
		return wire.Result{}, wrap("flush", err)
	}
	t.done(h)
	return wire.Result{}, nil
}

func fsync(ctx context.Context, t *Tree, p wire.Params) (wire.Result, error) {
	return syncFH(ctx, t, p, "fsync")
}

func syncFH(ctx context.Context, t *Tree, p wire.Params, op string) (wire.Result, error) {
	if err := ctx.Err(); err != nil {
		return wire.Result{}, err
	}
	h, err := t.use(p.Fh)
	if err != nil {
		return wire.Result{}, wrap(op, err)
	}
	defer t.done(h)
	if err := h.f.Sync(); err != nil {
		return wire.Result{}, wrap(op, err)
	}
	return wire.Result{}, nil
}

func release(ctx context.Context, t *Tree, p wire.Params) (wire.Result, error) {
	if err := ctx.Err(); err != nil {
		return wire.Result{}, err
	}
	if err := t.dropFH(p.Fh); err != nil {
		return wire.Result{}, wrap("release", err)
	}
	return wire.Result{}, nil
}

func (t *Tree) add(f *os.File, ino uint64) uint64 {
	t.fmu.Lock()
	defer t.fmu.Unlock()
	t.nextFH++
	id := t.nextFH
	t.files[id] = &openFile{f: f, ino: ino}
	return id
}

func (t *Tree) use(fh uint64) (*openFile, error) {
	if fh == 0 {
		return nil, wire.ErrParams
	}
	t.fmu.Lock()
	defer t.fmu.Unlock()
	h, ok := t.files[fh]
	if !ok {
		return nil, syscall.EBADF
	}
	h.refs++
	return h, nil
}

func (t *Tree) done(h *openFile) {
	t.fmu.Lock()
	h.refs--
	closeNow := h.drop && h.refs == 0
	t.fmu.Unlock()
	if closeNow {
		h.f.Close()
	}
}

func (t *Tree) dropFH(fh uint64) error {
	if fh == 0 {
		return wire.ErrParams
	}
	t.fmu.Lock()
	h, ok := t.files[fh]
	if !ok {
		t.fmu.Unlock()
		return syscall.EBADF
	}
	delete(t.files, fh)
	h.drop = true
	closeNow := h.refs == 0
	t.fmu.Unlock()
	if closeNow {
		return h.f.Close()
	}
	return nil
}
