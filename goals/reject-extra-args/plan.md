# Plan: Reject unexpected positional arguments in fs-relay

Read `facts.md` and `goal.md` before executing any step.

## Execution Steps

- [x] **T1 - Add `checkNoArgs` and reject positional arguments in `run()`**
  - Files: `cmd/fs-relay/cli.go`, `cmd/fs-relay/main.go`.
  - Requirements:
    1. Define `checkNoArgs(args []string) error` in `cmd/fs-relay/cli.go` returning `nil` when `len(args) == 0` and `fmt.Errorf("unknown argument %q", args[0])` otherwise.
    2. Call `checkNoArgs(flags.Args())` in `run()` in `cmd/fs-relay/main.go` right after `*showVersion`, writing `fmt.Fprintf(os.Stderr, "%v\n\n%s", err, usageText(flags))` and returning `errUsage`.
  - Acceptance (`AC-002`): `fs-relay help` exits `2` with `unknown argument "help"` on stderr without creating `./data` or opening a listener.

- [x] **T2 - Table-test `checkNoArgs` in `cmd/fs-relay/cli_test.go`**
  - Files: `cmd/fs-relay/cli_test.go`.
  - Requirements:
    1. Add `TestCheckNoArgs(t *testing.T)` covering empty args, `["help"]`, `["version"]`, and `["extra", "more"]`.
  - Acceptance (`AC-001`): `CGO_ENABLED=0 go test -race -v -run TestCheckNoArgs ./cmd/fs-relay` passes with exit `0`.

- [x] **T3 - Quality gates & Evidence**
  - Files: `goals/reject-extra-args/evidence.md`.
  - Acceptance (`AC-003`): `gofmt -l .` empty, `go vet ./...` and `CGO_ENABLED=0 go test -race ./...` pass; `evidence.md` has `Verdict: PASS`.
