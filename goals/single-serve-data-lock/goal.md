# Goal: Guard the data directory with an exclusive lock unless forced

## Chain of Custody (Isnad)
- **Registered By**: @golang-implementer
- **Spec Audited By**: @golang-implementer
- **Implemented By**: @golang-implementer
- **Mystery Audited By**: @mystery-auditor | Report: `receipt_single-serve-data-lock_1793987375`
## Goal Description

Prevent two `fs-relay` server processes from accidentally serving the same `-data` directory with split in-memory inode, handle, lock, and eviction state (upstream issue `muayyad-alsadi/fs-relay#7`, assigned to `@jadmadi` and agreed on `-force-multi-serve` in `issuecomment-6680908619`).

Repository and working directory: `/mnt/Jad/github/lab/fs-relay`.

Today, `(*Tree).Prepare()` in `internal/local/tree.go` creates and stats the `-data` root directory, and `run()` in `cmd/fs-relay/main.go` immediately starts serving. A second `fs-relay` pointed at the same `-data` directory starts on another port with its own in-memory `ns` inode maps, `files` handle table, advisory lock records, and pubsub eviction hub.

This goal adds:
1. `(*Tree).Lock() error` in `internal/local/tree.go`, which opens `<root>/.fs-relay.lock` with mode `0600`, acquires `unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)`, truncates and writes `<pid>\n`, stores `lockFile` under `t.fmu`, and releases the descriptor in `(*Tree).Close()`.
2. `-force-multi-serve` boolean flag in `cmd/fs-relay/main.go`, calling `tree.Lock()` after `tree.Prepare()` whenever `!*forceMulti`.
3. Synchronized documentation in `SPEC.md`, `ROADMAP.md`, and `README.md`, plus table-driven and lifecycle unit tests in `internal/local/local_test.go`.

## Dependencies & Execution Order
- **Mode**: Sequential
- **Depends On**: `reject-extra-args`
- **Sequence**: 16
- **Shape**: ship
- **Tier**: roadmap
- **Memory Protocol**: required

## Files to Touch
- internal/local/tree.go
- internal/local/path.go
- internal/local/file.go
- internal/local/local_test.go
- cmd/fs-relay/main.go
- cmd/fs-relay/cli_test.go
- SPEC.md
- ROADMAP.md
- README.md

## File & Symbol Scope
- Edit: `internal/local/tree.go` (`Tree`, `(*Tree).Lock`, `(*Tree).CheckNamespace`, `(*Tree).Close`)
- Edit: `internal/local/path.go` (`(*Tree).abs`)
- Edit: `internal/local/file.go` (`(*Tree).source`, `readCapped`)
- Edit: `internal/local/local_test.go` (`TestTreeLockRefusesSecondServerAndReleasesOnClose`, `TestTreeLockRejectsSymlinkAndProtectsLockFile`)
- Edit: `cmd/fs-relay/main.go` (`main`, `run`)
- Edit: `cmd/fs-relay/cli_test.go` (`TestForceMultiServeFlag`)
- Edit: `SPEC.md` (`## Clients`, `## Locks`)
- Edit: `ROADMAP.md` (`## What is already done`)
- Edit: `README.md` (`## Running it`)
- Create: none
- Read: `AGENTS.md` (`## Concurrency`, `## Errors`, `## Types and seams`, `## Done means`)

## Interfaces / Signatures (Fixed)

```go
// In internal/local/tree.go:
// Lock takes an exclusive non-blocking lock on the data root so a second
// server cannot serve the same directory unless forced.
func (t *Tree) Lock() error

// Close closes every open file and releases the data directory lock.
func (t *Tree) Close() error
```

Required error behavior:
- If opening `<root>/.fs-relay.lock` (`os.O_CREATE|os.O_RDWR`, `0o600`) fails: return `fmt.Errorf("data directory %s: %w", t.root, err)`.
- If `unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)` fails: close `f`, read `<root>/.fs-relay.lock`, trim whitespace, and:
  - when non-empty `pid`: return `fmt.Errorf("data directory %s is already served by pid %s", t.root, pid)`
  - when empty `pid`: return `fmt.Errorf("data directory %s is already served", t.root)`
- If `(*Tree).Lock()` is called again on a `Tree` that already holds `t.lockFile`: close the previous `t.lockFile` under `t.fmu` before storing the new descriptor so no file descriptor leaks.

## Decisions Fixed Before Dispatch

