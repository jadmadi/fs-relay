# Evidence: Exit cleanly or with a busy error on SIGINT in fs-relay-bind

Date: 2026-11-06
Goal: `bind-sigint-busy`
Issue: #4
Branch: `fix/lock-swarm-coherence`

## Finding

`unmountOnSignal` in `cmd/fs-relay-bind/main.go` received one signal, called `_ = server.Unmount()`, and exited its goroutine. When a file on the mount was open, `server.Unmount()` failed with `EBUSY`, `server.Wait()` blocked forever, and a second `SIGINT` was ignored.

## Fix

| Change | File |
|--------|------|
| Add `mountServer` interface, `lazyUnmount(mnt string) error`, and `waitMount(mnt string, srv mountServer, detach func(string) error, sigc <-chan os.Signal) error` | `cmd/fs-relay-bind/cli.go` |
| Replace `go unmountOnSignal(server); server.Wait()` with `waitMount(mnt, server, lazyUnmount, sigc)` | `cmd/fs-relay-bind/main.go` |
| Add `TestWaitMount` covering external unmount, clean signal unmount, busy `EBUSY` unmount, and second signal while `Unmount` blocks | `cmd/fs-relay-bind/cli_test.go` |

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
  "goal_slug": "bind-sigint-busy",
  "prime": {"ran": true},
  "recall": [{"query": "cli extra args bind mountpoint sigint busy multi serve", "matches": 5}],
  "memos": [],
  "lessons": [{"id": "fact_a766b27b"}],
  "cases": [],
  "facts_verify": {"exit_code": 0},
  "handoff": {"id": "hoff_af4f3dc0607f"}
}
```

Verdict: PASS
