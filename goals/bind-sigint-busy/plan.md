# Plan: Exit cleanly or with a busy error on SIGINT in fs-relay-bind

Read `facts.md`.

## Execution Steps

- [x] **T1 - Replace `unmountOnSignal` with `waitMount` and `lazyUnmount`**
  - Files: `cmd/fs-relay-bind/cli.go`, `cmd/fs-relay-bind/main.go`.
  - Requirements: define `mountServer` (`Unmount() error`, `Wait()`), `waitMount(mnt string, srv mountServer, detach func(string) error, sigc <-chan os.Signal) error`, and `lazyUnmount(mnt string) error`; wire `waitMount` in `run()` with `signal.Notify` and `defer signal.Stop(sigc)`.
  - Acceptance: `fs-relay-bind` exits `0` on clean unmount and returns a descriptive busy error (exit `1`) on `EBUSY` or a second signal.

- [x] **T2 - Table-test `waitMount` in `cmd/fs-relay-bind/cli_test.go`**
  - Files: `cmd/fs-relay-bind/cli_test.go`.
  - Requirements: add `TestWaitMount` covering external unmount, clean signal unmount, busy unmount (`EBUSY` triggers `detach` and returns busy error), and second signal while `Unmount` blocks.
  - Acceptance: `go test -race ./cmd/fs-relay-bind` passes with zero goroutine leaks or races.

- [x] **T3 - Evidence**
  - Files: `goals/bind-sigint-busy/evidence.md`.
  - Acceptance: `gofmt -l .` empty, `go vet ./...` and `go test -race ./...` pass; `evidence.md` has `Verdict: PASS`.
