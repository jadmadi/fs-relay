package local

import (
	"context"
	"os"
	"strconv"
	"time"

	"fs-relay/internal/wire"
	"golang.org/x/sys/unix"
)

func statAttr(abs string, ino uint64) (wire.Attr, error) {
	var st unix.Stat_t
	if err := unix.Lstat(abs, &st); err != nil {
		return wire.Attr{}, err
	}
	return attrFrom(st, ino), nil
}

func attrFrom(st unix.Stat_t, ino uint64) wire.Attr {
	return wire.Attr{
		Ino:   ino,
		Mode:  uint32(st.Mode),
		Size:  uint64(st.Size),
		Nlink: uint32(st.Nlink),
		Uid:   st.Uid,
		Gid:   st.Gid,
		Atime: st.Atim.Nano(),
		Mtime: st.Mtim.Nano(),
		Ctime: st.Ctim.Nano(),
	}
}

func (t *Tree) statRel(ns, rel string) (wire.Attr, error) {
	abs, err := t.abs(ns, rel)
	if err != nil {
		return wire.Attr{}, err
	}
	return statAttr(abs, t.assign(ns, rel))
}

func lookup(ctx context.Context, t *Tree, p wire.Params) (wire.Result, error) {
	if err := ctx.Err(); err != nil {
		return wire.Result{}, err
	}
	return t.withR(func() (wire.Result, error) {
		rel, abs, err := child(t, p.Namespace, p.Inode, p.Name)
		if err != nil {
			return wire.Result{}, wrap("lookup", err)
		}
		a, err := statAttr(abs, t.assign(p.Namespace, rel))
		if err != nil {
			return wire.Result{}, wrap("lookup", err)
		}
		return wire.Result{Attr: &a}, nil
	})
}

func getattr(ctx context.Context, t *Tree, p wire.Params) (wire.Result, error) {
	if err := ctx.Err(); err != nil {
		return wire.Result{}, err
	}
	if p.Fh != 0 {
		return getattrFH(t, p.Fh)
	}
	return t.withR(func() (wire.Result, error) {
		rel, err := t.relOf(p.Namespace, p.Inode)
		if err != nil {
			return wire.Result{}, wrap("getattr", err)
		}
		a, err := t.statRel(p.Namespace, rel)
		if err != nil {
			return wire.Result{}, wrap("getattr", err)
		}
		return wire.Result{Attr: &a}, nil
	})
}

func getattrFH(t *Tree, fh uint64) (wire.Result, error) {
	h, err := t.use(fh)
	if err != nil {
		return wire.Result{}, wrap("getattr", err)
	}
	defer t.done(h)
	var st unix.Stat_t
	if err := unix.Fstat(int(h.f.Fd()), &st); err != nil {
		return wire.Result{}, wrap("getattr", err)
	}
	a := attrFrom(st, h.ino)
	return wire.Result{Attr: &a}, nil
}

func access(ctx context.Context, t *Tree, p wire.Params) (wire.Result, error) {
	if err := ctx.Err(); err != nil {
		return wire.Result{}, err
	}
	return t.withR(func() (wire.Result, error) {
		rel, err := t.relOf(p.Namespace, p.Inode)
		if err != nil {
			return wire.Result{}, wrap("access", err)
		}
		abs, err := t.abs(p.Namespace, rel)
		if err != nil {
			return wire.Result{}, wrap("access", err)
		}
		if err := unix.Faccessat(unix.AT_FDCWD, abs, p.Mask, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			return wire.Result{}, wrap("access", err)
		}
		return wire.Result{}, nil
	})
}

func statfs(ctx context.Context, t *Tree, p wire.Params) (wire.Result, error) {
	if err := ctx.Err(); err != nil {
		return wire.Result{}, err
	}
	return t.withR(func() (wire.Result, error) {
		if _, err := t.relOf(p.Namespace, p.Inode); err != nil {
			return wire.Result{}, wrap("statfs", err)
		}
		abs, err := t.abs(p.Namespace, "")
		if err != nil {
			return wire.Result{}, wrap("statfs", err)
		}
		var st unix.Statfs_t
		if err := unix.Statfs(abs, &st); err != nil {
			return wire.Result{}, wrap("statfs", err)
		}
		s := &wire.FSStat{
			Blocks: st.Blocks, Bfree: st.Bfree, Bavail: st.Bavail,
			Files: st.Files, Ffree: st.Ffree,
			Bsize: uint32(st.Bsize), NameLen: uint32(st.Namelen), Frsize: uint32(st.Frsize),
		}
		return wire.Result{Stat: s}, nil
	})
}

