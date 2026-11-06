# Evidence: Close lock wake-up races across EAGAIN registration, downgrades, and multi-waiters

Date: 2026-11-06
Goal: `lock-wake-races`
Issue: #32
Branch: `fix/lock-swarm-coherence`

## Finding

Lock release wake-ups had three server-side races:

1. `serve` (`cmd/fs-relay/serve.go`) and `readCalls` (`cmd/fs-relay/ws.go`) called
   `h.markWaiting` after `tree.Call` released `t.mu` and returned `EAGAIN`. An
   unlock arriving in that gap consumed `e.locks.wake` while the waiter map was
   still empty and sent no release event.
2. `(*hub).clearWaiting` deleted the entire `h.waiting[ns][ino]` map on the first
   release. With three or more contenders (A holds, B and C wait), A's unlock
   woke B and wiped C's waiter entry; if B acquired and unlocked before C
   re-registered, B's unlock saw an empty waiter set and sent no release event.
3. Downgrading a write lock to a read lock (`LockWrite -> LockRead`) in
   `(*lockRec).apply`, `(*lockRec).put`, or `(*lockRec).flockSet` did not set
   `r.wake = true`, leaving waiting readers parked until the 1s fallback poll.

## Fix

| Change | File |
|--------|------|
| Track `waiters []lockWaiter` on `lockRec` under `t.mu`, record waiters atomically in `setlkw` on `EAGAIN`, drop them individually on grant/unlock/`dropFH`/`DropOwner`/`dropClient`, set `r.wake = true` on write-to-read downgrades, and gate `TookWake` on `len(e.locks.waiters) > 0` | `internal/local/lock.go`, `internal/local/call.go` |
| Remove `waiting`, `markWaiting`, `clearWaiting`, `anyWaiting`, and `lockInode` from the HTTP/WS hub and trigger `releaseAll` directly from `tree.TookWake` and `tree.AnyWaiting` | `cmd/fs-relay/hub.go`, `cmd/fs-relay/serve.go`, `cmd/fs-relay/ws.go`, `cmd/fs-relay/lockwait.go` |
| Add `TestLockDowngradeWakesWaitingReader`, `TestThreeWaitersWakeOnSuccessiveUnlocks`, and `TestConcurrentSetlkwAndUnlockNeverMissesRelease` | `internal/local/lock_test.go`, `cmd/fs-relay/lock_test.go` |
| Document atomic `setlkw` waiter registration and downgrade wake-ups | `SPEC.md` |

## Tests

- `TestLockDowngradeWakesWaitingReader`: verifies canonical (`sharedStart` and
  `pendingByte`), non-canonical byte-range, and `FlagFlock` write-to-read
  downgrades report `TookWake = (ino, true)` when a reader is waiting and
  `(0, false)` when nobody is waiting.
- `TestThreeWaitersWakeOnSuccessiveUnlocks`: verifies that when Client B and
  Client C both wait on Client A, Client A unlocks, and Client B locks and
  unlocks before Client C retries, Client B's unlock emits a second release
  event that wakes Client C immediately.
- `TestConcurrentSetlkwAndUnlockNeverMissesRelease`: runs 50 concurrent
  iterations racing `Setlkw` against `Setlk(LockUnlock)` and verifies no release
  wake-up is missed.

## Commands

```
gofmt -l .          # empty
go vet ./...        # exit 0
go test ./...       # exit 0
go test -race ./... # exit 0
```

## Memory Writes

```json
{
  "schema": "sila/memory-writes/v1",
  "goal_slug": "lock-wake-races",
  "prime": {"ran": true},
  "recall": [{"query": "setlkw markWaiting clearWaiting downgrade wake", "matches": 5}],
  "memos": [{"id": "fact_404d9611"}],
  "lessons": [{"id": "fact_a8eab9c2"}],
  "cases": [{"id": "fact_58696f07"}],
  "facts_verify": {"exit_code": 0},
  "handoff": {"id": "hoff_83df0afe4bac"}
}
```

Verdict: PASS
