package main

import (
	"context"
	"sort"
	"sync"
	"syscall"
	"testing"
	"time"

	"fs-relay/internal/wire"

	"github.com/hanwen/go-fuse/v2/fuse"
)

type callRecord struct {
	method string
	p      wire.Params
}

type fakeCaller struct {
	mu     sync.Mutex
	calls  []callRecord
	rel    chan struct{}
	relMu  sync.Mutex
	deny   bool
	before func(method string, p wire.Params)
}

func (f *fakeCaller) Call(ctx context.Context, method string, p wire.Params) (wire.Result, error) {
	f.mu.Lock()
	f.calls = append(f.calls, callRecord{method: method, p: p})
	deny := f.deny && (method == wire.Setlk || method == wire.Setlkw)
	before := f.before
	f.mu.Unlock()
	if before != nil {
		before(method, p)
	}
	if deny {
		return wire.Result{}, &wire.Error{Code: uint32(syscall.EAGAIN)}
	}
	return wire.Result{}, nil
}

func (f *fakeCaller) Releases() <-chan struct{} {
	f.relMu.Lock()
	defer f.relMu.Unlock()
	if f.rel == nil {
		f.rel = make(chan struct{})
	}
	return f.rel
}

func (f *fakeCaller) release() {
	f.relMu.Lock()
	ch := f.rel
	f.rel = make(chan struct{})
	f.relMu.Unlock()
	if ch != nil {
		close(ch)
	}
}

func (f *fakeCaller) setBefore(fn func(method string, p wire.Params)) {
	f.mu.Lock()
	f.before = fn
	f.mu.Unlock()
}

func (f *fakeCaller) setDeny(v bool) {
	f.mu.Lock()
	f.deny = v
	f.mu.Unlock()
}

func (f *fakeCaller) count(method string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if c.method == method {
			n++
		}
	}
	return n
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

func TestSetlkwRetriesOnRelease(t *testing.T) {
	f := &fakeCaller{}
	n := &node{c: f, ino: 5}
	h := &handle{c: f, n: n, ino: 5, local: true}
	lk := &fuse.FileLock{Start: 0, End: 0, Typ: syscall.F_RDLCK}
	f.setDeny(true)

	done := make(chan syscall.Errno, 1)
	go func() { done <- n.Setlkw(context.Background(), h, 7, lk, 0) }()

	deadline := time.Now().Add(time.Second)
	for f.count(wire.Setlkw) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("setlkw never tried")
		}
		time.Sleep(5 * time.Millisecond)
	}
	f.setDeny(false)
	f.release()

	select {
	case errno := <-done:
		if errno != 0 {
			t.Fatalf("setlkw: %v", errno)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("setlkw did not wake")
	}
	if f.count(wire.Setlkw) < 2 {
		t.Fatalf("setlkw tried %d times", f.count(wire.Setlkw))
	}
}

func TestSetlkwCancelReturnsEINTR(t *testing.T) {
	f := &fakeCaller{}
	f.setDeny(true)
	n := &node{c: f, ino: 5}
	h := &handle{c: f, n: n, ino: 5, local: true}
	lk := &fuse.FileLock{Start: 0, End: 0, Typ: syscall.F_RDLCK}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if errno := n.Setlkw(ctx, h, 7, lk, 0); errno != syscall.EINTR {
		t.Fatalf("cancel: %v", errno)
	}
}

func TestReleaseWaitsForInFlightGrant(t *testing.T) {
	for _, tc := range []struct {
		name string
		fh   uint64
	}{
		{"read-only", 0},
		{"writable", 11},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeCaller{}
			n := &node{c: f, ino: 5}
			h := &handle{c: f, n: n, ino: 5, fh: tc.fh}
			lk := &fuse.FileLock{Start: 0, End: 9, Typ: syscall.F_RDLCK}

			granting := make(chan struct{})
			finish := make(chan struct{})
			var once sync.Once
			f.setBefore(func(method string, p wire.Params) {
				switch method {
				case wire.Setlk:
					once.Do(func() { close(granting) })
					<-finish
				case wire.Release:
					select {
					case <-finish:
					default:
						t.Error("release sent while a grant was in flight")
					}
				}
			})

			grant := make(chan syscall.Errno, 1)
			go func() { grant <- n.Setlk(context.Background(), h, 7, lk, 0) }()
			<-granting

			released := make(chan syscall.Errno, 1)
			go func() { released <- h.Release(context.Background()) }()
			close(finish)

			if errno := <-grant; errno != 0 {
				t.Fatalf("setlk: %v", errno)
			}
			if errno := <-released; errno != 0 {
				t.Fatalf("release: %v", errno)
			}
			if n := f.count(wire.Release); n != 1 {
				t.Fatalf("release calls: %d", n)
			}
		})
	}
}

func TestGrantAfterCloseIsRefused(t *testing.T) {
	f := &fakeCaller{}
	n := &node{c: f, ino: 5}
	h := &handle{c: f, n: n, ino: 5, local: true}
	h.markClosed()
	lk := &fuse.FileLock{Start: 0, End: 9, Typ: syscall.F_RDLCK}
	if errno := n.Setlk(context.Background(), h, 7, lk, 0); errno != syscall.EINTR {
		t.Fatalf("setlk after close: %v", errno)
	}
	if n := f.count(wire.Setlk); n != 0 {
		t.Fatalf("setlk calls after close: %d", n)
	}
}

func TestReleaseWithErrorStillDropsOwners(t *testing.T) {
	f := &fakeCaller{}
	n := &node{c: f, ino: 5}
	h := &handle{c: f, n: n, ino: 5, local: true}
	read := &fuse.FileLock{Start: 0, End: 9, Typ: syscall.F_RDLCK}
	if errno := n.Setlk(context.Background(), h, 7, read, 0); errno != 0 {
		t.Fatalf("setlk: %v", errno)
	}
	h.wb.err = syscall.EIO

	if errno := h.Release(context.Background()); errno != syscall.EIO {
		t.Fatalf("release: %v", errno)
	}
	var released bool
	for _, c := range f.records() {
		if c.method == wire.Release && c.p.Owner == 7 {
			released = true
		}
	}
	if !released {
		t.Fatal("owners kept after a failed close")
	}
}
