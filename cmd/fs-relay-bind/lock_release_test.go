package main

import (
	"context"
	"sort"
	"sync"
	"syscall"
	"testing"

	"fs-relay/internal/wire"

	"github.com/hanwen/go-fuse/v2/fuse"
)

type callRecord struct {
	method string
	p      wire.Params
}

type fakeCaller struct {
	mu    sync.Mutex
	calls []callRecord
}

func (f *fakeCaller) Call(ctx context.Context, method string, p wire.Params) (wire.Result, error) {
	f.mu.Lock()
	f.calls = append(f.calls, callRecord{method: method, p: p})
	f.mu.Unlock()
	return wire.Result{}, nil
}

func (f *fakeCaller) records() []callRecord {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]callRecord, len(f.calls))
	copy(out, f.calls)
	return out
}

func TestReadOnlyCloseDropsOnlyItsOwners(t *testing.T) {
	f := &fakeCaller{}
	n := &node{c: f, ino: 5}
	h := &handle{c: f, n: n, ino: 5, local: true}

	read := &fuse.FileLock{Start: 0, End: 9, Typ: syscall.F_RDLCK, Pid: 1}
	if errno := n.Setlk(context.Background(), h, 7, read, 0); errno != 0 {
		t.Fatalf("setlk 7: %v", errno)
	}
	if errno := n.Setlk(context.Background(), h, 8, read, 0); errno != 0 {
		t.Fatalf("setlk 8: %v", errno)
	}
	if errno := h.Release(context.Background()); errno != 0 {
		t.Fatalf("release: %v", errno)
	}

	var owners []uint64
	for _, c := range f.records() {
		if c.method != wire.Release {
			continue
		}
		if c.p.Fh != 0 {
			t.Fatalf("release named a handle: %+v", c.p)
		}
		owners = append(owners, c.p.Owner)
	}
	sort.Slice(owners, func(i, j int) bool { return owners[i] < owners[j] })
	if len(owners) != 2 || owners[0] != 7 || owners[1] != 8 {
		t.Fatalf("owners released: %v", owners)
	}
	if len(h.takeOwners()) != 0 {
		t.Fatal("owners kept after release")
	}
}

func TestWritableHandleReleasesByHandle(t *testing.T) {
	f := &fakeCaller{}
	n := &node{c: f, ino: 5}
	h := &handle{c: f, n: n, ino: 5, fh: 11}

	read := &fuse.FileLock{Start: 0, End: 9, Typ: syscall.F_RDLCK}
	if errno := n.Setlk(context.Background(), h, 7, read, 0); errno != 0 {
		t.Fatalf("setlk: %v", errno)
	}
	if errno := h.Release(context.Background()); errno != 0 {
		t.Fatalf("release: %v", errno)
	}

	recs := f.records()
	for _, c := range recs {
		if c.method == wire.Setlk && c.p.Fh != 11 {
			t.Fatalf("setlk without handle: %+v", c.p)
		}
	}
	var rel *callRecord
	for i := range recs {
		if recs[i].method == wire.Release {
			rel = &recs[i]
		}
	}
	if rel == nil || rel.p.Fh != 11 || rel.p.Owner != 0 || rel.p.Inode != 5 {
		t.Fatalf("release record %+v", rel)
	}
}

func TestGetlkRecordsNoOwner(t *testing.T) {
	f := &fakeCaller{}
	n := &node{c: f, ino: 5}
	h := &handle{c: f, n: n, ino: 5, local: true}

	ask := &fuse.FileLock{Start: 0, End: 9, Typ: syscall.F_WRLCK}
	var out fuse.FileLock
	_ = n.Getlk(context.Background(), h, 7, ask, 0, &out)
	if len(h.takeOwners()) != 0 {
		t.Fatal("getlk recorded an owner")
	}
}
