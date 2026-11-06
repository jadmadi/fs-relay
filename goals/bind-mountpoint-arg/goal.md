# Goal: Cover the bind mountpoint argument with unit tests and a mount guard

## Chain of Custody (Isnad)
- **Registered By**: @golang-implementer
- **Spec Audited By**: @golang-implementer
- **Implemented By**: @golang-implementer
- **Mystery Audited By**: @mystery-auditor | Report: `receipt_bind-mountpoint-arg_1793986049`
## Goal Description
Extract `mountArg(fs *flag.FlagSet) (string, string, error)` in `cmd/fs-relay-bind/cli.go`, call it once in `cmd/fs-relay-bind/main.go` and reuse the returned `mnt` variable through `checkMountpoint`, a non-empty guard before `fs.Mount`, and the startup log line, table-test `mountArg` in `cmd/fs-relay-bind/cli_test.go`, and add the mountpoint check to `README.md`.

## Dependencies & Execution Order
- **Mode**: Independent
- **Depends On**: none
- **Sequence**: 14
- **Shape**: ship
- **Tier**: roadmap
- **Memory Protocol**: required

## Context
- **Goal ID**: `bind-mountpoint-arg` (Closes Upstream Issue `#22`)
- **One-line why**: A previous refactor moved `fs-relay-bind` flags to a local `flag.FlagSet` while leaving `flag.Arg(0)` reading the global set, causing `fs-relay-bind` to mount over the current working directory (`""`); extracting `mountArg(fs *flag.FlagSet)` with unit tests and adding a pre-mount non-empty guard prevents any recurrence.
- **Repository / cwd**: `/mnt/Jad/github/lab/fs-relay` (`muayyad-alsadi/fs-relay`)
- **Prerequisite task IDs**: none
- **Readiness**: READY (Await upstream issue `#22` assignment to `@jadmadi` before opening PR)

## Files to Touch
- cmd/fs-relay-bind/main.go
- cmd/fs-relay-bind/cli.go
- cmd/fs-relay-bind/cli_test.go
- README.md

## File & Symbol Scope
- **Edit**:
  - `cmd/fs-relay-bind/cli.go` (`mountArg`)
  - `cmd/fs-relay-bind/main.go` (`run`)
  - `cmd/fs-relay-bind/cli_test.go` (`TestMountArg`)
  - `README.md` (`## Run` smoke checklist)
- **Create**: none
- **Read**:
  - `SPEC.md` (`## Command line`)

## Inline Reference Excerpts

### `cmd/fs-relay-bind/cli.go`
```go
// mountArg extracts the single mountpoint positional argument from a parsed
// FlagSet, returning any misplaced trailing flag and errUsage when missing or extra.
func mountArg(fs *flag.FlagSet) (string, string, error) {
	if fs.NArg() != 1 || fs.Arg(0) == "" {
		return "", misplacedFlag(fs.Args()), errUsage
	}
	return fs.Arg(0), "", nil
}
```

### `cmd/fs-relay-bind/main.go`
```go
	mnt, bad, err := mountArg(flags)
	if err != nil {
		if bad != "" {
			fmt.Fprintf(os.Stderr, "flags must come before the mountpoint: %s\n", bad)
		}
		fmt.Fprint(os.Stderr, usageText(flags))
		return err
	}
...
	if mnt == "" {
		return fmt.Errorf("mountpoint is empty")
	}
	server, err := fs.Mount(mnt, root, mountOptions(*allowOther, *debug))
```

## Decisions & Requirements Fixed Before Dispatch

| Decision | Fixed Value | Rationale |
|----------|-------------|-----------|
| Helper signature | `func mountArg(fs *flag.FlagSet) (string, string, error)` in `cmd/fs-relay-bind/cli.go` | Tests the parsed `*flag.FlagSet` directly so reading global `flag.Arg(0)` cannot recur |
| Return values | `(mnt, misplacedFlag, errUsage)` | Preserves the existing `flags must come before the mountpoint: <flag>` message and exit code `2` |
| Single source of truth | `mnt` returned by `mountArg(flags)` is passed to `checkMountpoint(mnt, ...)`, `fs.Mount(mnt, ...)`, `log.Printf`, and `waitMount(mnt, ...)` | Eliminates repeated `flags.Arg(0)` calls across `run()` |
| Pre-mount guard | `if mnt == "" { return fmt.Errorf("mountpoint is empty") }` immediately before `fs.Mount` | Second line of defense requested by Issue `#22` |
| Smoke check doc | Note in `README.md` `## Run` smoke paragraph | Requested by Issue `#22` (`confirm mount shows the intended directory, not an empty path`) |

