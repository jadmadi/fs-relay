package local

import (
	"errors"
	"syscall"
	"testing"

	"fs-relay/internal/wire"
)

func lockFile(t *testing.T, tr *Tree, name string) (ino, fh uint64) {
	t.Helper()
	res := call(t, tr, wire.Create, wire.Params{Inode: 1, Name: name, Mode: 0o644, Flags: syscall.O_RDWR})
	return res.Attr.Ino, res.Fh
}

func lp(ino, fh uint64, client string, owner uint64, typ uint32, start, size uint64) wire.Params {
	return wire.Params{
		Namespace: "demo",
		Inode:     ino, Fh: fh, Client: client, Owner: owner, Pid: 4242,
		Lock: typ, Offset: int64(start), Size: size,
	}
}

func wantErrno(t *testing.T, err error, want syscall.Errno) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("want %v, got %v", want, err)
	}
}

func TestLockGrantMatrix(t *testing.T) {
	type step struct {
		client string
		typ    uint32
		start  uint64
		want   syscall.Errno
	}
	tests := []struct {
		name  string
		steps []step
	}{
		{"read pending free", []step{{"A", wire.LockRead, pendingByte, 0}}},
		{"two readers share", []step{{"A", wire.LockRead, sharedStart, 0}, {"B", wire.LockRead, sharedStart, 0}}},
		{"reader blocks writer", []step{{"A", wire.LockRead, sharedStart, 0}, {"B", wire.LockWrite, sharedStart, syscall.EAGAIN}}},
		{"writer blocks reader", []step{{"A", wire.LockWrite, sharedStart, 0}, {"B", wire.LockRead, sharedStart, syscall.EAGAIN}}},
		{"reserved is exclusive", []step{{"A", wire.LockWrite, reservedByte, 0}, {"B", wire.LockWrite, reservedByte, syscall.EAGAIN}}},
		{"pending writer blocks reader", []step{{"A", wire.LockWrite, pendingByte, 0}, {"B", wire.LockRead, pendingByte, syscall.EAGAIN}}},
		{"pending reader blocks writer", []step{{"A", wire.LockRead, pendingByte, 0}, {"B", wire.LockWrite, pendingByte, syscall.EAGAIN}}},
		{"other byte never conflicts", []step{{"A", wire.LockWrite, 100, 0}, {"B", wire.LockWrite, 200, 0}}},
		{"same owner merges", []step{{"A", wire.LockWrite, 100, 0}, {"A", wire.LockWrite, 150, 0}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr, _ := newTree(t)
			ino, fh := lockFile(t, tr, "a")
			for i, s := range tt.steps {
				p := lp(ino, fh, s.client, 7, s.typ, s.start, 1)
				_, err := tr.Call(t.Context(), wire.Setlk, p)
				if s.want == 0 && err != nil {
					t.Fatalf("step %d: %v", i, err)
				}
				if s.want != 0 {
					wantErrno(t, err, s.want)
				}
			}
		})
	}
}

func TestLockUpgradeIgnoresSelf(t *testing.T) {
	tr, _ := newTree(t)
	ino, fh := lockFile(t, tr, "a")
	call(t, tr, wire.Setlk, lp(ino, fh, "A", 1, wire.LockRead, sharedStart, 510))
	call(t, tr, wire.Setlk, lp(ino, fh, "B", 2, wire.LockRead, sharedStart, 510))

	_, err := tr.Call(t.Context(), wire.Setlk, lp(ino, fh, "A", 1, wire.LockWrite, sharedStart, 510))
	wantErrno(t, err, syscall.EAGAIN)

	call(t, tr, wire.Setlk, lp(ino, fh, "B", 2, wire.LockUnlock, sharedStart, 510))
	call(t, tr, wire.Setlk, lp(ino, fh, "A", 1, wire.LockWrite, sharedStart, 510))

	got := call(t, tr, wire.Getlk, lp(ino, fh, "B", 2, wire.LockRead, sharedStart, 510))
	if got.Lock == nil || got.Lock.Type != wire.LockWrite || got.Lock.Pid != 4242 {
		t.Fatalf("getlk %+v", got.Lock)
	}
	call(t, tr, wire.Setlk, lp(ino, fh, "A", 1, wire.LockUnlock, sharedStart, 510))
	got = call(t, tr, wire.Getlk, lp(ino, fh, "B", 2, wire.LockRead, sharedStart, 510))
	if got.Lock == nil || got.Lock.Type != wire.LockUnlock {
		t.Fatalf("getlk after unlock %+v", got.Lock)
	}
}

