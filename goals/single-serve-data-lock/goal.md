# Goal: Guard the data directory with an exclusive lock unless forced

## Chain of Custody (Isnad)
- **Registered By**: @golang-implementer
- **Spec Audited By**: @golang-implementer
- **Implemented By**: @golang-implementer
- **Mystery Audited By**: Pending

## Goal Description
Issue #7 reports that two `fs-relay` servers can serve the same `-data` directory on different ports with separate in-memory inode, handle, lock, and eviction tables. Nothing warns or refuses. The maintainer asked whether the override flag should be `-force-multi-serve` or `-allow-multi-serve`, noting that `-force-*` is the convention for discouraged, sharp-edge behavior (like `-force-existing-mnt` in #12/#20). Because a single Go `fs-relay` process already uses all CPU cores via `GOMAXPROCS` and splitting a `-data` directory across processes breaks inode stability, advisory locks, and cache eviction unless the tree is immutable and mounts are sticky, `-force-multi-serve` is the right flag name.

Add `(*Tree).Lock() error` in `internal/local/tree.go` that opens `<root>/.fs-relay.lock` (`0600`), acquires `unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)`, writes `<pid>\n`, and holds the descriptor until `(*Tree).Close()`. If `Flock` fails with `EWOULDBLOCK`/`EAGAIN`, read the holder PID from `<root>/.fs-relay.lock` and return `fmt.Errorf("data directory %s is already served by pid %s", t.root, pid)`. Add `-force-multi-serve` to `cmd/fs-relay/main.go` and call `tree.Lock()` when `-force-multi-serve` is not set.

## Dependencies & Execution Order
- **Mode**: Sequential
- **Depends On**: reject-extra-args
- **Sequence**: 16
- **Shape**: ship
- **Tier**: roadmap
- **Memory Protocol**: required

## Files to Touch
- internal/local/tree.go
- internal/local/local_test.go
- cmd/fs-relay/main.go
- README.md
- ROADMAP.md

## Decisions fixed before dispatch

| Decision | Value | Reason |
|----------|-------|--------|
| Override flag | `-force-multi-serve` | Matches `-force-existing-mnt`; multi-serving one `-data` directory splits in-memory inode, handle, lock, and eviction state |
| Lock file path | `<data>/.fs-relay.lock` (mode `0600`) | Leading dot is rejected by `wire.CheckNS`, so it never collides with a namespace directory |
| Lock primitive | `unix.Flock(fd, unix.LOCK_EX\|unix.LOCK_NB)` | Per open file description (testable in-process across two `Tree` instances) and released automatically by the kernel on crash |
| API surface | `(*Tree).Lock() error` (separate from `Prepare()`) | Avoids a boolean parameter on `Prepare` (`AGENTS.md` rule) and keeps existing `Prepare()` callers unchanged |

## References
- **Shared Understanding & Fact Sheet**: [`goals/single-serve-data-lock/facts.md`](facts.md)
- **Execution Plan**: [`goals/single-serve-data-lock/plan.md`](plan.md)
- Issue #7

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
