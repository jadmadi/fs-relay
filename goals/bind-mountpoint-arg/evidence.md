# Evidence: Cover the bind mountpoint argument with unit tests and a mount guard

Date: 2026-11-06
Goal: `bind-mountpoint-arg`
Issue: #22
Branch: `fix/lock-swarm-coherence`

## Finding

`cmd/fs-relay-bind/main.go` read `flags.Arg(0)` inline in three places without an extracted helper or regression test covering the parsed `flag.FlagSet` positional argument.

## Fix

| Change | File |
|--------|------|
| Extract `mountArg(fs *flag.FlagSet) (string, string, error)` returning `(mnt, misplacedFlag, errUsage)` | `cmd/fs-relay-bind/cli.go` |
| Call `mountArg(flags)` once in `run()`, reuse `mnt`, and assert `mnt != ""` before `fs.Mount` | `cmd/fs-relay-bind/main.go` |
| Add `TestMountArg` table-testing single mountpoint, flags before mountpoint, missing/empty positional, flag after positional, and extra positional | `cmd/fs-relay-bind/cli_test.go` |
| Add mount verification check to the smoke checklist | `README.md` |

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
  "goal_slug": "bind-mountpoint-arg",
  "prime": {"ran": true},
  "recall": [{"query": "cli extra args bind mountpoint sigint busy multi serve", "matches": 5}],
  "memos": [],
  "lessons": [{"id": "fact_361e200b"}],
  "cases": [],
  "facts_verify": {"exit_code": 0},
  "handoff": {"id": "hoff_af4f3dc0607f"}
}
```

Verdict: PASS