func setattr(ctx context.Context, t *Tree, p wire.Params) (wire.Result, error) {
	if err := ctx.Err(); err != nil {
		return wire.Result{}, err
	}
	if p.Fh != 0 {
		return setattrFH(t, p)
	}
	return t.withW(func() (wire.Result, error) {
		rel, err := t.relOf(p.Namespace, p.Inode)
		if err != nil {
			return wire.Result{}, wrap("setattr", err)
		}
		abs, err := t.abs(p.Namespace, rel)
		if err != nil {
			return wire.Result{}, wrap("setattr", err)
		}
		if err := applyPath(abs, p); err != nil {
			return wire.Result{}, wrap("setattr", err)
		}
		a, err := t.statRel(p.Namespace, rel)
		if err != nil {
			return wire.Result{}, wrap("setattr", err)
		}
		return wire.Result{Attr: &a}, nil
	})
}

func applyPath(abs string, p wire.Params) error {
	if p.Valid&wire.SetMode != 0 {
		if err := unix.Fchmodat(unix.AT_FDCWD, abs, p.Mode&0o777, 0); err != nil {
			return err
		}
	}
	if p.Valid&(wire.SetUID|wire.SetGID) != 0 {
		uid, gid := -1, -1
		if p.Valid&wire.SetUID != 0 {
			uid = int(p.UID)
		}
		if p.Valid&wire.SetGID != 0 {
			gid = int(p.GID)
		}
		if err := unix.Lchown(abs, uid, gid); err != nil {
			return err
		}
	}
	if p.Valid&wire.SetSize != 0 {
		if err := os.Truncate(abs, int64(p.Size)); err != nil {
			return err
		}
	}
	return applyTimes(abs, p, unix.AT_SYMLINK_NOFOLLOW)
}

func applyTimes(abs string, p wire.Params, flags int) error {
	at, mt, ok := timePair(p)
	if !ok {
		return nil
	}
	return unix.UtimesNanoAt(unix.AT_FDCWD, abs, []unix.Timespec{at, mt}, flags)
}

func timespec(ns int64) unix.Timespec {
	return unix.Timespec{Sec: ns / int64(time.Second), Nsec: ns % int64(time.Second)}
}

func omitTime() unix.Timespec {
	return unix.Timespec{Nsec: unix.UTIME_OMIT}
}

func setattrFH(t *Tree, p wire.Params) (wire.Result, error) {
	h, err := t.use(p.Fh)
	if err != nil {
		return wire.Result{}, wrap("setattr", err)
	}
	defer t.done(h)
	fd := int(h.f.Fd())
	if err := applyFH(fd, p); err != nil {
		return wire.Result{}, wrap("setattr", err)
	}
	return getattrFH(t, p.Fh)
}

func applyFH(fd int, p wire.Params) error {
	if p.Valid&wire.SetMode != 0 {
		if err := unix.Fchmod(fd, p.Mode&0o777); err != nil {
			return err
		}
	}
	if p.Valid&(wire.SetUID|wire.SetGID) != 0 {
		uid, gid := -1, -1
		if p.Valid&wire.SetUID != 0 {
			uid = int(p.UID)
		}
		if p.Valid&wire.SetGID != 0 {
			gid = int(p.GID)
		}
		if err := unix.Fchown(fd, uid, gid); err != nil {
			return err
		}
	}
	if p.Valid&wire.SetSize != 0 {
		if err := unix.Ftruncate(fd, int64(p.Size)); err != nil {
			return err
		}
	}
	return applyTimes("/proc/self/fd/"+strconv.Itoa(fd), p, 0)
}

func timePair(p wire.Params) (at, mt unix.Timespec, ok bool) {
	if p.Valid&(wire.SetAtime|wire.SetMtime|wire.SetAtimeNow|wire.SetMtimeNow) == 0 {
		return omitTime(), omitTime(), false
	}
	at, mt = omitTime(), omitTime()
	if p.Valid&wire.SetAtimeNow != 0 {
		at = unix.Timespec{Nsec: unix.UTIME_NOW}
	} else if p.Valid&wire.SetAtime != 0 {
		at = timespec(p.Atime)
	}
	if p.Valid&wire.SetMtimeNow != 0 {
		mt = unix.Timespec{Nsec: unix.UTIME_NOW}
	} else if p.Valid&wire.SetMtime != 0 {
		mt = timespec(p.Mtime)
	}
	return at, mt, true
}
