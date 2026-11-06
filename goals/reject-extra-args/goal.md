# Goal: Reject unexpected positional arguments in fs-relay

## Chain of Custody (Isnad)
- **Registered By**: @golang-implementer
- **Spec Audited By**: @golang-implementer
- **Implemented By**: @golang-implementer
- **Mystery Audited By**: @mystery-auditor | Report: `receipt_reject-extra-args_1793935897`
## Goal Description
`fs-relay` parses flags with `flag.NewFlagSet("fs-relay", flag.ContinueOnError)` in `cmd/fs-relay/main.go`, checks `-version`, and then starts the server without checking `flags.NArg()`. Running `fs-relay help`, `fs-relay version`, or `fs-relay extra` silently ignores the positional arguments, creates `./data`, prints a random token, and blocks listening on `:8080`.

Reject any positional arguments (`flags.NArg() > 0`) before token selection or directory preparation by printing `unknown argument "<arg>"` followed by the usage text to `os.Stderr` and returning `errUsage` (exit status `2`).

## Dependencies & Execution Order
- **Mode**: Independent
- **Depends On**: none
- **Sequence**: 13
- **Shape**: ship
- **Tier**: roadmap
- **Memory Protocol**: required

## Files to Touch
- cmd/fs-relay/main.go
- cmd/fs-relay/cli.go
- cmd/fs-relay/cli_test.go

## Decisions fixed before dispatch

| Decision | Value | Reason |
|----------|-------|--------|
| Helper signature | `checkNoArgs(args []string) error` in `cmd/fs-relay/cli.go` | Pure, table-testable without spawning a listener |
| Error message | `unknown argument "<first-arg>"` | Matches #8 ("Exit 2 with unknown argument \"help\" and a hint") |
| Output stream | `os.Stderr` with `usageText(flags)` appended | Matches existing flag error handling in `run()` |
| Exit status | `2` (`errUsage`) | Matches CLI convention in `cmd/fs-relay/main.go` |

## References
- **Shared Understanding & Fact Sheet**: [`goals/reject-extra-args/facts.md`](facts.md)
- **Execution Plan**: [`goals/reject-extra-args/plan.md`](plan.md)
- Issue #8

## Memory Plan
- **Prime**: run `sila prime` at task start.
- **Recall**: run `sila recall` before implementation.
- **Writes**: record a grounded `lesson` or `memo` for positional argument validation.
- **Facts**: run `sila facts verify` before certification.
- **Handoff**: run `sila handoff` at session end.
- **Evidence**: embed a `sila/memory-writes/v1` block under `## Memory Writes`.

## Done Condition
1. All tasks in `plan.md` are checked (`- [x]`).
2. Implementation is committed with `(goals/reject-extra-args/goal.md)` attribution.
3. Goal is certified (`goals/reject-extra-args/certification.json` and `audit-receipt.json`) and `evidence.md` contains a standalone `Verdict: PASS` line.
4. `sila goals --json` reports `status: "implemented"`, `verification_status: "verified"`, and `unverified_count: 0`.
