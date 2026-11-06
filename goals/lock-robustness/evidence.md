# Evidence: Harden the lock release path

Date: 2026-11-04
Goal: `lock-robustness`
Issue: #23

## Findings and fixes

| Finding | Fix |
|---------|-----|
| A full 32-slot release channel blocked the socket and join readers | `Releases()` returns a coalescing broadcast: a release closes it and installs a new channel; every waiter wakes and the reader never blocks |
| The bind could miss a release between `EAGAIN` and `Releases()` | The bind captures the channel before the grant attempt |
| A failed close kept the locks | `release` calls `DropFH` on the write error and short-write paths |
| Closing one of two writable descriptors kept the owner | An owner keeps two handles inline plus an overflow slice; any matching close drops the owner, matching POSIX any-close |

## Tests

- `TestReleaseBroadcastWakesAll`
- `TestReleaseWithoutWaiterDoesNotBlock`
- `TestReleaseFailureDropsLocks`
- `TestLockAnyCloseDropsOwner`
- `TestLockCanonicalNoAlloc` still reports zero allocations
- `TestPostSetlkwReturnsEAGAINAndReleaseWakesClient` updated to capture the
  broadcast before the unlock

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
  "goal_slug": "lock-robustness",
  "prime": {"ran": true},
  "recall": [{"query": "release channel blocking", "matches": 0}],
  "memos": [{"id": "fact_6c4c75aa"}],
  "lessons": [{"id": "fact_2017fa81"}],
  "cases": [],
  "facts_verify": {"exit_code": 0},
  "handoff": {"id": "session-end"}
}
```

Verdict: PASS
