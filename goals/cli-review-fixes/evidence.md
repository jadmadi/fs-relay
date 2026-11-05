# Evidence: Close the CLI review findings on PR 19

Date: 2026-11-03
Goal: `cli-review-fixes`
PR: #19, commit `baf60d6`
Scope: `cmd/fs-relay`, `cmd/fs-relay-bind`, `justfile`, `README.md`.

## What was built

1. Both `run()` functions use a `flag.FlagSet` with `ContinueOnError`.
   `flag.ErrHelp` prints help to stdout and exits 0; another parse error
   prints the error and help to stderr and exits 2. The argv pre-scan is
   gone, so `-token -h` is a token value.
2. Help includes a purpose, usage, example and a `Version:` line.
3. The health JSON carries `version` beside `status`, `time_ms` and `time`.
4. `buildVersion()` prefers `main.version` from
   `-ldflags -X main.version=...`, then `debug.ReadBuildInfo`.
5. `just` stamps `git describe --tags --always --dirty` into both binaries.

## Acceptance results

| Check | Result |
|-------|--------|
| AC-1.1 `-token -h` is a token error | `fs-relay: token must be at least 8 characters`, exit 1 |
| AC-1.2 `-h` prints help to stdout | exit 0, stdout 506 B, stderr 0 B |
| AC-1.3 `-bogus` exits 2 | exit 2, usage on stderr |
| AC-1.4 same for bind | pass |
| AC-2.1 help has the version | `Version:\n  fs-relay v0.1.2-14-g6e75321-dirty` |
| AC-2.2 bind help has the version | `Version:\n  fs-relay-bind ...` |
| AC-3.1 health has four keys | `{"status":"ok","version":"...","time_ms":...,"time":"..."}` |
| AC-3.2 version is non-empty | pass |
| AC-4.1 `just build` stamps the version | `./fs-relay -version` prints the describe value |
| AC-4.2 a tag prints the tag | `git describe` supplies it |
| AC-4.3 dirty worktree is marked | `-dirty` observed |
| AC-5.1 checks | gofmt empty, vet, test and race pass |
| AC-5.2 PR updated | #19 `test` green |

## Commands

```
gofmt -l .
go vet ./...
go test ./...
go test -race ./...
just build
./fs-relay -h
./fs-relay -token -h
./fs-relay -bogus
./fs-relay -version
curl -s http://127.0.0.1:<port>/
```

## Notes

- `sila facts verify` exits 1 because two files from another project are
  missing from the ledger; the lock and CLI evidence is intact.

- `just build` on a dirty worktree appends `-dirty`; that is intended.
- The help text lists the flags from the FlagSet, so a future flag appears
  without editing the prose.
- The bind's usage error returns a sentinel and `main` exits 2, keeping the
  previous exit code.

## Memory Writes

```json
{
  "schema": "sila/memory-writes/v1",
  "goal_slug": "cli-review-fixes",
  "prime": {"ran": true},
  "recall": [{"query": "flag help value", "matches": 0}],
  "memos": [{"id": "fact_63405e01"}],
  "lessons": [{"id": "fact_9df835da"}],
  "cases": [],
  "facts_verify": {"exit_code": 1},
  "handoff": {"id": "session-end"}
}
```

Verdict: PASS