func TestLockPartialUnlockSplits(t *testing.T) {
	tr, _ := newTree(t)
	ino, fh := lockFile(t, tr, "a")
	call(t, tr, wire.Setlk, lp(ino, fh, "A", 1, wire.LockWrite, 0, 101))
	call(t, tr, wire.Setlk, lp(ino, fh, "A", 1, wire.LockUnlock, 40, 21))

	call(t, tr, wire.Setlk, lp(ino, fh, "B", 2, wire.LockWrite, 40, 21))
	for _, r := range []struct{ start, size uint64 }{{0, 40}, {61, 40}} {
		_, err := tr.Call(t.Context(), wire.Setlk, lp(ino, fh, "B", 2, wire.LockWrite, r.start, r.size))
		wantErrno(t, err, syscall.EAGAIN)
	}
}

func TestLockCanonicalCounts(t *testing.T) {
	tr, _ := newTree(t)
	ino, fh := lockFile(t, tr, "a")
	for _, c := range []string{"A", "B", "C"} {
		call(t, tr, wire.Setlk, lp(ino, fh, c, 1, wire.LockRead, sharedStart, 510))
	}
	_, err := tr.Call(t.Context(), wire.Setlk, lp(ino, fh, "W", 9, wire.LockWrite, sharedStart, 510))
	wantErrno(t, err, syscall.EAGAIN)

	call(t, tr, wire.Setlk, lp(ino, fh, "B", 1, wire.LockUnlock, sharedStart, 510))
	_, err = tr.Call(t.Context(), wire.Setlk, lp(ino, fh, "W", 9, wire.LockWrite, sharedStart, 510))
	wantErrno(t, err, syscall.EAGAIN)

	call(t, tr, wire.Setlk, lp(ino, fh, "A", 1, wire.LockUnlock, sharedStart, 510))
	call(t, tr, wire.Setlk, lp(ino, fh, "C", 1, wire.LockUnlock, sharedStart, 510))
	call(t, tr, wire.Setlk, lp(ino, fh, "W", 9, wire.LockWrite, sharedStart, 510))
}

func TestLockFlockIsSeparate(t *testing.T) {
	tr, _ := newTree(t)
	ino, fh := lockFile(t, tr, "a")
	flock := lp(ino, fh, "A", 1, wire.LockWrite, 0, 0)
	flock.Flags = wire.FlagFlock
	call(t, tr, wire.Setlk, flock)

	other := lp(ino, fh, "B", 2, wire.LockRead, 0, 0)
	other.Flags = wire.FlagFlock
	_, err := tr.Call(t.Context(), wire.Setlk, other)
	wantErrno(t, err, syscall.EAGAIN)

	// A byte range never conflicts with a flock.
	call(t, tr, wire.Setlk, lp(ino, fh, "B", 2, wire.LockRead, 100, 10))

	flock.Lock = wire.LockUnlock
	call(t, tr, wire.Setlk, flock)
	call(t, tr, wire.Setlk, other)
}

