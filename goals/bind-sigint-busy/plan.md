# Plan: Exit cleanly or with a busy error on SIGINT in fs-relay-bind

Read `facts.md` and `goal.md` before executing any step.

## Execution Steps

- [x] **T1 - Replace `unmountOnSignal` with `waitMount` and `lazyUnmount`**
  - Files: `cmd/fs-relay-bind/cli.go`, `cmd/fs-relay-bind/main.go`.
  - Requirements:
    1. Define `mountServer` (`Unmount() error`, `Wait()`), `waitMount(mnt string, srv mountServer, detach func(string) error, sigc <-chan os.Signal) error`, and `lazyUnmount(mnt string) error` in `cmd/fs-relay-bind/cli.go`.
    2. Wire `waitMount` in `run()` in `cmd/fs-relay-bind/main.go` with `signal.Notify` and `defer signal.Stop(sigc)`, removing `unmountOnSignal`.
  - Acceptance (`AC-001`, `AC-002`): `fs-relay-bind` exits `0` on clean unmount and returns a descriptive busy error (exit `1`) on `EBUSY` or a second signal.

- [x] **T2 - Table-test `waitMount` in `cmd/fs-relay-bind/cli_test.go`**
  - Files: `cmd/fs-relay-bind/cli_test.go`.
  - Requirements:
    1. Add `TestWaitMount(t *testing.T)` covering external unmount, clean signal unmount, busy unmount (`EBUSY` triggers `detach` and returns busy error), and second signal while `Unmount` blocks.
  - Acceptance (`AC-001`): `CGO_ENABLED=0 go test -race -v -run TestWaitMount ./cmd/fs-relay-bind` passes with zero goroutine leaks or races.

- [x] **T3 - Quality gates & Evidence**
  - Files: `goals/bind-sigint-busy/evidence.md`.
  - Acceptance (`AC-003`): `gofmt -l .` empty, `go vet ./...` and `CGO_ENABLED=0 go test -race ./...` pass; `evidence.md` has `Verdict: PASS`.
