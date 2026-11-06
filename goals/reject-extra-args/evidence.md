# Evidence: Reject unexpected positional arguments in fs-relay

Date: 2026-11-06
Goal: `reject-extra-args`
Issue: #8
Branch: `fix/lock-swarm-coherence`

## Finding

`run()` in `cmd/fs-relay/main.go` parsed flags with `flags.Parse(os.Args[1:])` and checked `*showVersion`, but never checked `flags.Args()`. Running `fs-relay help`, `fs-relay version`, or `fs-relay extra` ignored the positional arguments, created `./data`, printed a random token, and blocked listening on `127.0.0.1:8080`.

## Fix

| Change | File |
|--------|------|
| Add `checkNoArgs(args []string) error` returning `unknown argument "<arg>"` when `len(args) > 0` | `cmd/fs-relay/cli.go` |
| Call `checkNoArgs(flags.Args())` after `*showVersion`, write the error and `usageText(flags)` to `os.Stderr`, and return `errUsage` (exit `2`) | `cmd/fs-relay/main.go` |
| Add `TestCheckNoArgs` table-testing empty args, `["help"]`, `["version"]`, and `["extra", "more"]` | `cmd/fs-relay/cli_test.go` |

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
  "goal_slug": "reject-extra-args",
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
