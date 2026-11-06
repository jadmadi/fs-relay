# Goal: Elevate html-prototypes 10x and align server landing page

## Chain of Custody (Isnad)
- **Registered By**: Pending Registration
- **Spec Audited By**: Pending Spec Audit
- **Implemented By**: Pending Implementation
- **Mystery Audited By**: @mystery-auditor | Report: `receipt_html-prototypes-10x_1793924570`
## Goal Description
Upgrade the eight `html-prototypes/` pages and animated explainer with token-strict styling, GPU-accelerated transitions, pointer-scrubbable playback, non-clipping responsive SVG charts, and progressive-enhancement interactive explorers (`html-prototypes/ui.js`), and synchronize the embedded server welcome page in `cmd/fs-relay/root.go` with the ink-slate and signal-rose design system in `.ui-craft/tokens.md`.

## Dependencies & Execution Order
- **Mode**: Independent (disjoint, parallelizable)
- **Depends On**: none
- **Sequence**: 10
- **Shape**: ship
- **Tier**: roadmap
- **Memory Protocol**: required

## Files to Touch
- .ui-craft/brief.md
- .ui-craft/tokens.md
- html-prototypes/CNAME
- html-prototypes/style.css
- html-prototypes/explainer.css
- html-prototypes/explainer.js
- html-prototypes/ui.js
- html-prototypes/index.html
- html-prototypes/run.html
- html-prototypes/compare.html
- html-prototypes/wire.html
- html-prototypes/coherence.html
- html-prototypes/locks.html
- html-prototypes/raw.html
- html-prototypes/explainer.html
- interactive-prototype/index.html
- interactive-prototype/style.css
- interactive-prototype/app.js
- interactive-prototype/vendor/three.module.min.js
- interactive-prototype/vendor/controls/OrbitControls.js
- cmd/fs-relay/root.go
- cmd/fs-relay/root_test.go

## References
- **Shared Understanding & Fact Sheet**: [`goals/html-prototypes-10x/facts.md`](facts.md)
- **Execution Plan**: [`goals/html-prototypes-10x/plan.md`](plan.md)

## Memory Plan
- **Prime**: run `sila prime` (CLI) or `prime_context` (MCP) at task start and record the briefing timestamp.
- **Recall**: run `sila recall "<symptom or design question>"` before implementation; record each query and match count.
- **Writes**: record at least one `memo`, `lesson`, or `case` grounded to a file in this goal; use a `case` when the goal introduces a reusable mechanism.
- **Facts**: run `sila facts verify` before certification and record the exit code and violation delta.
- **Handoff**: run `sila handoff "<summary>" --open="..."` at session end and record the handoff ID.
- **Evidence**: embed a `sila/memory-writes/v1` block under `## Memory Writes`, per `memory-protocol-goal-contract`.

## Done Condition
1. All tasks in `plan.md` are checked (`- [x]`); unchecked tasks block the `implemented` transition.
2. Implementation is committed with `(goals/html-prototypes-10x/goal.md)` or `[goal: html-prototypes-10x]` attribution under a `feat:`, `fix:`, `refactor:`, `perf:`, or `chore:` prefix.
3. Goal is certified (`goals/html-prototypes-10x/certification.json` with `verified: true` or a signed `audit-receipt.json`) and any markdown evidence contains an explicit standalone `Verdict: PASS` declaration.
4. `sila goals --json` reports `status: "implemented"`, `verification_status: "verified"`, and `unverified_count: 0` for delivered goals, with a clean `git status`.
