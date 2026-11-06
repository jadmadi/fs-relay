# Goal: Exit cleanly or with a busy error on SIGINT in fs-relay-bind

## Chain of Custody (Isnad)
- **Registered By**: @golang-implementer
- **Spec Audited By**: @golang-implementer
- **Implemented By**: @golang-implementer
- **Mystery Audited By**: @mystery-auditor | Report: `receipt_bind-sigint-busy_1793986053`
## Goal Description
Replace the fire-and-forget `unmountOnSignal` goroutine with `waitMount(mnt string, srv mountServer, detach func(string) error, sigc <-chan os.Signal) error` so `fs-relay-bind` exits `0` on a clean unmount and lazily detaches (`fusermount3 -uz <mnt>`) and returns a descriptive busy error (exit `1`) on `EBUSY` or a second signal.

## Dependencies & Execution Order
- **Mode**: Sequential
- **Depends On**: bind-mountpoint-arg
- **Sequence**: 15
- **Shape**: ship
- **Tier**: roadmap
- **Memory Protocol**: required

## Context
- **Goal ID**: `bind-sigint-busy` (Closes Upstream Issue `#4`)
- **One-line why**: `fs-relay-bind` hangs indefinitely on `SIGINT` or `SIGTERM` when a file on the mount is open because `unmountOnSignal` reads one signal, ignores `EBUSY` from `server.Unmount()`, and exits while `server.Wait()` blocks forever.
- **Repository / cwd**: `/mnt/Jad/github/lab/fs-relay` (`muayyad-alsadi/fs-relay`)
- **Prerequisite task IDs**: `bind-mountpoint-arg`
- **Readiness**: READY (Await upstream issue `#4` assignment to `@jadmadi` before opening PR)

## Files to Touch
- cmd/fs-relay-bind/main.go
- cmd/fs-relay-bind/cli.go
- cmd/fs-relay-bind/cli_test.go

## File & Symbol Scope
- **Edit**:
  - `cmd/fs-relay-bind/cli.go` (`mountServer`, `lazyUnmount`, `waitMount`)
  - `cmd/fs-relay-bind/main.go` (`run`, remove `unmountOnSignal`)
  - `cmd/fs-relay-bind/cli_test.go` (`fakeMount`, `TestWaitMount`)
- **Create**: none
- **Read**:
  - `AGENTS.md` (`## Concurrency`, `## Types and seams`, `## Errors`)

## Inline Reference Excerpts

### `cmd/fs-relay-bind/cli.go`
```go
type mountServer interface {
	Unmount() error
	Wait()
}

// lazyUnmount detaches a busy FUSE mountpoint so it clears when open files close.
func lazyUnmount(mnt string) error {
	if err := exec.Command("fusermount3", "-uz", mnt).Run(); err == nil {
		return nil
	}
	return syscall.Unmount(mnt, syscall.MNT_DETACH)
}

// waitMount waits for a FUSE server to exit or unmounts it on SIGINT/SIGTERM.
// When the mount is busy or a second signal arrives, it detaches lazily and
// returns an error so the process exits non-zero instead of hanging.
func waitMount(mnt string, srv mountServer, detach func(string) error, sigc <-chan os.Signal) error {
...
```

### `cmd/fs-relay-bind/main.go`
```go
	log.Printf("fs-relay-bind: mounted on %s, namespace %s, %s, %s", mnt, *ns, mode, base)
	sigc := make(chan os.Signal, 2)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigc)
	return waitMount(mnt, server, lazyUnmount, sigc)
```

## Decisions & Requirements Fixed Before Dispatch

| Decision | Fixed Value | Rationale |
|----------|-------------|-----------|
| Consumer seam | `type mountServer interface { Unmount() error; Wait() }` in `cmd/fs-relay-bind/cli.go` | Smallest consumer interface for `*fs.Server` and `fakeMount` (`AGENTS.md` seam rule) |
| Busy unmount action | Call `detach(mnt)` (`fusermount3 -uz <mnt>` with `syscall.Unmount(mnt, syscall.MNT_DETACH)` fallback) and return `fmt.Errorf("mount %s is busy; close open files or run fusermount3 -uz %s", mnt, mnt)` | Matches Issue `#4` Suggested Fix (`On EBUSY, try a lazy unmount and exit 1 with something like mount is busy; close open files or run fusermount3 -uz /tmp/mnt`) |
| Second signal behavior | Interrupts a blocked `Unmount()` or post-unmount `Wait()`, calls `detach(mnt)`, and returns the busy error (exit `1`) | Matches Issue `#4` (`Treat a second signal as a forced exit`) |
| Signal cleanup | `defer signal.Stop(sigc)` in `run()` | Prevents leaking signal notification registrations |

1. Define `mountServer`, `lazyUnmount(mnt string) error`, and `waitMount(mnt string, srv mountServer, detach func(string) error, sigc <-chan os.Signal) error` in `cmd/fs-relay-bind/cli.go`.
2. Replace `go unmountOnSignal(server)` and `server.Wait()` in `cmd/fs-relay-bind/main.go` with `sigc := make(chan os.Signal, 2)`, `signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)`, `defer signal.Stop(sigc)`, and `return waitMount(mnt, server, lazyUnmount, sigc)`.
3. Delete `unmountOnSignal` from `cmd/fs-relay-bind/main.go`.
4. Add `TestWaitMount` in `cmd/fs-relay-bind/cli_test.go` covering:
   - `external-unmount` (`Wait()` returns without signal -> `nil`),
   - `clean-signal-unmount` (first `SIGINT` -> `Unmount()` returns `nil` and closes `Wait()` -> `nil`, `detach` not called),
   - `busy-unmount-detaches-and-returns-error` (first `SIGINT` -> `Unmount()` returns `syscall.EBUSY` -> `detach("/tmp/mnt")` called and error contains `mount /tmp/mnt is busy; close open files or run fusermount3 -uz /tmp/mnt`),
   - `second-signal-forces-exit` (first `SIGINT` blocks in `Unmount()`, second `SIGINT` arrives -> `detach("/tmp/mnt")` called and busy error returned immediately).

