package local

import (
	"strings"

	"fs-relay/internal/wire"
)

// Changed reports the names a successful call requires other mounts to drop.
// The server does not call it when the call failed, and then nothing is published.
func (t *Tree) Changed(method string, p wire.Params) []wire.Change {
	switch method {
	case wire.Mkdir, wire.Create, wire.Symlink, wire.Unlink, wire.Rmdir:
		return oneChange(p.Inode, p.Name)
	case wire.Rename:
		old := oneChange(p.Inode, p.Name)
		return append(old, oneChange(p.NewInode, p.NewName)...)
	case wire.Write, wire.Setattr:
		return t.fileChange(p)
	case wire.Release:
		if len(p.Data) == 0 {
			return nil
		}
		return t.fileChange(p)
	case wire.CopyFileRange:
		return t.fileChange(wire.Params{Namespace: p.Namespace, Fh: p.FhOut})
	default:
		return nil
	}
}

func oneChange(parent uint64, name string) []wire.Change {
	if parent == 0 || name == "" {
		return nil
	}
	return []wire.Change{{Parent: parent, Name: name}}
}

func (t *Tree) fileChange(p wire.Params) []wire.Change {
	ino := p.Inode
	if p.Fh != 0 {
		if id, ok := t.inoOf(p.Fh); ok {
			ino = id
		}
	}
	if ino == 0 {
		return nil
	}
	parent, name, ok := t.parentOf(p.Namespace, ino)
	if !ok {
		return nil
	}
	return oneChange(parent, name)
}

func (t *Tree) inoOf(fh uint64) (uint64, bool) {
	t.fmu.Lock()
	defer t.fmu.Unlock()
	h, ok := t.files[fh]
	if !ok {
		return 0, false
	}
	return h.ino, true
}

func (t *Tree) parentOf(ns string, ino uint64) (uint64, string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	st := t.ns[ns]
	if st == nil {
		return 0, "", false
	}
	rel, ok := st.byIno[ino]
	if !ok || rel.rel == "" {
		return 0, "", false
	}
	parentRel, name := splitRel(rel.rel)
	pid, ok := st.byPath[parentRel]
	if !ok {
		return 0, "", false
	}
	return pid, name, true
}

func splitRel(rel string) (string, string) {
	i := strings.LastIndex(rel, "/")
	if i < 0 {
		return "", rel
	}
	return rel[:i], rel[i+1:]
}
