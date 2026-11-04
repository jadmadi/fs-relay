# AGENTS.md

Working notes for agents in this repo. `SPEC.md` holds the design; this file holds how the code is written.

`fs-relay` is an HTTP relay of FUSE calls to a local directory, in Go. The wire is FlatBuffers. Five packages, on purpose; do not add a sixth: `cmd/fs-relay`, `cmd/fs-relay-bind`, `internal/wire` (codec), `internal/local` (every `os` call, inode map, handles, locks, error sentinels), `internal/client` (the POST and the websocket).

## Who the words are for

The person reading the welcome page, the README, a flag, or an error is a sysadmin. They are the driver. A driver does not need to be a mechanical engineer to drive the car. They need to know what the car offers, how to start it, and how it compares with the other cars. They do not need the workshop notes.

Write the README, the welcome page, flags, and errors that way. Say what fs-relay is, what a mount can do, how to run it, and how the timings compare with the other filesystems. Leave out encodings, package names, which function answered the call, the clock time of a benchmark run, and a tool's exit status. Those facts do not help someone decide to mount it or operate it.

The README section on earlier designs is the exception. It speaks to developers, and its job is to stop a retry: say what was already measured and that it stays out. The run that produced a published number is recorded in `ROADMAP.md`, not in the README.

## Commands

```bash
gofmt -l .   # must print nothing
go vet ./...
go test ./...
go build -o fs-relay ./cmd/fs-relay
go build -o fs-relay-bind ./cmd/fs-relay-bind
```

## Errors

- Return errors; never log an error and return it too.
- Wrap with context: `fmt.Errorf("mkdir %s: %w", name, err)`.
- Match with `errors.Is`/`errors.As`, never by comparing strings.
- Errno travels as a number. `internal/local` returns `syscall.Errno` or wraps it; one table in `cmd/fs-relay` turns an error into `error.code`, and one in `cmd/fs-relay-bind` turns the code back into an errno.
- No `panic` outside a startup invariant.

## Functions

- Guard clauses and early returns; no `else` after a `return`.
- One level of abstraction per function: it decides, or it does I/O, not both.
- Keep functions under roughly 40 lines and nesting under four levels.
- If a block needs a comment explaining what it does, extract it and use the comment as the name.
- A boolean parameter that flips behavior is two functions wearing one name.
- `switch` over a closed set (the method names), not an if-chain; dispatch through one table of method to handler.

## Types and seams

- Pass the concrete type until a second implementation exists; then declare the smallest interface at the consumer.
- No interface with one implementation and one caller. No `XxxManager`/`XxxService` that forwards calls.
- Dependencies arrive through the constructor. No package-level mutable state, no `init()`.
- `internal/wire` imports neither `net/http`, go-fuse nor `os`.
- Only `internal/local` touches the filesystem, and it resolves every path under `data/<namespace>/` in one function that refuses escape.

## Spaghetti and ravioli

- One entry point, one exit per branch; iterate once and collect, then act.
- Prefer a data table (method handlers, errno mappings) to a chain of special cases.
- Every indirection earns its keep: a second caller, a seam a test cannot get otherwise, or a real boundary (HTTP, FUSE, syscalls).
- If a wrapper's body is a single call, delete the wrapper.

## Concurrency

- `context.Context` is the first parameter and is never stored in a struct.
- Every goroutine has an owner and a shutdown path; `Close` waits for it.
- The inode map and handle table are guarded by one owner each; never hold their lock across file I/O or a network call.
- A lock waiter holds no mutex while it waits on its connection.

## Tests

- Table-driven; `t.Helper()` in helpers, `t.TempDir()` for paths, no `time.Sleep`.
- Assert through the public surface of `internal/local` against a real temp directory. Use `net/http/httptest` for the HTTP edge.
- Anything that needs a real mount is not a unit test; it belongs in the README smoke checklist.

## Dependencies

Standard library first. Expected: `go-fuse`, `github.com/google/flatbuffers`, and `golang.org/x/sys/unix` for the syscalls the standard library does not wrap. UUIDv7 is a small local function. Ask before adding another.

`internal/wire/fsrelay_generated.go` is generated. Regenerate it with `flatc --go --gen-onefile --go-namespace wire -o internal/wire internal/wire/fsrelay.fbs`. Do not hand-edit it.

## Done means

- `gofmt -l .` is empty; `go vet ./...` and `go test ./...` are clean.
- `go test -race ./...` is clean. CI runs it in its own job.
- New exported identifiers have doc comments; a new operation has a `local` test and a golden case in `wire`.
- The run that produced a published benchmark number is recorded in `ROADMAP.md`. The README states the comparison.
- A behavior change updates `SPEC.md` and `ROADMAP.md` in the same change.

