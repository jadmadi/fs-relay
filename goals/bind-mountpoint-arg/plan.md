# Plan: Cover the bind mountpoint argument with unit tests and a mount guard

Read `facts.md`.

## Execution Steps

- [x] **T1 - Extract `mountArg` and guard `fs.Mount`**
  - Files: `cmd/fs-relay-bind/cli.go`, `cmd/fs-relay-bind/main.go`, `README.md`.
  - Requirements: extract `mountArg(fs *flag.FlagSet) (string, string, error)` in `cmd/fs-relay-bind/cli.go`; call it in `run()` and pass `mnt` to `checkMountpoint`, a non-empty check before `fs.Mount`, `fs.Mount`, and the startup log; update the `README.md` smoke checklist to confirm `mount` lists the intended directory.
  - Acceptance: `flags.Arg(0)` is read only inside `mountArg`.

- [x] **T2 - Table-test `mountArg` in `cmd/fs-relay-bind/cli_test.go`**
  - Files: `cmd/fs-relay-bind/cli_test.go`.
  - Requirements: add `TestMountArg` covering `["/tmp/mnt"]`, `[]`, `[""]`, `["/tmp/mnt", "-post"]`, `["/tmp/a", "/tmp/b"]`, and `["-ns", "demo", "/tmp/mnt"]`.
  - Acceptance: `go test -race ./cmd/fs-relay-bind` passes and `gopls check ./cmd/fs-relay-bind/*.go` is clean.

- [x] **T3 - Evidence**
  - Files: `goals/bind-mountpoint-arg/evidence.md`.
  - Acceptance: `gofmt -l .` empty, `go vet ./...` and `go test -race ./...` pass; `evidence.md` has `Verdict: PASS`.
