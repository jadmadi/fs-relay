# Plan: Guard the data directory with an exclusive lock unless forced

Read `facts.md`.

## Execution Steps

- [x] **T1 - Implement `(*Tree).Lock()` and release in `(*Tree).Close()`**
  - Files: `internal/local/tree.go`.
  - Requirements: open `<root>/.fs-relay.lock` with `0600`, acquire `unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)`, write `<pid>\n`, store `lockFile` on `Tree`, close it in `Close()`, and return `data directory <root> is already served by pid <pid>` when already locked.
  - Acceptance: a second `Tree.Lock()` on the same root fails with the holder PID until the first `Tree.Close()` runs.

- [x] **T2 - Wire `-force-multi-serve` in `cmd/fs-relay/main.go` and add unit tests**
  - Files: `cmd/fs-relay/main.go`, `internal/local/local_test.go`, `README.md`, `ROADMAP.md`.
  - Requirements: add `-force-multi-serve` flag in `cmd/fs-relay/main.go`; call `tree.Lock()` unless `*forceMulti`; add `TestTreeLockRefusesSecondServerAndReleasesOnClose` in `internal/local/local_test.go`; document `-force-multi-serve` in `README.md` and `ROADMAP.md`.
  - Acceptance: `go test -race ./...` passes and `gopls check` prints nothing.

- [x] **T3 - Evidence**
  - Files: `goals/single-serve-data-lock/evidence.md`.
  - Acceptance: `gofmt -l .` empty, `go vet ./...` and `go test -race ./...` pass; `evidence.md` has `Verdict: PASS`.