<!-- sila:begin (managed by sila; content inside is regenerated by `sila install` / `sila agents sync`) -->
<!-- sila:meta rev=1 scope=core hash=a57df412ba43bc74 -->
## 0. Safety Rails (NEVER Do This)
- No unprompted remote push: Local commits are autonomous; remote push requires user consent.
- No unauthorized repo creation or re-homing: NEVER run 'gh repo create' or push external specs to personal namespaces; fail closed on auth hurdles.

## 1. Working Relationship & Discipline
- Concise and direct.
- Terminal Markdown: Keep visible line and table width under 80 chars; no hard breaks inside bullets or links.
- Decision Brief & `/adoo` Receipts: Lead with 1-line verdict ("X happens because Y, confirmed by Z"), compact receipt/risk table, file links, and next action. No 1-5 scores.
- Verified Repo Runbook Primacy: Never guess commands; verify exit code 0 locally.
- Closed-Loop Goal Attribution (6 Steps) for `goals/<slug>/`:
  1. Check every completed task `- [x]` in `goals/<slug>/plan.md`.
  2. Write markdown evidence with a standalone `Verdict: PASS` line.
  3. Commit with `(goals/<slug>/goal.md)` or `[goal: <slug>]` under `feat:`, `fix:`, `refactor:`, `perf:`, or `chore:` (`docs:`/`test:` ignored).
  4. Certify via `sila goals certify <slug> --evidence=goals/<slug>/evidence.md` or `sila goals audit <slug>`.
  5. Run `sila goals` to reconcile `goals/README.md` and commit receipts.
  6. Verify `sila goals --json` reports `status: "implemented"`, `verification_status: "verified"`, `unverified_count: 0`, and clean `git status`.

## Sila Trigger Contract

Memory writes are required by the goal contract and enforced by `sila gate`.
- `sila prime` (start) → briefing + goals.
- `sila recall "<symptom>"` (stuck) → prior resolutions.
- `sila memo "<directive>" --files=...` (decision) → grounded directive.
- `sila lesson "<symptom>" --cause=... --fix=...` (solved) → grounded lesson.
- `sila case "<intent>" --approach=... --invariant=...` (pattern) → case record.
- `sila facts verify` (grounding) → file/ledger evidence.
- `sila gate` (before commit) → deterministic quality receipt.
- `sila handoff "<summary>" --open=...` (end) → handoff record.
- `sila sync` (after changes) → cross-harness continuity.
- `sila salam`, `sila guide`, `docs/reference.md` → detail.
- Prior research/specs (BEFORE grep/find): `sila search "<q>" --omni` (`omni_search`), `sila search expand <id>` (`get_message_context`), `sila doc search "<q>"` (`search_docs`).
- Activity/domains/review/skills: `sila standup last 24h`, `sila digest --since=24h`, `sila recap`; `sila guide` (`get_operational_guidebook`), `sila learn`, `sila persona`, `sila hire --for-goal=<slug>`, `sila audit` (`audit_goal`), `sila skill` (`skill_search`, `skill_load`).
- Symbols/decisions: `sila edit-check --symbol=<sym>`, `sila safe-delete --symbol=<sym>`, `sila note --symbol=<sym>`, `sila decision` (`create_decision`), `sila vision triage`.

---

## Stack-Specific Standards (Go)

- 1. Code Review & Hygiene: maintain pure Go standard library with zero third-party dependencies, Pure-Go CGO-Free Portability with CGO_ENABLED=0, and hard file ceilings under 1,000 LOC.
- 2. Goroutine & Context Safety: require context.Context on all background routines; declare defer cancel() immediately after timeout context creation; ensure goroutine leak prevention.
- 3. Goal Scaffolding: break findings into discrete single-outcome packages under goals/<slug>/ with goal.md, facts.md, plan.md, and explicit dependencies.
- 4. Test-Driven Implementation: claim goal locks before editing; write table-driven tests verifying both acceptance assertions and negative rejection paths.
- 5. Quality Gates & Race Detector Discipline: verify changes locally with linter scripts and full race detection ('go test -race ./...') with zero tolerance for data races.
- 6. Attributed Commits: use semantic prefixes containing the goal slug: feat(scope): desc [goal: <slug>]; run sila goals audit to certify receipts.
- 7. Git Branch Discipline: develop on dedicated feature branches; never push unprompted to remotes; re-verify race tests on merged main.
- 8. Security & Trust Boundaries: validate all CLI flags and stdin readers; enforce UID ownership; use mode 0600 on sensitive temporary storage.
- 9. Release Lifecycle & Value Changelog: implement a 24-hour cached daily version check, offer direct upgrades, provide a human-readable benefits-focused changelog, and credit contributors.

---
<!-- sila:end -->
