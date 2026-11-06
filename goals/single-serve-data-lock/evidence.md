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
| Add `(*Tree).Lock() error` acquiring `unix.Flock(fd, unix.LOCK_EX\|unix.LOCK_NB)` on `<root>/.fs-relay.lock` (`0600`, `syscall.O_NOFOLLOW`, verified regular file), writing `<pid>\n`, guarding `t.lockFile` under `t.fmu`, releasing in `(*Tree).Close()`, and rejecting symlinked namespaces with `os.Lstat` in `CheckNamespace` | `internal/local/tree.go` |
| Verify in `(*Tree).abs` that the namespace root is a directory (`os.Lstat`) and parent directories inside a namespace resolve under `data/<ns>` via `filepath.EvalSymlinks`, and hold `t.op.RLock()` across path resolution + `os.OpenFile` in `(*Tree).source` and `openCapped` | `internal/local/path.go`, `internal/local/file.go` |
| Add `-force-multi-serve` flag, call `tree.Lock()` when `!*forceMulti`, register `signal.Notify` before `srv.Serve`, and document `main`/`run` | `cmd/fs-relay/main.go` |
| Add `TestTreeLockRefusesSecondServerAndReleasesOnClose`, `TestTreeLockRejectsSymlinkAndProtectsLockFile`, and `TestForceMultiServeFlag` | `internal/local/local_test.go`, `cmd/fs-relay/cli_test.go` |
| Document `-force-multi-serve` and advisory single-server data root locking | `SPEC.md`, `README.md`, `ROADMAP.md` |

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
  "lessons": [{"id": "fact_a766b27b"}, {"id": "fact_d62b5686"}, {"id": "fact_f16be28c"}],
  "cases": [],
  "facts_verify": {"exit_code": 0},
  "handoff": {"id": "hoff_af4f3dc0607f"}
}
```

Verdict: PASS