| Decision       | Value                                        | Reason                                                                                              |
| -------------- | -------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| Override flag  | `-force-multi-serve`                         | Confirmed by maintainer in `#7`; matches `-force-existing-mnt` for discouraged sharp-edge behavior |
| Lock file path | `<data>/.fs-relay.lock` (mode `0600`)        | Leading dot is rejected by `wire.CheckNS`, so it never collides with a valid namespace directory   |
| Lock primitive | `unix.Flock(fd, unix.LOCK_EX\|unix.LOCK_NB)` | Per open file description (testable in-process across two `Tree` values) and released on exit/crash |
| API seam       | `(*Tree).Lock() error`                       | Avoids a boolean parameter on `Prepare()` (`AGENTS.md` rule) and leaves existing callers unchanged |
| Mutex discipline| Guard `t.lockFile` with `t.fmu`             | Matches `(*Tree).Close()` lock ownership and satisfies `go test -race ./...`                        |

## Constraints and Non-Goals
- Allowed scope: only the 6 files listed under `## Files to Touch` plus `goals/single-serve-data-lock/*`.
- No new dependencies (`golang.org/x/sys/unix` is already in `go.mod`).
- Do not unlink `.fs-relay.lock` in `Close()`, because unlinking a locked lockfile allows a third process to create a new inode while another process holds the old inode.
- Do not change wire protocol (`internal/wire`) or client mount flags (`cmd/fs-relay-bind`).

## Example & Boundary Case

- Concrete input/output:
  - Process 1 (PID `4120`) runs `fs-relay -data /srv/relay -token secret1234` -> creates `/srv/relay/.fs-relay.lock` (`0600`), flocks it `LOCK_EX|LOCK_NB`, writes `"4120\n"`, and serves.
  - Process 2 runs `fs-relay -data /srv/relay -token secret1234` -> fails before listening with exit code 1 and stderr `fs-relay: data directory /srv/relay is already served by pid 4120`.
  - Process 2 runs `fs-relay -data /srv/relay -force-multi-serve -token secret1234` -> skips `tree.Lock()` and serves.
- Boundary case most likely to be implemented incorrectly:
  - Storing `t.lockFile = f` without `t.fmu.Lock()`, or unlinking `.fs-relay.lock` on `Close()`. Keep the inode in place and release the lock solely by closing `t.lockFile` under `t.fmu`.

## Acceptance Criteria
- `AC-001`: `CGO_ENABLED=0 go test -race ./internal/local -run TestTreeLockRefusesSecondServerAndReleasesOnClose` in `/mnt/Jad/github/lab/fs-relay` exits `0` and verifies that a second `Tree.Lock()` on the same root fails with `"is already served by pid "`, then succeeds after the first `Tree.Close()`.
- `AC-002`: `gofmt -l . && go vet ./... && CGO_ENABLED=0 go test -race ./...` in `/mnt/Jad/github/lab/fs-relay` exits `0` with empty `gofmt` output.
- `AC-003`: Starting a compiled `./fs-relay` against a directory already locked by another `./fs-relay` exits non-zero with `"is already served by pid"` unless `-force-multi-serve` is passed.

## Return Schema

```json
{
  "status": "ready_for_review",
  "summary": "string",
  "changed_files": ["internal/local/tree.go", "internal/local/local_test.go", "cmd/fs-relay/main.go", "SPEC.md", "ROADMAP.md", "README.md"],
  "case_results": [{"id": "AC-001", "status": "PASS"}, {"id": "AC-002", "status": "PASS"}, {"id": "AC-003", "status": "PASS"}],
  "commands": [{"cmd": "go test -race ./...", "exit": 0}],
  "evidence": "goals/single-serve-data-lock/evidence.md",
  "candidate": "feat/single-serve-data-lock",
  "blocker": null
}
```

## References
- **Shared Understanding & Fact Sheet**: [`goals/single-serve-data-lock/facts.md`](facts.md)
- **Execution Plan**: [`goals/single-serve-data-lock/plan.md`](plan.md)
- **Upstream Issue**: `https://github.com/muayyad-alsadi/fs-relay/issues/7`

## Memory Plan
- **Prime**: run `sila prime` at task start.
- **Recall**: run `sila recall` before implementation.
- **Writes**: record a grounded `memo` and `lesson` for `(*Tree).Lock()` and `-force-multi-serve`.
- **Facts**: run `sila facts verify` before certification.
- **Handoff**: run `sila handoff` at session end.
- **Evidence**: embed a `sila/memory-writes/v1` block under `## Memory Writes`.

## Done Condition
1. All tasks in `plan.md` are checked (`- [x]`).
2. Implementation is committed with `(goals/single-serve-data-lock/goal.md)` attribution.
3. Goal is certified (`goals/single-serve-data-lock/certification.json` and `audit-receipt.json`) and `evidence.md` contains a standalone `Verdict: PASS` line.
4. `sila goals --json` reports `status: "implemented"`, `verification_status: "verified"`, and `unverified_count: 0`.
