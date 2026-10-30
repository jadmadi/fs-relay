package local

import (
	"context"
	"os"
	"strings"
	"syscall"

	"fs-relay/internal/wire"
	"golang.org/x/sys/unix"
)

func mkdir(ctx context.Context, t *Tree, p wire.Params) (wire.Result, error) {
	if err := ctx.Err(); err != nil {
		return wire.Result{}, err
	}
	return t.withW(func() (wire.Result, error) {
		rel, abs, err := child(t, p.Namespace, p.Inode, p.Name)
		if err != nil {
			return wire.Result{}, wrap("mkdir", err)
		}
		mode := os.FileMode(p.Mode & 0o777)
		if mode == 0 {
			mode = 0o755
		}
		if err := os.Mkdir(abs, mode); err != nil {
			return wire.Result{}, wrap("mkdir", err)
		}
		a, err := statAttr(abs, t.assign(p.Namespace, rel))
		if err != nil {
			return wire.Result{}, wrap("mkdir", err)
		}
		return wire.Result{Attr: &a}, nil
	})
}

func readdir(ctx context.Context, t *Tree, p wire.Params) (wire.Result, error) {
	if err := ctx.Err(); err != nil {
		return wire.Result{}, err
	}
	return t.withR(func() (wire.Result, error) {
		rel, err := t.relOf(p.Namespace, p.Inode)
		if err != nil {
			return wire.Result{}, wrap("readdir", err)
		}
		abs, err := t.abs(p.Namespace, rel)
		if err != nil {
			return wire.Result{}, wrap("readdir", err)
		}
		ents, err := os.ReadDir(abs)
		if err != nil {
			return wire.Result{}, wrap("readdir", err)
		}
		out := make([]wire.Dirent, 0, len(ents))
		for _, ent := range ents {
			a, err := statAttr(abs+"/"+ent.Name(), t.assign(p.Namespace, childRel(rel, ent.Name())))
			if err != nil {
				return wire.Result{}, wrap("readdir", err)
			}
			out = append(out, wire.Dirent{Name: ent.Name(), Attr: a})
		}
		return wire.Result{Entries: out}, nil
	})
}

func childRel(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "/" + name
}

func unlink(ctx context.Context, t *Tree, p wire.Params) (wire.Result, error) {
	return remove(ctx, t, p, "unlink", unix.Unlink)
}

func rmdir(ctx context.Context, t *Tree, p wire.Params) (wire.Result, error) {
	return remove(ctx, t, p, "rmdir", unix.Rmdir)
}

func remove(ctx context.Context, t *Tree, p wire.Params, op string, fn func(string) error) (wire.Result, error) {
	if err := ctx.Err(); err != nil {
		return wire.Result{}, err
	}
	return t.withW(func() (wire.Result, error) {
		rel, abs, err := child(t, p.Namespace, p.Inode, p.Name)
		if err != nil {
			return wire.Result{}, wrap(op, err)
		}
		if err := fn(abs); err != nil {
			return wire.Result{}, wrap(op, err)
		}
		t.forget(p.Namespace, rel)
		return wire.Result{}, nil
	})
}

func rename(ctx context.Context, t *Tree, p wire.Params) (wire.Result, error) {
	if err := ctx.Err(); err != nil {
		return wire.Result{}, err
	}
	if p.Flags&^uint32(unix.RENAME_NOREPLACE) != 0 {
		return wire.Result{}, syscall.EINVAL
	}
	return t.withW(func() (wire.Result, error) {
		oldRel, oldAbs, err := child(t, p.Namespace, p.Inode, p.Name)
		if err != nil {
			return wire.Result{}, wrap("rename", err)
		}
		newRel, newAbs, err := child(t, p.Namespace, p.NewInode, p.NewName)
		if err != nil {
			return wire.Result{}, wrap("rename", err)
		}
		if newRel == oldRel || strings.HasPrefix(newRel, oldRel+"/") {
			return wire.Result{}, wrap("rename", syscall.EINVAL)
		}
		if err := unix.Renameat2(unix.AT_FDCWD, oldAbs, unix.AT_FDCWD, newAbs, uint(p.Flags)); err != nil {
			return wire.Result{}, wrap("rename", err)
		}
		t.shift(p.Namespace, oldRel, newRel)
		return wire.Result{}, nil
	})
}

func symlink(ctx context.Context, t *Tree, p wire.Params) (wire.Result, error) {
	if err := ctx.Err(); err != nil {
		return wire.Result{}, err
	}
	if p.Target == "" {
		return wire.Result{}, wire.ErrParams
	}
	return t.withW(func() (wire.Result, error) {
		rel, abs, err := child(t, p.Namespace, p.Inode, p.Name)
		if err != nil {
			return wire.Result{}, wrap("symlink", err)
		}
		if err := os.Symlink(p.Target, abs); err != nil {
			return wire.Result{}, wrap("symlink", err)
		}
		a, err := statAttr(abs, t.assign(p.Namespace, rel))
		if err != nil {
			return wire.Result{}, wrap("symlink", err)
		}
		return wire.Result{Attr: &a}, nil
	})
}

func readlink(ctx context.Context, t *Tree, p wire.Params) (wire.Result, error) {
	if err := ctx.Err(); err != nil {
		return wire.Result{}, err
	}
	return t.withR(func() (wire.Result, error) {
		rel, err := t.relOf(p.Namespace, p.Inode)
		if err != nil {
			return wire.Result{}, wrap("readlink", err)
		}
		abs, err := t.abs(p.Namespace, rel)
		if err != nil {
			return wire.Result{}, wrap("readlink", err)
		}
		target, err := os.Readlink(abs)
		if err != nil {
			return wire.Result{}, wrap("readlink", err)
		}
		return wire.Result{Target: target}, nil
	})
}

func opendir(ctx context.Context, t *Tree, p wire.Params) (wire.Result, error) {
	if err := ctx.Err(); err != nil {
		return wire.Result{}, err
	}
	return t.withW(func() (wire.Result, error) {
		rel, err := t.relOf(p.Namespace, p.Inode)
		if err != nil {
			return wire.Result{}, wrap("opendir", err)
		}
		abs, err := t.abs(p.Namespace, rel)
		if err != nil {
			return wire.Result{}, wrap("opendir", err)
		}
		var st unix.Stat_t
		if err := unix.Lstat(abs, &st); err != nil {
			return wire.Result{}, wrap("opendir", err)
		}
		if st.Mode&syscall.S_IFMT != syscall.S_IFDIR {
			return wire.Result{}, wrap("opendir", syscall.ENOTDIR)
		}
		f, err := os.Open(abs)
		if err != nil {
			return wire.Result{}, wrap("opendir", err)
		}
		return wire.Result{Fh: t.add(f, p.Inode)}, nil
	})
}
