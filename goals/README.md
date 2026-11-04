# Goals Status & Pragmatic Execution Roadmap

This index tracks all goal packages under `goals/`, their execution mode (Independent ⚡ vs Dependent 🔗), dependencies, and progress status.

## Reading these records

A package records the work as it stood when it was certified. `SPEC.md` is the
current contract, and a later change that replaced a decision is marked in the
record. Two findings from the review of these records are open code defects,
not record errors: #27 (a grant that can outlive its released handle) and #28
(a namespace cache that is written but never read).

## 🤖 Agent Guide: How to Create, Claim & Execute Goals

1. **Create a Goal**: Scaffold a new goal package conforming to standards:
   ```bash
   sila goals create <slug> --title="..." --files="path/to/file.go" [--depends-on="..."] [--independent]
   ```
2. **Claim a Goal**: Lock the goal so no other agent duplicates work:
   ```bash
   sila goals claim <slug> --agent=<name> [--note="evaluating..."]
   ```
3. **Launch & Implement**: Follow `facts.md` and `plan.md` until all tests pass:
   ```bash
   /goal goals/<slug>/goal.md
   ```
4. **Progress Updates**: Report status updates during execution:
   ```bash
   sila goals report <slug> "Running e2e test suite"
   ```
5. **Commit with Goal Reference**: Include the goal path in your commit message:
   ```bash
   git commit -m "feat(scope): implement description (goals/<slug>/goal.md)"
   ```
6. **Auto-Reconciliation**: Run `sila goals` to scan commits, mark 🟢 **Implemented**, and clear the claim lock.

## Summary
- **Total Goals**: 7
- 🟡 **Ready to Execute (Pending)**: 0 (0 independent ⚡, 0 unblocked 🔗)
- 🟢 **Implemented**: 7 (2 verified 📜, 5 unbound ○, 0 unverified)
- 🎯 **By Tier**: 0 immediate (Tier 1), 7 roadmap (Tier 2), 0 nice-to-have (Tier 3), 0 wont-fix (Tier 4)
- 📋 **Execution Tasks Progress**: 45/45 completed (100%)

---

## 🟢 Implemented

| Goal Package | Mode | Depends On | Shape | Verification | Task / Ref | Commit |
| :--- | :---: | :--- | :---: | :---: | :--- | :--- |
| [`advisory-record-locks`](advisory-record-locks/goal.md) | `⚡ Indep` | none | 🚢 SHIP | unbound ○ | Git Commit | `c84e4b2` |
| [`cli-review-fixes`](cli-review-fixes/goal.md) | `⚡ Indep` | none | 🚢 SHIP | unbound ○ | Git Commit | `a69607e` |
| [`startup-validation`](startup-validation/goal.md) | `⚡ Indep` | none | 🚢 SHIP | unbound ○ | Git Commit | `12bcc6e` |
| [`lock-robustness`](lock-robustness/goal.md) | `⚡ Indep` | none | 🚢 SHIP | verified 📜 | Git Commit | `7399591` |
| [`ci-race`](ci-race/goal.md) | `⚡ Indep` | none | 🚢 SHIP | verified 📜 | Git Commit | `6e4d9b2` |
| [`sila-onboarding`](sila-onboarding/goal.md) | `⚡ Indep` | none | 🚢 SHIP | unbound ○ | Git Commit | `e991617` |
| [`sqlite-rollback-journal`](sqlite-rollback-journal/goal.md) | `🔗 Dep` | `advisory-record-locks` | 🚢 SHIP | unbound ○ | Git Commit | `daa9689` |
