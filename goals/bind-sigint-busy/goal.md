# Goal: Exit cleanly or with a busy error on SIGINT in fs-relay-bind

## Chain of Custody (Isnad)
- **Registered By**: @golang-implementer
- **Spec Audited By**: @golang-implementer
- **Implemented By**: @golang-implementer
- **Mystery Audited By**: @mystery-auditor | Report: `receipt_bind-sigint-busy_1793936114`
## Goal Description
Issue #4 reports that `fs-relay-bind` hangs indefinitely on `SIGINT` or `SIGTERM` when a file on the mount is open. `unmountOnSignal` in `cmd/fs-relay-bind/main.go` reads one signal, calls `_ = server.Unmount()`, and exits its goroutine. When a file on the mount is open, `server.Unmount()` fails with `EBUSY`, `server.Wait()` blocks forever, and a second `SIGINT` is ignored.

Replace the fire-and-forget `unmountOnSignal` goroutine with `waitMount(mnt string, srv mountServer, detach func(string) error, sigc <-chan os.Signal) error`:
1. If `srv.Wait()` finishes on its own (external unmount), return `nil`.
2. On the first signal, call `srv.Unmount()`. If it succeeds, wait for `srv.Wait()` (or a second signal) and return `nil`.
3. If `srv.Unmount()` fails (e.g. `EBUSY` from an open file) or a second signal arrives while unmounting, attempt a lazy detach (`fusermount3 -uz <mnt>` with `syscall.Unmount(mnt, syscall.MNT_DETACH)` fallback) and return `fmt.Errorf("mount %s is busy; close open files or run fusermount3 -uz %s", mnt, mnt)` so `main()` prints the error to `os.Stderr` and exits `1`.

## Dependencies & Execution Order
- **Mode**: Sequential
- **Depends On**: bind-mountpoint-arg
- **Sequence**: 15
- **Shape**: ship
- **Tier**: roadmap
- **Memory Protocol**: required

## Files to Touch
- cmd/fs-relay-bind/main.go
- cmd/fs-relay-bind/cli.go
- cmd/fs-relay-bind/cli_test.go

## Decisions fixed before dispatch

| Decision | Value | Reason |
|----------|-------|--------|
| Consumer seam | `mountServer` interface (`Unmount() error`, `Wait()`) in `cmd/fs-relay-bind/cli.go` | Allows deterministic table tests without a live FUSE mount (`AGENTS.md` seam rule) |
| Busy unmount action | Run lazy detach (`fusermount3 -uz <mnt>`) and return `mount <mnt> is busy; close open files or run fusermount3 -uz <mnt>` | Matches #4 Suggested Fix ("On EBUSY, try a lazy unmount and exit 1 with something like mount is busy; close open files or run fusermount3 -uz /tmp/mnt") |
| Second signal | Interrupts a blocked `Unmount()` or `Wait()`, detaches lazily, and returns the busy error (exit `1`) | Matches #4 ("Treat a second signal as a forced exit") |

## References
- **Shared Understanding & Fact Sheet**: [`goals/bind-sigint-busy/facts.md`](facts.md)
- **Execution Plan**: [`goals/bind-sigint-busy/plan.md`](plan.md)
- Issue #4

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
