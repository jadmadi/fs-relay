# Plan: Close lock wake-up races across EAGAIN registration, downgrades, and multi-waiters

Read [`goals/lock-wake-races/facts.md`](facts.md) first. Complete prerequisite
goal [`goals/bind-lock-coherence/goal.md`](../bind-lock-coherence/goal.md)
before starting this plan.

## Execution Steps

- [ ] **T1 - Move waiter tracking into lockRec under t.mu and wake on lock downgrades**
  - Context: Goal `lock-wake-races` (upstream issue `#32`), cwd `/mnt/Jad/github/lab/fs-relay`, prerequisite: `bind-lock-coherence`.
  - Files:
    - Edit: `internal/local/lock.go` (`lockRec`, `trim`, `put`, `apply`, `flockSet`, `dropFH`, ` dropClient`, `DropOwner`, `setlk`, `setlkw`, `TookWake`, `AnyWaiting`)
    - Edit: `internal/local/call.go` (`operations[wire.Setlkw] = setlkw`)
  - Requirements:
    1. Add `waiters []lockWaiter` (`type lockWaiter struct { key lockKey; fh uint64 }`) to `lockRec` with `addWaiter`, `dropWaiter`, `dropWaiterFH`, and `dropWaiterClient`.
    2. Split `setlk` and `setlkw` via `setlkMode(ctx, t, p, wait bool)` and wire `wire.Setlkw: setlkw` in `internal/local/call.go`.
    3. Under `t.mu` in `setlkMode`:
       - When a lock request (`LockRead` or `LockWrite`, POSIX or `FlagFlock`) fails with `syscall.EAGAIN` and `wait` is `true`, call `rec.addWaiter(key, p.Fh)` before releasing `t.mu`.
       - When a lock request succeeds or `p.Lock == wire.LockUnlock`, call `rec.dropWaiter(key)`.
    4. Clean up waiters on handle/owner/client teardown:
       - `(*lockRec).dropFH(fh)` calls `r.dropWaiterFH(fh)`.
       - `(*Tree).DropOwner(ns, ino, client, owner)` calls `e.locks.dropWaiter(key)`.
       - `(*lockRec).dropClient(client)` calls `r.dropWaiterClient(client)`.
    5. Set `r.wake = true` on every write-to-read downgrade:
       - In `(*lockRec).apply`: when `start == pendingByte && typ == wire.LockRead` replaces `t == wire.LockWrite`, and when `start == sharedStart && typ == wire.LockRead` clears `r.exclusive == o.key`.
       - In `(*lockRec).put`: have `o.trim(start, end)` report whether it trimmed any `wire.LockWrite` span; when it did and `typ == wire.LockRead`, set `r.wake = true`.
       - In `(*lockRec).flockSet`: when `typ == wire.LockRead` and `o.flock == wire.LockWrite`, set `r.wake = true`.
    6. Update `(*Tree).TookWake(ns, p)` to return `(ino, true)` only when `e.locks.wake` was `true` AND `len(e.locks.waiters) > 0` (clearing `e.locks.wake = false` either way). Add `(*Tree).AnyWaiting(ns string) bool`.
  - Constraints & non-goals:
    - Preserve zero allocations in `TestLockCanonicalNoAlloc`.
    - Keep `(*Tree).ReleaseClient(ns, client)` returning `true` whenever `client` held a lock.
  - Acceptance criteria:
    - AC-1.1: `go test ./internal/local` exits `0`.
    - AC-1.2: `TestLockCanonicalNoAlloc` passes with `0` allocations.

