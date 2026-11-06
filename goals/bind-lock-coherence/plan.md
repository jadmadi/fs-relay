# Plan: Drain write-behind before unlock and drop cached reads on lock grant

Read [`goals/bind-lock-coherence/facts.md`](facts.md) first. Merge `upstream/main`
(`90e87b0` or newer) into the working branch before editing so `beginOp`/`endOp`
in `cmd/fs-relay-bind/lock.go` is present.

## Execution Steps

- [ ] **T1 - Drain write-behind before unlock and drop caches on lock grant**
  - Context: Goal `bind-lock-coherence` (upstream issue `#31`), cwd `/mnt/Jad/github/lab/fs-relay`, prerequisite: none.
  - Files:
    - Edit: `cmd/fs-relay-bind/lock.go` (`(*node).lock`)
    - Read: `cmd/fs-relay-bind/writeback.go` (`(*node).drain`, `(*handle).dropCache`)
    - Read: `cmd/fs-relay-bind/cache.go` (`(*kernel).forgetIno`)
  - Requirements:
    1. In `(*node).lock`, when `method != wire.Getlk && p.Lock == wire.LockUnlock`, call `drainErr := n.drain(ctx)` before `n.c.Call(ctx, method, p)`.
    2. Always send `n.c.Call(ctx, method, p)` even when `drainErr != 0`. If `drainErr != 0`, return `res, drainErr` so the caller sees the writeback failure while the server lock is still released.
    3. When `grant` is true (`method != wire.Getlk && p.Lock != wire.LockUnlock`) and `err == nil`, call `n.forgetAttr()` and, when `n.k != nil`, `n.k.forgetIno(n.ino)` before returning.
    4. Keep `Getlk` free of both `n.drain(ctx)` and cache invalidation.
  - Constraints & non-goals:
    - Do not hold `n.mu` or `h.omu` across `n.drain(ctx)` or `n.c.Call`.
    - Do not modify `(*handle).Flush` in `cmd/fs-relay-bind/handle.go`.
  - Example & boundary case:
    - Input: `h.Write(ctx, []byte("abc"), 0)` on handle `fh=11` (buffered in `h.wb.buf`), followed by `n.Setlk(ctx, h, 7, &fuse.FileLock{Typ: syscall.F_UNLCK}, 0)`.
    - Expected output: `caller.Call` receives `wire.Write` (`Data: "abc"`) first, and `wire.Setlk` (`Lock: wire.LockUnlock`) second.
    - Edge case most likely to be implemented wrong: returning early when `n.drain(ctx)` returns `EIO`, which skips `wire.Setlk(LockUnlock)` and leaves the lock held forever on the server.
  - Acceptance criteria:
    - AC-1.1: `go build ./cmd/fs-relay-bind` in `/mnt/Jad/github/lab/fs-relay` exits `0`.
    - AC-1.2: `ripwire . --quality-delta --legend=compact` reports `gating=0`.

- [ ] **T2 - Add unit tests for unlock write-behind drain, drain failure, and grant cache drop**
  - Context: Prove all three behaviors in `cmd/fs-relay-bind` without a real FUSE mount, prerequisite: `T1`.
  - Files:
    - Edit: `cmd/fs-relay-bind/lock_release_test.go`
  - Requirements:
    1. Add `TestUnlockDrainsWriteBehindBeforeSetlk`: buffer a small write (`< writeBehindMin`) through `h.Write`, call `n.Setlk` with `F_UNLCK`, and assert `f.records()` contains `wire.Write` at index `0` and `wire.Setlk` (`LockUnlock`) at index `1`.
    2. Add `TestUnlockWithDrainErrorStillReleasesLock`: set a sticky or failing writeback on `h`, call `n.Setlk` with `F_UNLCK`, and assert `n.Setlk` returns the writeback `syscall.Errno` AND `f.records()` still contains the `wire.Setlk` (`LockUnlock`) call.
    3. Add `TestLockGrantDropsCachedAttrAndBlob`: populate `n.remember(wire.Attr{Ino: 5, Size: 99})` and `k.rememberBlob(1, "db", 5, []byte("old"))`, call `n.Setlk` with `F_RDLCK` (or `F_WRLCK`), and assert `_, hit := n.cached()` is `false` and `_, hit := k.blob(5, 0, 3)` is `false`. Also assert `n.Getlk` leaves both caches intact.
  - Acceptance criteria:
    - AC-2.1: `go test -v -run 'TestUnlockDrainsWriteBehindBeforeSetlk|TestUnlockWithDrainErrorStillReleasesLock|TestLockGrantDropsCachedAttrAndBlob' ./cmd/fs-relay-bind` exits `0`.
    - AC-2.2: `go test -race ./cmd/fs-relay-bind` exits `0`.

- [ ] **T3 - Update SPEC.md, run full gates, and certify goal**
  - Context: Keep `SPEC.md` aligned with the write-behind and lock coherence contract, prerequisite: `T2`.
  - Files:
    - Edit: `SPEC.md` (section `Coherence`, line 214, and section `Locks`)
    - Create: `goals/bind-lock-coherence/evidence.md`
  - Requirements:
    1. In `SPEC.md` under `Coherence`, add unlock (`setlk` with `F_UNLCK`) to the list of operations that send held write-behind bytes, and state under `Locks` that a lock grant drops the mount's cached attributes and prefetched body for that inode.
    2. Run `gofmt -l .`, `go vet ./...`, `go test ./...`, `go test -race ./...`, `sila gate`, and `sila facts verify`.
    3. Write `goals/bind-lock-coherence/evidence.md` with command outputs, a `sila/memory-writes/v1` block, and a standalone `Verdict: PASS` line.
    4. Commit with `(goals/bind-lock-coherence/goal.md)`, run `sila goals certify bind-lock-coherence --evidence=goals/bind-lock-coherence/evidence.md`, and reconcile `goals/README.md`.
  - Acceptance criteria:
    - AC-3.1: `gofmt -l .` prints nothing; `go vet ./...` and `go test -race ./...` exit `0`.
    - AC-3.2: `sila goals --json` reports `bind-lock-coherence` as `implemented` and `verified`.
