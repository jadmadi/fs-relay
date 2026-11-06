# Facts: Close lock wake-up races across EAGAIN registration, downgrades, and multi-waiters

## Verified Current State

Facts observed at `upstream/main` (`90e87b0`). Every row names the file and
symbol where the implementer can verify it.

| Fact | Where |
| ---- | ----- |
| `operations[wire.Setlk]` and `operations[wire.Setlkw]` both dispatch to `setlk` | `internal/local/call.go`, `operations` (lines 36-37) |
| `serve` and `readCalls` call `h.markWaiting(ns, lockInode(tree, req.Params), req.Params.Client)` only after `tree.Call` has released `t.mu` and returned `EAGAIN` | `cmd/fs-relay/serve.go` (lines 66-69), `cmd/fs-relay/ws.go` (lines 98-101) |
| `(*hub).clearWaiting` deletes the entire `h.waiting[ns][ino]` map on the first release (`delete(h.waiting[ns], ino)`) | `cmd/fs-relay/hub.go`, `(*hub).clearWaiting` (lines 69-82) |
| `(*Tree).TookWake` consumes `e.locks.wake = false` under `t.mu` without checking whether any waiter is registered on `e.locks` | `internal/local/lock.go`, `(*Tree).TookWake` (lines 586-602) |
| `(*lockRec).apply` downgrades `LockWrite` to `LockRead` on `pendingByte` (lines 231-241) and `sharedStart` (lines 254-258) without setting `r.wake = true` | `internal/local/lock.go`, `(*lockRec).apply` (lines 229-273) |
| `(*lockRec).put` trims overlapping spans of the same owner (`o.trim(start, end)`) without setting `r.wake = true` when a write span is replaced by a read span | `internal/local/lock.go`, `(*lockRec).put` (lines 218-227) |
| `(*lockRec).flockSet` downgrades `o.flock` from `wire.LockWrite` to `wire.LockRead` without setting `r.wake = true` | `internal/local/lock.go`, `(*lockRec).flockSet` (lines 338-345) |
| `TestLockReleaseClient` asserts `tr.ReleaseClient("demo", "A")` returns `true` when client `"A"` held a lock even with no waiters | `internal/local/lock_test.go`, `TestLockReleaseClient` (lines 201-209) |

## Architectural Invariants & Constraints

1. Only `internal/local` owns lock and waiter state; guard all mutations to
   `lockRec` (including `waiters` and `wake`) with `t.mu`.
2. Preserve zero allocations on the uncontended canonical lock/unlock cycle
   (`TestLockCanonicalNoAlloc` in `internal/local/lock_test.go:211-227`).
3. Do not broadcast a release event when `len(e.locks.waiters) == 0`.
4. Keep `(*Tree).ReleaseClient(ns, client string) bool` returning `true`
   whenever `client` held at least one lock in `ns`.

## Data Layout & Interface Contracts

Waiter entry in `internal/local/lock.go`:

```go
type lockWaiter struct {
	key lockKey
	fh  uint64
}
```

Add `waiters []lockWaiter` to `lockRec` in `internal/local/lock.go`:

- `func (r *lockRec) addWaiter(key lockKey, fh uint64)`: if `key` is already in
  `r.waiters`, update `fh` if non-zero and return; otherwise append
  `lockWaiter{key: key, fh: fh}`.
- `func (r *lockRec) dropWaiter(key lockKey)`: swap-remove `key` from
  `r.waiters`.
- `func (r *lockRec) dropWaiterFH(fh uint64)`: if `fh != 0`, remove every entry
  in `r.waiters` with `w.fh == fh`.
- `func (r *lockRec) dropWaiterClient(client string)`: remove every entry in
  `r.waiters` with `w.key.client == client`.

Updated `TookWake` and `AnyWaiting` on `*Tree` in `internal/local/lock.go`:

```go
// TookWake consumes a pending release wake and reports the inode when at least
// one client is waiting on it.
func (t *Tree) TookWake(ns string, p wire.Params) (uint64, bool)

// AnyWaiting reports whether any inode in ns has a waiting lock request.
func (t *Tree) AnyWaiting(ns string) bool
```