- [ ] **T2 - Remove hub-side waiter map and wire server release broadcasts through Tree**
  - Context: Eliminate the race window between `tree.Call` and `hub.markWaiting` in `cmd/fs-relay`, prerequisite: `T1`.
  - Files:
    - Edit: `cmd/fs-relay/hub.go` (`hub`, `newHub`, `dropHold`)
    - Edit: `cmd/fs-relay/serve.go` (`serve`, `publishCall`)
    - Edit: `cmd/fs-relay/ws.go` (`readCalls`)
    - Edit: `cmd/fs-relay/lockwait.go`
  - Requirements:
    1. Remove `waiting`, `markWaiting`, `clearWaiting`, and `anyWaiting` from `cmd/fs-relay/hub.go`, and remove `lockInode` from `cmd/fs-relay/lockwait.go`.
    2. In `cmd/fs-relay/serve.go` and `cmd/fs-relay/ws.go`, remove the post-call `if req.Method == wire.Setlkw && errors.Is(..., syscall.EAGAIN) { h.markWaiting(...) }` block (`tree.Call` now records the waiter atomically under `t.mu`).
    3. In `publishCall` (`cmd/fs-relay/serve.go`), trigger `h.releaseAll(ns)` whenever `_, ok := tree.TookWake(ns, p); ok`.
    4. In `(*hub).dropHold` (`cmd/fs-relay/hub.go`), check `if h.tree != nil && h.tree.ReleaseClient(ns, id) && h.tree.AnyWaiting(ns) { h.releaseAll(ns) }`.
  - Acceptance criteria:
    - AC-2.1: `go test -race ./cmd/fs-relay ./internal/local` exits `0`.
    - AC-2.2: `ripwire . --quality-delta --legend=compact` reports `gating=0`.

- [ ] **T3 - Add tests for downgrade wake-ups, three-waiter contention, and concurrent Setlkw/unlock**
  - Context: Prove all three races in `#32` are closed, prerequisite: `T2`.
  - Files:
    - Edit: `internal/local/lock_test.go`
    - Edit: `cmd/fs-relay/lock_test.go`
  - Requirements:
    1. In `internal/local/lock_test.go`, add `TestLockDowngradeWakesWaitingReader` covering canonical (`sharedStart` and `pendingByte`), non-canonical byte-range, and `FlagFlock` downgrades from `LockWrite` to `LockRead`, asserting `tr.TookWake` returns `(ino, true)` when a reader is waiting in `Setlkw` and `(0, false)` when nobody is waiting.
    2. In `cmd/fs-relay/lock_test.go`, add `TestThreeWaitersWakeOnSuccessiveUnlocks`: Client A holds a write lock; Clients B and C both call `Setlkw` and receive `EAGAIN`; Client A unlocks (waking B); Client B acquires the lock via `Setlkw` and immediately unlocks before C retries; assert Client B's unlock emits a second release event so Client C wakes without waiting for the 1s fallback.
    3. In `cmd/fs-relay/lock_test.go`, add `TestConcurrentSetlkwAndUnlockNeverMissesRelease`: run 50 iterations racing a `Setlkw` call against a concurrent `Setlk(LockUnlock)` and assert the waiter either acquires the lock immediately or receives a release notification within 100ms.
  - Acceptance criteria:
    - AC-3.1: `go test -v -race ./internal/local ./cmd/fs-relay` exits `0`.

- [ ] **T4 - Update SPEC.md, run full verification gates, and certify goal**
  - Context: Document downgrade wake-ups and close the goal in Sila, prerequisite: `T3`.
  - Files:
    - Edit: `SPEC.md` (section `Locks`)
    - Create: `goals/lock-wake-races/evidence.md`
  - Requirements:
    1. In `SPEC.md` under `Locks`, state that waiter registration happens atomically with a conflicting `setlkw` and that downgrading a write lock to a read lock wakes waiting mounts.
    2. Run `gofmt -l .`, `go vet ./...`, `go test ./...`, `go test -race ./...`, `sila gate`, and `sila facts verify`.
    3. Write `goals/lock-wake-races/evidence.md` with command outputs, a `sila/memory-writes/v1` block, and a standalone `Verdict: PASS` line.
    4. Commit with `(goals/lock-wake-races/goal.md)`, run `sila goals certify lock-wake-races --evidence=goals/lock-wake-races/evidence.md`, and reconcile `goals/README.md`.
  - Acceptance criteria:
    - AC-4.1: `gofmt -l .` prints nothing; `go vet ./...` and `go test -race ./...` exit `0`.
    - AC-4.2: `sila goals --json` reports `lock-wake-races` as `implemented` and `verified`.