## Interfaces / Signatures (Fixed)
```go
// In cmd/fs-relay-bind/cli.go:
type mountServer interface {
	Unmount() error
	Wait()
}

// lazyUnmount detaches a busy FUSE mountpoint so it clears when open files close.
func lazyUnmount(mnt string) error

// waitMount waits for a FUSE server to exit or unmounts it on SIGINT/SIGTERM.
// When the mount is busy or a second signal arrives, it detaches lazily and
// returns an error so the process exits non-zero instead of hanging.
func waitMount(mnt string, srv mountServer, detach func(string) error, sigc <-chan os.Signal) error
```

## Constraints and Non-Goals
- **Five-package ceiling**: Do not add a sixth Go package (`AGENTS.md`).
- **Zero `time.Sleep` in tests**: Coordinate goroutines in `TestWaitMount` with channels (`entered`, `waitCh`), never `time.Sleep` (`AGENTS.md`).
- **No goroutine leaks in tests**: Always close `waitCh` via `defer close(waitCh)` in test cases where `Unmount` fails or blocks so the background `srv.Wait()` goroutine exits cleanly before the test finishes.

## Concrete Input / Output Examples & Boundary Cases
- **Example 1 (Clean signal unmount)**:
  - Input: `sigc <- syscall.SIGINT`; `srv.Unmount()` succeeds and unblocks `srv.Wait()`.
  - Output: `waitMount` returns `nil` without calling `detach`.
- **Example 2 (Busy mount `EBUSY`)**:
  - Input: `sigc <- syscall.SIGINT`; `srv.Unmount()` returns `syscall.EBUSY`.
  - Output: `detach("/tmp/mnt")` is called once; `waitMount` returns `mount /tmp/mnt is busy; close open files or run fusermount3 -uz /tmp/mnt`; `main()` prints `fs-relay-bind: mount /tmp/mnt is busy; close open files or run fusermount3 -uz /tmp/mnt` to `os.Stderr` and exits `1`.
- **Boundary Case (Second signal while `srv.Unmount()` is blocked in kernel)**:
  - Input: First `SIGINT` starts `srv.Unmount()` which blocks; second `SIGINT` arrives on `sigc`.
  - Expected: `waitMount` does not wait for `srv.Unmount()` to return; it immediately runs `detach(mnt)` and returns the busy error.

## Acceptance Criteria
- **AC-001 (Unit test & race detector)**:
  - Command (`cwd`: `/mnt/Jad/github/lab/fs-relay`): `CGO_ENABLED=0 go test -race -v -run TestWaitMount ./cmd/fs-relay-bind`
  - Expected: Exit `0`; all 4 subtests in `TestWaitMount` pass with zero race warnings.
- **AC-002 (Negative busy unmount path)**:
  - Covered by subtest `TestWaitMount/busy-unmount-detaches-and-returns-error`: `srv.Unmount()` returning `syscall.EBUSY` invokes `detach("/tmp/mnt")` and returns non-nil busy error.
- **AC-003 (Repo hygiene)**:
  - Command (`cwd`: `/mnt/Jad/github/lab/fs-relay`): `test -z "$(gofmt -l .)" && go vet ./... && CGO_ENABLED=0 go test -race ./...`
  - Expected: Exit `0` with empty stdout from `gofmt -l .`.

## If Blocked
Stop affected work; preserve valid changes and report evidence and the decision needed. Do not guess or push an unassigned issue PR.

## Return Schema
```json
{
  "status": "ready_for_review",
  "summary": "string",
  "changed_files": ["cmd/fs-relay-bind/cli.go", "cmd/fs-relay-bind/main.go", "cmd/fs-relay-bind/cli_test.go"],
  "case_results": [{"id": "AC-001", "status": "pass"}, {"id": "AC-002", "status": "pass"}, {"id": "AC-003", "status": "pass"}],
  "commands_exits": [{"command": "CGO_ENABLED=0 go test -race ./...", "exit_code": 0}],
  "evidence": "goals/bind-sigint-busy/evidence.md",
  "candidate": "commit-sha",
  "blocker": null
}
```

## References
- **Shared Understanding & Fact Sheet**: [`goals/bind-sigint-busy/facts.md`](facts.md)
- **Execution Plan**: [`goals/bind-sigint-busy/plan.md`](plan.md)
- Upstream Issue: `https://github.com/muayyad-alsadi/fs-relay/issues/4`

## Memory Plan
- **Prime**: run `sila prime` at task start.
- **Recall**: run `sila recall` before implementation.
- **Writes**: record a grounded `lesson` or `memo` for `waitMount`.
- **Facts**: run `sila facts verify` before certification.
- **Handoff**: run `sila handoff` at session end.
- **Evidence**: embed a `sila/memory-writes/v1` block under `## Memory Writes`.

## Done Condition
1. All tasks in `plan.md` are checked (`- [x]`).
2. Implementation is committed with `(goals/bind-sigint-busy/goal.md)` attribution.
3. Goal is certified (`goals/bind-sigint-busy/certification.json` and `audit-receipt.json`) and `evidence.md` contains a standalone `Verdict: PASS` line.
4. `sila goals --json` reports `status: "implemented"`, `verification_status: "verified"`, and `unverified_count: 0`.