func TestLockValidation(t *testing.T) {
	tr, _ := newTree(t)
	ino, fh := lockFile(t, tr, "a")
	if _, err := tr.Call(t.Context(), wire.Setlk, lp(ino, fh, "", 1, wire.LockRead, 0, 1)); !errors.Is(err, syscall.EINVAL) {
		t.Fatalf("no client: %v", err)
	}
	if _, err := tr.Call(t.Context(), wire.Setlk, lp(ino, fh, "A", 1, 9, 0, 1)); !errors.Is(err, syscall.EINVAL) {
		t.Fatalf("bad type: %v", err)
	}
	bad := lp(ino, fh, "A", 1, wire.LockRead, 0, 1)
	bad.Offset = -1
	if _, err := tr.Call(t.Context(), wire.Setlk, bad); !errors.Is(err, syscall.EINVAL) {
		t.Fatalf("negative offset: %v", err)
	}
	dir := call(t, tr, wire.Mkdir, wire.Params{Inode: 1, Name: "d", Mode: 0o755})
	if _, err := tr.Call(t.Context(), wire.Setlk, lp(dir.Attr.Ino, 0, "A", 1, wire.LockRead, 0, 1)); !errors.Is(err, syscall.EINVAL) {
		t.Fatalf("directory: %v", err)
	}
}

func TestLockSurvivesRename(t *testing.T) {
	tr, _ := newTree(t)
	dir := call(t, tr, wire.Mkdir, wire.Params{Inode: 1, Name: "d", Mode: 0o755})
	res := call(t, tr, wire.Create, wire.Params{Inode: dir.Attr.Ino, Name: "a", Mode: 0o644, Flags: syscall.O_RDWR})
	ino, fh := res.Attr.Ino, res.Fh
	call(t, tr, wire.Setlk, lp(ino, fh, "A", 1, wire.LockWrite, 100, 1))

	call(t, tr, wire.Rename, wire.Params{Inode: 1, Name: "d", NewInode: 1, NewName: "e"})

	_, err := tr.Call(t.Context(), wire.Setlk, lp(ino, fh, "B", 2, wire.LockWrite, 100, 1))
	wantErrno(t, err, syscall.EAGAIN)
}

func TestLockDroppedWithInode(t *testing.T) {
	tr, _ := newTree(t)
	ino, fh := lockFile(t, tr, "a")
	call(t, tr, wire.Setlk, lp(ino, fh, "A", 1, wire.LockWrite, 100, 1))
	call(t, tr, wire.Release, wire.Params{Fh: fh})
	call(t, tr, wire.Unlink, wire.Params{Inode: 1, Name: "a"})
	if _, ok := tr.ns["demo"].byIno[ino]; ok {
		t.Fatal("inode kept after unlink")
	}
	_, err := tr.Call(t.Context(), wire.Setlk, lp(ino, 0, "B", 2, wire.LockWrite, 100, 1))
	if !errors.Is(err, syscall.ENOENT) {
		t.Fatalf("stale inode: %v", err)
	}
}

func TestLockReleaseDropsByHandle(t *testing.T) {
	tr, _ := newTree(t)
	ino, fh := lockFile(t, tr, "a")
	call(t, tr, wire.Setlk, lp(ino, fh, "A", 1, wire.LockWrite, 100, 1))
	call(t, tr, wire.Release, wire.Params{Fh: fh, Inode: ino})
	call(t, tr, wire.Setlk, lp(ino, 0, "B", 2, wire.LockWrite, 100, 1))
}

func TestLockReleaseClient(t *testing.T) {
	tr, _ := newTree(t)
	ino, fh := lockFile(t, tr, "a")
	call(t, tr, wire.Setlk, lp(ino, fh, "A", 1, wire.LockWrite, 100, 1))
	if !tr.ReleaseClient("demo", "A") {
		t.Fatal("release client reported no change")
	}
	call(t, tr, wire.Setlk, lp(ino, fh, "B", 2, wire.LockWrite, 100, 1))
}

func TestLockCanonicalNoAlloc(t *testing.T) {
	tr, _ := newTree(t)
	ino, fh := lockFile(t, tr, "a")
	rec := &lockRec{}
	key := lockKey{client: "A", owner: 1}
	rec.put(key, fh, sharedStart, sharedEnd, wire.LockRead, 1)
	rec.unlock(key, sharedStart, sharedEnd)
	_ = ino

	got := testing.AllocsPerRun(200, func() {
		rec.put(key, fh, sharedStart, sharedEnd, wire.LockRead, 1)
		rec.unlock(key, sharedStart, sharedEnd)
	})
	if got != 0 {
		t.Fatalf("allocations per lock cycle: %v", got)
	}
}

