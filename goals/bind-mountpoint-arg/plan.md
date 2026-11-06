# Plan: Cover the bind mountpoint argument with unit tests and a mount guard

Read `facts.md` and `goal.md` before executing any step.

## Execution Steps

- [x] **T1 - Extract `mountArg` and guard `fs.Mount`**
  - Files: `cmd/fs-relay-bind/cli.go`, `cmd/fs-relay-bind/main.go`, `README.md`.
  - Requirements:
    1. Extract `mountArg(fs *flag.FlagSet) (string, string, error)` in `cmd/fs-relay-bind/cli.go`.
    2. Call `mountArg(flags)` in `run()` in `cmd/fs-relay-bind/main.go` and pass `mnt` to `checkMountpoint`, a non-empty check before `fs.Mount`, `fs.Mount`, and the startup log.
    3. Update the `README.md` smoke checklist to confirm `mount` lists the intended directory.
  - Acceptance (`AC-002`): `flags.Arg(0)` is read only inside `mountArg`, and `fs-relay-bind -token local-demo ""` exits `2`.

- [x] **T2 - Table-test `mountArg` in `cmd/fs-relay-bind/cli_test.go`**
  - Files: `cmd/fs-relay-bind/cli_test.go`.
  - Requirements:
    1. Add `TestMountArg(t *testing.T)` covering `["/tmp/mnt"]`, `["-ns", "demo", "/tmp/mnt"]`, `[]`, `[""]`, `["/tmp/mnt", "-post"]`, and `["/tmp/a", "/tmp/b"]`.
  - Acceptance (`AC-001`): `CGO_ENABLED=0 go test -race -v -run TestMountArg ./cmd/fs-relay-bind` passes.

- [x] **T3 - Quality gates & Evidence**
  - Files: `goals/bind-mountpoint-arg/evidence.md`.
  - Acceptance (`AC-003`): `gofmt -l .` empty, `go vet ./...` and `CGO_ENABLED=0 go test -race ./...` pass; `evidence.md` has `Verdict: PASS`.
