# Plan: Guard the data directory with an exclusive lock unless forced

Read `facts.md` and `goal.md`.

## Execution Steps

- [x] **T1 - Implement `(*Tree).Lock()` and release in `(*Tree).Close()` (`AC-001`)**
  - CWD: `/mnt/Jad/github/lab/fs-relay`
  - Edit: `internal/local/tree.go` (`Tree`, `(*Tree).Lock`, `(*Tree).Close`).
  - Requirements:
    1. Add `lockFile *os.File` to `Tree` guarded by `t.fmu`.
    2. Implement `(*Tree).Lock() error` to open `filepath.Join(t.root, ".fs-relay.lock")` with `os.O_CREATE|os.O_RDWR` and mode `0o600`.
    3. Acquire `unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)`. On error, read the file, close `f`, and return `fmt.Errorf("data directory %s is already served by pid %s", t.root, pid)` (or without PID if empty).
    4. Truncate `f` to `0`, write `fmt.Fprintf(f, "%d\n", os.Getpid())`, and store `f` in `t.lockFile` under `t.fmu` (closing any prior `t.lockFile`).
    5. In `(*Tree).Close()`, clear `t.lockFile` under `t.fmu` and join `lf.Close()` into the returned error without unlinking `.fs-relay.lock`.
  - Verification: `CGO_ENABLED=0 go test -race ./internal/local` (expected exit `0`).

- [x] **T2 - Wire `-force-multi-serve`, update docs, and add unit tests (`AC-001`, `AC-002`, `AC-003`)**
  - CWD: `/mnt/Jad/github/lab/fs-relay`
  - Edit: `cmd/fs-relay/main.go`, `internal/local/local_test.go`, `SPEC.md`, `ROADMAP.md`, `README.md`.
  - Requirements:
    1. Add `forceMulti := flags.Bool("force-multi-serve", false, "serve a data directory that another fs-relay is already using")` in `cmd/fs-relay/main.go` and call `tree.Lock()` after `tree.Prepare()` when `!*forceMulti`.
    2. Add `TestTreeLockRefusesSecondServerAndReleasesOnClose` in `internal/local/local_test.go` with doc comment.
    3. Update `SPEC.md`, `ROADMAP.md`, and `README.md` to record the data directory lock and `-force-multi-serve`.
  - Verification: `gofmt -l . && go vet ./... && CGO_ENABLED=0 go test -race ./...` (expected exit `0`, empty `gofmt` output).

- [x] **T3 - Record verification evidence and audit (`AC-001`, `AC-002`, `AC-003`)**
  - CWD: `/mnt/Jad/github/lab/fs-relay`
  - Edit: `goals/single-serve-data-lock/evidence.md`.
  - Verification: `sila goals audit single-serve-data-lock && sila gate` (expected exit `0`, `Verdict: PASS`).
