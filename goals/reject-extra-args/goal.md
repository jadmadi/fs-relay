# Goal: Reject unexpected positional arguments in fs-relay

## Chain of Custody (Isnad)
- **Registered By**: @golang-implementer
- **Spec Audited By**: @golang-implementer
- **Implemented By**: @golang-implementer
- **Mystery Audited By**: @mystery-auditor | Report: `receipt_reject-extra-args_1793985901`
## Context
- **Goal ID**: `reject-extra-args` (Closes Upstream Issue `#8`)
- **One-line why**: Running `fs-relay help`, `fs-relay version`, or `fs-relay extra` silently ignores positional arguments, creates `./data`, prints a random token, and blocks listening on `:8080` instead of failing fast with exit status `2`.
- **Repository / cwd**: `/mnt/Jad/github/lab/fs-relay` (`muayyad-alsadi/fs-relay`)
- **Prerequisite task IDs**: none
- **Readiness**: READY (Await upstream issue `#8` assignment to `@jadmadi` before opening PR)

## Files to Touch
- **Edit**:
  - `cmd/fs-relay/cli.go` (`checkNoArgs`)
  - `cmd/fs-relay/main.go` (`run`)
  - `cmd/fs-relay/cli_test.go` (`TestCheckNoArgs`)
- **Create**: none
- **Read**:
  - `SPEC.md` (Section `## Command line`)

## Inline Reference Excerpts

### `cmd/fs-relay/main.go`
```go
	if *showVersion {
		fmt.Println(versionLine("fs-relay"))
		return nil
	}
	if err := checkNoArgs(flags.Args()); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n\n%s", err, usageText(flags))
		return errUsage
	}
```

### `cmd/fs-relay/cli.go`
```go
// checkNoArgs rejects unexpected positional arguments passed to fs-relay.
func checkNoArgs(args []string) error {
	if len(args) == 0 {
		return nil
	}
	return fmt.Errorf("unknown argument %q", args[0])
}
```

## Decisions & Requirements Fixed Before Dispatch

| Decision | Fixed Value | Rationale |
|----------|-------------|-----------|
| Helper signature | `func checkNoArgs(args []string) error` in `cmd/fs-relay/cli.go` | Pure, table-testable without spawning a listener or touching the filesystem |
| Error string | `fmt.Errorf("unknown argument %q", args[0])` | Matches Issue `#8` (`Exit 2 with unknown argument "help" and a hint`) |
| Call site order | Immediately after `if *showVersion` in `run()`, before token resolution or `tree.Prepare()` | Prevents creating `./data`, printing a random secret, or binding `:8080` on bad CLI input |
| Output format | `fmt.Fprintf(os.Stderr, "%v\n\n%s", err, usageText(flags))` | Appends full usage text to `os.Stderr` and returns `errUsage` (exit code `2`) |

1. Define `checkNoArgs(args []string) error` in `cmd/fs-relay/cli.go` with a doc comment; return `nil` when `len(args) == 0` and `fmt.Errorf("unknown argument %q", args[0])` otherwise.
2. Call `checkNoArgs(flags.Args())` in `run()` in `cmd/fs-relay/main.go` right after the `*showVersion` check; on non-nil error, print `fmt.Fprintf(os.Stderr, "%v\n\n%s", err, usageText(flags))` and return `errUsage` so `main()` exits with status `2` without printing a second `fs-relay: usage` line.
3. Add table-driven test `TestCheckNoArgs` in `cmd/fs-relay/cli_test.go` covering `nil`/`[]`, `["help"]`, `["version"]`, and `["extra", "more"]`.

## Interfaces / Signatures (Fixed)
```go
// In cmd/fs-relay/cli.go:
// checkNoArgs rejects unexpected positional arguments passed to fs-relay.
func checkNoArgs(args []string) error
```

## Constraints and Non-Goals
- **Five-package ceiling**: Do not add a sixth Go package (`AGENTS.md`).
- **Zero new dependencies**: Standard library only.
- **No filesystem side effects**: Positional argument rejection must happen before `chooseToken` and `tree.Prepare()` so `./data` is never created on invalid positional arguments.
- **Preserve `-h` and `-version`**: `fs-relay -h` and `fs-relay -version` must continue to print to `os.Stdout` and exit `0`.

## Concrete Input / Output Examples & Boundary Cases
- **Example 1 (Valid flags, zero positional args)**:
  - Input: `checkNoArgs(nil)` or `checkNoArgs([]string{})`
  - Output: `nil`
- **Example 2 (Subcommand-style positional arg)**:
  - Input: `fs-relay help` -> `checkNoArgs([]string{"help"})`
  - Output: `unknown argument "help"` followed by blank line and `usageText(flags)` on `stderr`; exit code `2`; `./data` is not created.
- **Boundary Case (Multiple positional arguments)**:
  - Input: `checkNoArgs([]string{"extra", "more"})`
  - Expected: Reports the first unexpected argument (`unknown argument "extra"`) deterministically.

## Acceptance Criteria
- **AC-001 (Unit test & race detector)**:
  - Command (`cwd`: `/mnt/Jad/github/lab/fs-relay`): `CGO_ENABLED=0 go test -race -v -run TestCheckNoArgs ./cmd/fs-relay`
  - Expected: Exit `0`; `PASS: TestCheckNoArgs` across all subtests (`empty`, `help-word`, `version-word`, `multiple`).
- **AC-002 (Negative CLI execution without side effects)**:
  - Command (`cwd`: `/mnt/Jad/github/lab/fs-relay`): `CGO_ENABLED=0 go build -o /tmp/fs-relay ./cmd/fs-relay && /tmp/fs-relay help`
  - Expected: Exit `2`; stderr starts with `unknown argument "help"` followed by `Usage:`; no `data/` directory is created.
- **AC-003 (Repo hygiene)**:
  - Command (`cwd`: `/mnt/Jad/github/lab/fs-relay`): `test -z "$(gofmt -l .)" && go vet ./... && CGO_ENABLED=0 go test -race ./...`
  - Expected: Exit `0` with empty stdout from `gofmt -l .`.

## If Blocked
Stop affected work; preserve valid changes and report evidence and the decision needed. Do not guess, widen scope, or push an unassigned issue PR.

## Return Schema
```json
{
  "status": "ready_for_review",
  "summary": "string",
  "changed_files": ["cmd/fs-relay/cli.go", "cmd/fs-relay/main.go", "cmd/fs-relay/cli_test.go"],
  "case_results": [{"id": "AC-001", "status": "pass"}, {"id": "AC-002", "status": "pass"}, {"id": "AC-003", "status": "pass"}],
  "commands_exits": [{"command": "CGO_ENABLED=0 go test -race ./...", "exit_code": 0}],
  "evidence": "goals/reject-extra-args/evidence.md",
  "candidate": "commit-sha",
  "blocker": null
}
```

## References
- **Shared Understanding & Fact Sheet**: [`goals/reject-extra-args/facts.md`](facts.md)
- **Execution Plan**: [`goals/reject-extra-args/plan.md`](plan.md)
- Upstream Issue: `https://github.com/muayyad-alsadi/fs-relay/issues/8`

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
