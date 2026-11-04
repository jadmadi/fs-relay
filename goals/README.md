# Goals Status & Pragmatic Execution Roadmap

This index tracks all goal packages under `goals/`, their execution mode (Independent ⚡ vs Dependent 🔗), dependencies, and progress status.

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
- **Total Goals**: 9
- 🟡 **Ready to Execute (Pending)**: 2 (2 independent ⚡, 0 unblocked 🔗)
- 🟢 **Implemented**: 7 (2 verified 📜, 5 unbound ○, 0 unverified)
- 🎯 **By Tier**: 0 immediate (Tier 1), 9 roadmap (Tier 2), 0 nice-to-have (Tier 3), 0 wont-fix (Tier 4)
- 📋 **Execution Tasks Progress**: 45/53 completed (84%)

---

## 🟡 Ready to Execute (Pending Goals)

| Goal Package | Tier | Mode & Sequence | Dependencies | Focus & Description | Launch Command |
| :--- | :---: | :---: | :--- | :--- | :--- |
| [`namespace-cache`](namespace-cache/goal.md) | `🗺️ Roadmap` | `🚢 SHIP ⚡ Independent (#5)` | - | Drop the namespace cache that nothing reads — OpenNamespace writes t.dirs on every call and never reads it, so Call stats the ... | `/goal goals/namespace-cache/goal.md` |
| [`bind-grant-race`](bind-grant-race/goal.md) | `🗺️ Roadmap` | `🚢 SHIP ⚡ Independent (#6)` | - | Stop a granted lock outliving its handle — A read-only handle can grant a lock after Release took its owners, leaving a loc... | `/goal goals/bind-grant-race/goal.md` |

---

## 🟢 Implemented

| Goal Package | Mode | Shape | Verification | Task / Ref | Commit |
| :--- | :---: | :--- | :---: | :--- | :--- |
| [`advisory-record-locks`](advisory-record-locks/goal.md) | `⚡ Indep` | 🚢 SHIP | unbound ○ | Git Commit | `c84e4b2` |
| [`cli-review-fixes`](cli-review-fixes/goal.md) | `⚡ Indep` | 🚢 SHIP | unbound ○ | Git Commit | `a69607e` |
| [`startup-validation`](startup-validation/goal.md) | `⚡ Indep` | 🚢 SHIP | unbound ○ | Git Commit | `12bcc6e` |
| [`lock-robustness`](lock-robustness/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `7399591` |
| [`ci-race`](ci-race/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `8740b26` |
| [`sila-onboarding`](sila-onboarding/goal.md) | `⚡ Indep` | 🚢 SHIP | unbound ○ | Git Commit | `e991617` |
| [`sqlite-rollback-journal`](sqlite-rollback-journal/goal.md) | `🔗 Dep` | 🚢 SHIP | unbound ○ | Git Commit | `daa9689` |
