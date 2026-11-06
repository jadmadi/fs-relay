# Evidence: Guard the data directory with an exclusive lock unless forced

Date: 2026-11-06
Goal: `single-serve-data-lock`
Issue: #7
Branch: `fix/lock-swarm-coherence`

## Finding

Two `fs-relay` processes could serve the same `-data` directory on different ports with separate in-memory inode, handle, lock, and eviction tables. Nothing warned or refused.

## Fix

| Change | File |
|--------|------|
| Add `(*Tree).Lock() error` acquiring `unix.Flock(fd, unix.LOCK_EX\|unix.LOCK_NB)` on `<root>/.fs-relay.lock` (`0600`), writing `<pid>\n`, guarding `t.lockFile` under `t.fmu`, and releasing in `(*Tree).Close()` | `internal/local/tree.go` |
| Add `-force-multi-serve` flag and call `tree.Lock()` when `!*forceMulti` | `cmd/fs-relay/main.go` |
| Add `TestTreeLockRefusesSecondServerAndReleasesOnClose` with doc comment | `internal/local/local_test.go` |
| Document `-force-multi-serve` and single-server data root locking | `SPEC.md`, `README.md`, `ROADMAP.md` |

## Commands

```
gofmt -l .          # empty
go vet ./...        # exit 0
go test -race ./... # exit 0
```

## Memory Writes

```json
{
  "schema": "sila/memory-writes/v1",
  "goal_slug": "single-serve-data-lock",
  "prime": {"ran": true},
  "recall": [{"query": "cli extra args bind mountpoint sigint busy multi serve", "matches": 5}],
  "memos": [{"id": "fact_9288582b"}],
  "lessons": [{"id": "fact_a766b27b"}],
  "cases": [],
  "facts_verify": {"exit_code": 0},
  "handoff": {"id": "hoff_af4f3dc0607f"}
}
```

Verdict: PASS