1. Define `mountArg(fs *flag.FlagSet) (string, string, error)` in `cmd/fs-relay-bind/cli.go` with a doc comment; return `("", misplacedFlag(fs.Args()), errUsage)` when `fs.NArg() != 1 || fs.Arg(0) == ""`, and `(fs.Arg(0), "", nil)` otherwise.
2. Call `mountArg(flags)` once in `run()` in `cmd/fs-relay-bind/main.go`, reuse `mnt` everywhere, and guard `if mnt == ""` immediately before `fs.Mount`.
3. Add table-driven test `TestMountArg` in `cmd/fs-relay-bind/cli_test.go` covering single mountpoint, flags before mountpoint, no positional arg, empty string positional arg, flag after positional arg, and extra positional arg.
4. Update `README.md` under `## Run` to mention checking `mount` output shows the intended directory.

## Interfaces / Signatures (Fixed)
```go
// In cmd/fs-relay-bind/cli.go:
// mountArg extracts the single mountpoint positional argument from a parsed
// FlagSet, returning any misplaced trailing flag and errUsage when missing or extra.
func mountArg(fs *flag.FlagSet) (string, string, error)
```

## Constraints and Non-Goals
- **Five-package ceiling**: Do not add a sixth Go package (`AGENTS.md`).
- **Zero new dependencies**: Standard library only.
- **No FUSE mount in unit tests**: `TestMountArg` must test `mountArg` in-process without mounting FUSE (`AGENTS.md`).

## Concrete Input / Output Examples & Boundary Cases
- **Example 1 (Flags before mountpoint)**:
  - Input: `fs.Parse([]string{"-ns", "demo", "/tmp/mnt"})` -> `mountArg(fs)`
  - Output: `("/tmp/mnt", "", nil)`
- **Example 2 (Misplaced flag after mountpoint)**:
  - Input: `fs.Parse([]string{"/tmp/mnt", "-post"})` -> `mountArg(fs)`
  - Output: `("", "-post", errUsage)`
- **Boundary Case (Explicitly empty mountpoint `""`)**:
  - Input: `fs.Parse([]string{""})` -> `mountArg(fs)`
  - Expected: Rejected with `("", "", errUsage)` even though `fs.NArg() == 1`.

## Acceptance Criteria
- **AC-001 (Unit test & race detector)**:
  - Command (`cwd`: `/mnt/Jad/github/lab/fs-relay`): `CGO_ENABLED=0 go test -race -v -run TestMountArg ./cmd/fs-relay-bind`
  - Expected: Exit `0`; all 6 subtests in `TestMountArg` pass.
- **AC-002 (Negative empty mountpoint CLI check)**:
  - Command (`cwd`: `/mnt/Jad/github/lab/fs-relay`): `CGO_ENABLED=0 go build -o /tmp/fs-relay-bind ./cmd/fs-relay-bind && /tmp/fs-relay-bind -token local-demo ""`
  - Expected: Exit `2` with usage on stderr; never reaches `fs.Mount`.
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
  "changed_files": ["cmd/fs-relay-bind/cli.go", "cmd/fs-relay-bind/main.go", "cmd/fs-relay-bind/cli_test.go", "README.md"],
  "case_results": [{"id": "AC-001", "status": "pass"}, {"id": "AC-002", "status": "pass"}, {"id": "AC-003", "status": "pass"}],
  "commands_exits": [{"command": "CGO_ENABLED=0 go test -race ./...", "exit_code": 0}],
  "evidence": "goals/bind-mountpoint-arg/evidence.md",
  "candidate": "commit-sha",
  "blocker": null
}
```

## References
- **Shared Understanding & Fact Sheet**: [`goals/bind-mountpoint-arg/facts.md`](facts.md)
- **Execution Plan**: [`goals/bind-mountpoint-arg/plan.md`](plan.md)
- Upstream Issue: `https://github.com/muayyad-alsadi/fs-relay/issues/22`

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
