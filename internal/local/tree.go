// Package local applies one relay call to a directory tree.
package local

import (
	"context"
	"errors"
	"os"
	"sync"
	"syscall"

	"fs-relay/internal/wire"
)

// Tree is one data directory. Inodes and open files live only as long as the process.
type Tree struct {
	root string

	op sync.RWMutex
	mu sync.Mutex
	ns map[string]*names

	fmu    sync.Mutex
	files  map[uint64]*openFile
	nextFH uint64
}

type names struct {
	next   uint64
	byIno  map[uint64]string
	byPath map[string]uint64
}

type openFile struct {
	f    *os.File
	ino  uint64
	refs int
	drop bool
}

// New serves root as the parent of every namespace directory.
func New(root string) *Tree {
	return &Tree{
		root:   root,
		ns:     map[string]*names{},
		files:  map[uint64]*openFile{},
		nextFH: 1,
	}
}

// Close closes every open file.
func (t *Tree) Close() error {
	t.fmu.Lock()
	files := t.files
	t.files = map[uint64]*openFile{}
	t.fmu.Unlock()
	var err error
	for _, h := range files {
		err = errors.Join(err, h.f.Close())
	}
	return err
}

// Call runs one method. Filesystem failures are syscall.Errno, possibly wrapped.
func (t *Tree) Call(ctx context.Context, method string, p wire.Params) (wire.Result, error) {
	if err := ctx.Err(); err != nil {
		return wire.Result{}, err
	}
	if err := wire.CheckNS(p.Namespace); err != nil {
		return wire.Result{}, err
	}
	fn, ok := operations[method]
	if !ok {
		return wire.Result{}, wire.ErrUnknown
	}
	return fn(ctx, t, p)
}

func (t *Tree) withR(fn func() (wire.Result, error)) (wire.Result, error) {
	t.op.RLock()
	defer t.op.RUnlock()
	return fn()
}

func (t *Tree) withW(fn func() (wire.Result, error)) (wire.Result, error) {
	t.op.Lock()
	defer t.op.Unlock()
	return fn()
}

func newNames() *names {
	return &names{
		next:   1,
		byIno:  map[uint64]string{1: ""},
		byPath: map[string]uint64{"": 1},
	}
}

func (t *Tree) state(ns string) *names {
	st := t.ns[ns]
	if st == nil {
		st = newNames()
		t.ns[ns] = st
	}
	return st
}

func (t *Tree) assign(ns, rel string) uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	st := t.state(ns)
	if id, ok := st.byPath[rel]; ok {
		return id
	}
	st.next++
	st.byPath[rel] = st.next
	st.byIno[st.next] = rel
	return st.next
}

func (t *Tree) relOf(ns string, ino uint64) (string, error) {
	if ino == 0 {
		return "", wire.ErrParams
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	st := t.ns[ns]
	if st == nil {
		if ino == 1 {
			return "", nil
		}
		return "", syscall.ENOENT
	}
	rel, ok := st.byIno[ino]
	if !ok {
		return "", syscall.ENOENT
	}
	return rel, nil
}

func (t *Tree) forget(ns, rel string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	st := t.ns[ns]
	if st == nil {
		return
	}
	st.drop(rel)
}

func (st *names) drop(rel string) {
	prefix := rel + "/"
	for path, id := range st.byPath {
		if path == rel || (rel != "" && len(path) > len(prefix)-1 && path[:len(rel)+1] == prefix) {
			delete(st.byPath, path)
			delete(st.byIno, id)
		}
	}
}

// shift keeps inode numbers when a path moves. The destination's old inode is dropped.
func (t *Tree) shift(ns, oldRel, newRel string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	st := t.ns[ns]
	if st == nil {
		return
	}
	if newRel != oldRel {
		st.drop(newRel)
	}
	type move struct {
		from, to string
		id       uint64
	}
	prefix := oldRel + "/"
	var moves []move
	for path, id := range st.byPath {
		if path == oldRel || len(path) >= len(prefix) && path[:len(prefix)] == prefix {
			moves = append(moves, move{path, newRel + path[len(oldRel):], id})
		}
	}
	for _, m := range moves {
		delete(st.byPath, m.from)
		delete(st.byIno, m.id)
	}
	for _, m := range moves {
		st.byPath[m.to] = m.id
		st.byIno[m.id] = m.to
	}
}
