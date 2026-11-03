# Goal: Close the CLI review findings on PR 19

## Goal Description

PR #19 implements #9 (`-version`), #10 (readable help) and #11 (flag-order
hint). Qodo's review found four gaps in that PR:

1. Help does not print the build version.
2. The health JSON does not carry the deployed version.
3. A help-like value, for example `-token -h`, is mistaken for help and
   stops the process.
4. A build from a tagged checkout reports `(devel)` instead of the tag.

Fix these inside PR #19. Do not widen beyond #9, #10 and #11.

## Dependencies & Execution Order

- **Mode**: Independent
- **Depends On**: none
- **Sequence**: 1
- **Shape**: ship
- **Tier**: roadmap
- **Memory Protocol**: required

## Decisions fixed before dispatch

| Decision | Value | Reason |
|----------|-------|--------|
| Version source | One `versionLine(name)` per binary, plus `var version string` set by `-ldflags -X main.version` | Build info alone cannot see a git tag |
| Release stamp | `just release` passes `git describe --tags --always --dirty` | A tagged build must not say `(devel)` |
| Help parsing | `flag.NewFlagSet` with `ContinueOnError` | A pre-scan cannot tell a value from a flag |
| Help stream | stdout for `-h`, stderr for usage-on-error | #10 asks for pipeable help |
| Usage exit | `2` on a flag or position error, `0` on `-h` | Matches the existing convention |
| Health | `version` field beside `status`, `time_ms`, `time` | One deployed identity |
| Scope | Only #9, #10, #11 and the four review findings | No new flags |

## Out of scope

- #8 (unknown positionals start a server). Not assigned.
- #7 (two servers). Not assigned.
- Any new flag except the version plumbing.
- Changing the default listen address, data directory or token behaviour.

## References

- `cmd/fs-relay/cli.go`, `cmd/fs-relay/cli_test.go`
- `cmd/fs-relay-bind/cli.go`, `cmd/fs-relay-bind/cli_test.go`
- `cmd/fs-relay/root.go` (health)
- `justfile` (`build`, `release`)
- PR #19, Qodo review 2026-11-03 15:25

## Done Condition

1. All tasks in `plan.md` are checked.
2. The PR branch is updated with `(goals/cli-review-fixes/goal.md)`.
3. `gofmt -l .`, `go vet ./...`, `go test ./...` and `go test -race ./...`
   pass.
4. Evidence has a standalone `Verdict: PASS`.
