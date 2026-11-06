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
- **Total Goals**: 12
- 🟡 **Ready to Execute (Pending)**: 1 (1 independent ⚡, 0 unblocked 🔗)
- ⛔ **Blocked on Prerequisites**: 1
- 🟢 **Implemented**: 10 (10 verified 📜, 0 unverified)
- 🎯 **By Tier**: 0 immediate (Tier 1), 12 roadmap (Tier 2), 0 nice-to-have (Tier 3), 0 wont-fix (Tier 4)
- 📋 **Execution Tasks Progress**: 58/65 completed (89%)

---

## 🟡 Ready to Execute (Pending Goals)

| Goal Package | Tier | Mode & Sequence | Dependencies | Focus & Description | Launch Command |
| :--- | :---: | :---: | :--- | :--- | :--- |
| [`bind-lock-coherence`](bind-lock-coherence/goal.md) | `🗺️ Roadmap` | `🚢 SHIP ⚡ Independent (#9)` | - | Drain write-behind before unlock and drop cached reads on lock grant — Ensure lock handoffs across mounts in `cmd/fs-relay-bind` are coherent with clie... | `/goal goals/bind-lock-coherence/goal.md` |
| [`lock-wake-races`](lock-wake-races/goal.md) | `🗺️ Roadmap` | `🚢 SHIP ⛔ Blocked (#10)` | Prereqs: bind-lock-coherence | Close lock wake-up races across EAGAIN registration, downgrades, and multi-waiters — Eliminate the three server-side lock wake-up races in `internal/local` and `cmd/... | `/goal goals/lock-wake-races/goal.md` |

---

## 🟢 Implemented

| Goal Package | Mode | Shape | Verification | Task / Ref | Commit |
| :--- | :---: | :--- | :---: | :--- | :--- |
| [`advisory-record-locks`](advisory-record-locks/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `c84e4b2` |
| [`cli-review-fixes`](cli-review-fixes/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `a69607e` |
| [`startup-validation`](startup-validation/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `12bcc6e` |
| [`lock-robustness`](lock-robustness/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `7399591` |
| [`ci-race`](ci-race/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `8740b26` |
| [`namespace-cache`](namespace-cache/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `f5e6f13` |
| [`bind-grant-race`](bind-grant-race/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `30504b1` |
| [`html-prototypes-10x`](html-prototypes-10x/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `b6464f2` |
| [`sila-onboarding`](sila-onboarding/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `e991617` |
| [`sqlite-rollback-journal`](sqlite-rollback-journal/goal.md) | `🔗 Dep` | 🚢 SHIP | verified 📜 | Git Commit | `daa9689` |
