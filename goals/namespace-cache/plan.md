# Plan: Drop the namespace cache that nothing reads

## Execution Steps
- [ ] **Phase 1 — Contracts & Schemas**: Define models, data structures, and signatures.
- [ ] **Phase 2 — Core Implementation**: Implement business logic, routes, and error handling.
- [ ] **Phase 3 — Verification & Evidence**: Write unit tests, run end-to-end verification, and write markdown evidence with an explicit standalone `Verdict: PASS` declaration (or `## Verdict` followed by `PASS`) to satisfy `EVIDENCE_VERDICT_REQUIRED`.
- [ ] **Phase 4 — Attribution & Registry Closure**: (1) check all completed tasks in `goals/namespace-cache/plan.md`; (2) commit with `(goals/namespace-cache/goal.md)` or `[goal: namespace-cache]` using a `feat:`, `fix:`, `refactor:`, `perf:`, or `chore:` subject (docs/test commits are ignored); (3) certify via `sila goals certify namespace-cache --evidence=goals/namespace-cache/evidence.md` (with `Verdict: PASS`) or `sila goals audit namespace-cache`; (4) reconcile via `sila goals` and commit the updated `goals/README.md` and certification receipts; verify `sila goals --json` reports `unverified_count: 0` for delivered goals.
