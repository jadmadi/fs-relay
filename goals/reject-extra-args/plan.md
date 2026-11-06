# Plan: Reject unexpected positional arguments in fs-relay

Read `facts.md`.

## Execution Steps

- [ ] **T1 - Add `checkNoArgs` and reject positional arguments in `run()`**
  - Files: `cmd/fs-relay/cli.go`, `cmd/fs-relay/main.go`.
  - Requirements: define `checkNoArgs(args []string) error` returning `nil` when `len(args) == 0` and `fmt.Errorf("unknown argument %q", args[0])` otherwise; call `checkNoArgs(flags.Args())` in `run()` right after `*showVersion`, writing the error and `usageText(flags)` to `os.Stderr` and returning `errUsage`.
  - Acceptance: `fs-relay help` exits `2` without creating `./data` or opening a listener.

- [ ] **T2 - Table-test `checkNoArgs` in `cmd/fs-relay/cli_test.go`**
  - Files: `cmd/fs-relay/cli_test.go`.
  - Requirements: add `TestCheckNoArgs` covering empty args, `["help"]`, `["version"]`, and `["extra", "more"]`.
  - Acceptance: `go test -race ./cmd/fs-relay` passes and `gopls check ./cmd/fs-relay/*.go` prints nothing.

- [ ] **T3 - Evidence**
  - Files: `goals/reject-extra-args/evidence.md`.
  - Acceptance: `gofmt -l .` empty, `go vet ./...` and `go test -race ./...` pass; `evidence.md` has `Verdict: PASS`.
