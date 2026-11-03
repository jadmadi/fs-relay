# Goals Status & Pragmatic Execution Roadmap

This index tracks all goal packages under `goals/`, their execution mode (Independent ⚡ vs Dependent 🔗), dependencies, and progress status.

## 🤖 Agent Guide: How to Create, Claim & Execute Goals

1. **Create a Goal**: Scaffold a new goal package conforming to standards:
   ```bash
   sila goals create <slug> --title="..." [--depends-on="..."] [--independent]
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
- **Total Goals**: 5
- 🟡 **Ready to Execute (Pending)**: 1 (1 independent ⚡, 0 unblocked 🔗)
- 🔵 **In Progress (Claimed)**: 1
- 🟢 **Implemented**: 3 (3 verified 📜, 0 unverified)
- 🎯 **By Tier**: 0 immediate (Tier 1), 5 roadmap (Tier 2), 0 nice-to-have (Tier 3), 0 wont-fix (Tier 4)
- 📋 **Execution Tasks Progress**: 31/36 completed (86%)

---

## 🔵 In Progress (Claimed Goals)

| Goal Package | Tier | Mode | Agent | Status & Note | Started |
| :--- | :---: | :---: | :--- | :--- | :--- |
| [`cli-review-fixes`](cli-review-fixes/goal.md) | `🗺️ Roadmap` | `⚡ Indep` | `@opencode` | in progress | 3m0s ago |

---

## 🟡 Ready to Execute (Pending Goals)

| Goal Package | Tier | Mode & Sequence | Dependencies | Focus & Description | Launch Command |
| :--- | :---: | :---: | :--- | :--- | :--- |
| [`startup-validation`](startup-validation/goal.md) | `🗺️ Roadmap` | `🚢 SHIP ⚡ Independent (#2)` | - | Validate the mountpoint, data directory and namespace — Close the three assigned startup issues: | `/goal goals/startup-validation/goal.md` |

---

## 🟢 Implemented

| Goal Package | Mode | Shape | Verification | Task / Ref | Commit |
| :--- | :---: | :--- | :---: | :--- | :--- |
| [`advisory-record-locks`](advisory-record-locks/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `c84e4b2` |
| [`sila-onboarding`](sila-onboarding/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `e991617` |
| [`sqlite-rollback-journal`](sqlite-rollback-journal/goal.md) | `🔗 Dep` | 🚢 SHIP | verified 📜 | Git Commit | `daa9689` |
