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
- **Total Goals**: 16
- 🟡 **Ready to Execute (Pending)**: 2 (2 independent ⚡, 0 unblocked 🔗)
- ⛔ **Blocked on Prerequisites**: 2
- 🟢 **Implemented**: 12 (6 verified 📜, 6 stale ⚠️, 0 unverified)
- 🎯 **By Tier**: 0 immediate (Tier 1), 16 roadmap (Tier 2), 0 nice-to-have (Tier 3), 0 wont-fix (Tier 4)
- 📋 **Execution Tasks Progress**: 65/77 completed (84%)

---

## 🟡 Ready to Execute (Pending Goals)

| Goal Package | Tier | Mode & Sequence | Dependencies | Focus & Description | Launch Command |
| :--- | :---: | :---: | :--- | :--- | :--- |
| [`reject-extra-args`](reject-extra-args/goal.md) | `🗺️ Roadmap` | `🚢 SHIP ⚡ Independent (#13)` | - | Reject unexpected positional arguments in fs-relay — `fs-relay` parses flags with `flag.NewFlagSet("fs-relay", flag.ContinueOnError)`... | `/goal goals/reject-extra-args/goal.md` |
| [`bind-mountpoint-arg`](bind-mountpoint-arg/goal.md) | `🗺️ Roadmap` | `🚢 SHIP ⚡ Independent (#14)` | - | Cover the bind mountpoint argument with unit tests and a mount guard — Issue | `/goal goals/bind-mountpoint-arg/goal.md` |
| [`bind-sigint-busy`](bind-sigint-busy/goal.md) | `🗺️ Roadmap` | `🚢 SHIP ⛔ Blocked (#15)` | Prereqs: bind-mountpoint-arg | Exit cleanly or with a busy error on SIGINT in fs-relay-bind — Issue | `/goal goals/bind-sigint-busy/goal.md` |
| [`single-serve-data-lock`](single-serve-data-lock/goal.md) | `🗺️ Roadmap` | `🚢 SHIP ⛔ Blocked (#16)` | Prereqs: reject-extra-args | Guard the data directory with an exclusive lock unless forced — Issue | `/goal goals/single-serve-data-lock/goal.md` |

---

## 🟢 Implemented

| Goal Package | Mode | Shape | Verification | Task / Ref | Commit |
| :--- | :---: | :--- | :---: | :--- | :--- |
| [`advisory-record-locks`](advisory-record-locks/goal.md) | `⚡ Indep` | 🚢 SHIP | stale ⚠️ | Git Commit | `c84e4b2` |
| [`cli-review-fixes`](cli-review-fixes/goal.md) | `⚡ Indep` | 🚢 SHIP | stale ⚠️ | Git Commit | `a69607e` |
| [`startup-validation`](startup-validation/goal.md) | `⚡ Indep` | 🚢 SHIP | stale ⚠️ | Git Commit | `12bcc6e` |
| [`lock-robustness`](lock-robustness/goal.md) | `⚡ Indep` | 🚢 SHIP | stale ⚠️ | Git Commit | `7399591` |
| [`ci-race`](ci-race/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `8740b26` |
| [`namespace-cache`](namespace-cache/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `f5e6f13` |
| [`bind-grant-race`](bind-grant-race/goal.md) | `⚡ Indep` | 🚢 SHIP | stale ⚠️ | Git Commit | `30504b1` |
| [`bind-lock-coherence`](bind-lock-coherence/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `636df49` |
| [`html-prototypes-10x`](html-prototypes-10x/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `b6464f2` |
| [`sila-onboarding`](sila-onboarding/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `e991617` |
| [`sqlite-rollback-journal`](sqlite-rollback-journal/goal.md) | `🔗 Dep` | 🚢 SHIP | stale ⚠️ | Git Commit | `daa9689` |
| [`lock-wake-races`](lock-wake-races/goal.md) | `🔗 Dep` | 🚢 SHIP | verified 📜 | Git Commit | `636df49` |
