# Goal: Cover the bind mountpoint argument with unit tests and a mount guard

## Chain of Custody (Isnad)
- **Registered By**: @golang-implementer
- **Spec Audited By**: @golang-implementer
- **Implemented By**: @golang-implementer
- **Mystery Audited By**: @mystery-auditor | Report: `receipt_bind-mountpoint-arg_1793935904`
## Goal Description
Issue #22 notes that a previous refactor moved `fs-relay-bind` flags to a `flag.FlagSet` while leaving `flag.Arg(0)` reading the global set, which caused `fs-relay-bind` to pass an empty mountpoint and mount FUSE over the current directory. Although #19 and #20 fixed the call site and added `checkMountpoint`, there is no unit test covering the parsed mountpoint argument helper, no non-empty guard immediately before `fs.Mount`, and no smoke-check reminder in `README.md`.

Extract `mountArg(fs *flag.FlagSet) (string, string, error)` in `cmd/fs-relay-bind/cli.go`, call it once in `cmd/fs-relay-bind/main.go` and reuse the returned `mnt` variable through `checkMountpoint`, a non-empty guard before `fs.Mount`, and the startup log line, table-test `mountArg` in `cmd/fs-relay-bind/cli_test.go`, and add the mountpoint check to `README.md`.

## Dependencies & Execution Order
- **Mode**: Independent
- **Depends On**: none
- **Sequence**: 14
- **Shape**: ship
- **Tier**: roadmap
- **Memory Protocol**: required

## Files to Touch
- cmd/fs-relay-bind/main.go
- cmd/fs-relay-bind/cli.go
- cmd/fs-relay-bind/cli_test.go
- README.md

## Decisions fixed before dispatch

| Decision | Value | Reason |
|----------|-------|--------|
| Helper signature | `mountArg(fs *flag.FlagSet) (string, string, error)` | Tests the parsed `flag.FlagSet` directly so reading global `flag.Arg(0)` cannot recur |
| Return values | `(mnt, misplacedFlag, errUsage)` | Preserves the existing misplaced-flag hint and exit code `2` |
| Pre-mount guard | Refuse `mnt == ""` before `fs.Mount` | Second line of defense required by #22 |
| Smoke check doc | Note in `README.md` `## Run` smoke paragraph | Requested by #22 ("confirm mount shows the intended directory, not an empty path") |

## References
- **Shared Understanding & Fact Sheet**: [`goals/bind-mountpoint-arg/facts.md`](facts.md)
- **Execution Plan**: [`goals/bind-mountpoint-arg/plan.md`](plan.md)
- Issue #22

## Memory Plan
- **Prime**: run `sila prime` at task start.
- **Recall**: run `sila recall` before implementation.
- **Writes**: record a grounded `lesson` or `memo` for `mountArg`.
- **Facts**: run `sila facts verify` before certification.
- **Handoff**: run `sila handoff` at session end.
- **Evidence**: embed a `sila/memory-writes/v1` block under `## Memory Writes`.

## Done Condition
1. All tasks in `plan.md` are checked (`- [x]`).
2. Implementation is committed with `(goals/bind-mountpoint-arg/goal.md)` attribution.
3. Goal is certified (`goals/bind-mountpoint-arg/certification.json` and `audit-receipt.json`) and `evidence.md` contains a standalone `Verdict: PASS` line.
4. `sila goals --json` reports `status: "implemented"`, `verification_status: "verified"`, and `unverified_count: 0`.