func TestLockSelfDowngrade(t *testing.T) {
	tr, _ := newTree(t)
	ino, fh := lockFile(t, tr, "a")
	call(t, tr, wire.Setlk, lp(ino, fh, "A", 1, wire.LockWrite, sharedStart, 510))
	call(t, tr, wire.Setlk, lp(ino, fh, "A", 1, wire.LockRead, sharedStart, 510))
	call(t, tr, wire.Setlk, lp(ino, fh, "B", 2, wire.LockRead, sharedStart, 510))
	_, err := tr.Call(t.Context(), wire.Setlk, lp(ino, fh, "B", 2, wire.LockWrite, sharedStart, 510))
	wantErrno(t, err, syscall.EAGAIN)
}

func TestLockDropOwnerIsNarrow(t *testing.T) {
	tr, _ := newTree(t)
	ino, fh := lockFile(t, tr, "a")
	second := call(t, tr, wire.Open, wire.Params{Inode: ino, Flags: syscall.O_RDWR})
	call(t, tr, wire.Setlk, lp(ino, fh, "A", 1, wire.LockWrite, 100, 1))
	call(t, tr, wire.Setlk, lp(ino, second.Fh, "A", 2, wire.LockWrite, 200, 1))

	if !tr.DropOwner("demo", ino, "A", 1) {
		t.Fatal("drop owner reported no change")
	}
	call(t, tr, wire.Setlk, lp(ino, 0, "B", 9, wire.LockWrite, 100, 1))
	_, err := tr.Call(t.Context(), wire.Setlk, lp(ino, 0, "B", 9, wire.LockWrite, 200, 1))
	wantErrno(t, err, syscall.EAGAIN)
}

func TestLockPendingDowngrade(t *testing.T) {
	tr, _ := newTree(t)
	ino, fh := lockFile(t, tr, "a")
	call(t, tr, wire.Setlk, lp(ino, fh, "A", 1, wire.LockWrite, pendingByte, 1))
	call(t, tr, wire.Setlk, lp(ino, fh, "A", 1, wire.LockRead, pendingByte, 1))

	got := call(t, tr, wire.Getlk, lp(ino, fh, "B", 2, wire.LockWrite, pendingByte, 1))
	if got.Lock == nil || got.Lock.Type != wire.LockRead {
		t.Fatalf("pending still write: %+v", got.Lock)
	}
	call(t, tr, wire.Setlk, lp(ino, fh, "B", 2, wire.LockRead, pendingByte, 1))
	_, err := tr.Call(t.Context(), wire.Setlk, lp(ino, fh, "B", 2, wire.LockWrite, pendingByte, 1))
	wantErrno(t, err, syscall.EAGAIN)

	call(t, tr, wire.Setlk, lp(ino, fh, "A", 1, wire.LockUnlock, pendingByte, 1))
	call(t, tr, wire.Setlk, lp(ino, fh, "B", 2, wire.LockWrite, pendingByte, 1))
}

func TestLockAnyCloseDropsOwner(t *testing.T) {
	tr, _ := newTree(t)
	ino, fh1 := lockFile(t, tr, "a")
	second := call(t, tr, wire.Open, wire.Params{Inode: ino, Flags: syscall.O_RDWR})
	call(t, tr, wire.Setlk, lp(ino, fh1, "A", 1, wire.LockWrite, 100, 1))
	call(t, tr, wire.Setlk, lp(ino, second.Fh, "A", 1, wire.LockWrite, 200, 1))

	if !tr.DropFH("demo", ino, fh1) {
		t.Fatal("drop reported no change")
	}
	call(t, tr, wire.Setlk, lp(ino, 0, "B", 2, wire.LockWrite, 100, 1))
	call(t, tr, wire.Setlk, lp(ino, 0, "B", 2, wire.LockWrite, 200, 1))
}
