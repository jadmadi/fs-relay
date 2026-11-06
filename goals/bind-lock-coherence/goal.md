# Goal: Drain write-behind before unlock and drop cached reads on lock grant

## Chain of Custody (Isnad)
- **Registered By**: Pending Registration
- **Spec Audited By**: Pending Spec Audit
- **Implemented By**: Pending Implementation
- **Mystery Audited By**: @mystery-auditor | Report: `receipt_bind-lock-coherence_1793935639`
## Goal Description

Ensure lock handoffs across mounts in `cmd/fs-relay-bind` are coherent with
client-side write-behind buffering and read/attribute caching (upstream issue
`muayyad-alsadi/fs-relay#31`).

Today, `(*node).lock` in `cmd/fs-relay-bind/lock.go` forwards `setlk` and
`setlkw` directly to `fs-relay`:

1. When a holder writes fewer than `writeMin` bytes (4 KiB with `/raw`, 128 KiB
   without) and unlocks via `fcntl(F_SETLK, F_UNLCK)` or `flock(LOCK_UN)` on an
   open file, or when `close(2)` sends `FUSE_SETLK(F_UNLCK)` from
   `locks_remove_posix` before the asynchronous `FUSE_RELEASE`, the lock is
   released on the server while dirty bytes remain in `h.wb.buf`. Another mount
   that wakes and reads the file sees stale data.
2. When a mount is granted a read or write lock (`setlk` or `setlkw`),
   `(*node).lock` leaves `n.known` (the 1s attribute cache) and `n.k.body[ino]`
   (the 64 MiB prefetched small-file cache) intact. If the lock reply arrives
   before the background pubsub eviction event is processed by `k.loop`, a
   subsequent `getattr` or `read` on the acquiring mount serves stale cached
   state.

## Why now

Multi-agent and multi-mount workloads (SQLite rollback journal transactions,
shared JSON/state files, and `flock` task queues) unlock open descriptors or
rely on `close(2)` POSIX lock removal between short writes. Without draining
write-behind before unlock and invalidating local caches on lock grant, two
mounts that properly serialize with locks can still read stale bytes or
overwrite each other's changes.

## Dependencies & Execution Order

- **Mode**: Independent (disjoint, parallelizable)
- **Depends On**: none
- **Sequence**: 9
- **Shape**: ship
- **Tier**: roadmap
- **Memory Protocol**: required

## Files to Touch

- cmd/fs-relay-bind/lock.go
- cmd/fs-relay-bind/writeback.go
- cmd/fs-relay-bind/lock_release_test.go
- SPEC.md

## Decisions Fixed Before Dispatch

| Decision               | Value                                                     | Reason                                                                 |
| ---------------------- | --------------------------------------------------------- | ---------------------------------------------------------------------- |
| Where to drain writes  | Inside `(*node).lock` when `method != Getlk` and `LockUnlock` | Covers explicit `F_UNLCK`, `flock(LOCK_UN)`, and `close(2)` POSIX unlock without adding an RTT to unlocked `Flush` |
| Drain failure behavior | Still send the unlock to the server; return the drain errno | Never strand a held server lock when a buffered write fails            |
| Cache drop on grant    | Call `n.forgetAttr()` and `n.k.forgetIno(n.ino)` when `grant && err == nil` | Prevents lock grant replies that beat async pubsub evictions from serving stale `attr` or prefetched `body` |
| `Getlk` behavior       | Never drain write-behind and never drop caches on `Getlk` | `Getlk` is a read-only probe and does not transfer lock ownership      |

## Out of Scope

- Server-side waiter registration and lock downgrade wake-ups (covered in
  `goals/lock-wake-races/goal.md`, upstream issue `#32`).
- Changing `Flush` to send buffered writes for unlocked files (`SPEC.md` line
  214 intentionally keeps `Flush` local).
- New wire fields or new packages.

## References

- **Shared Understanding & Fact Sheet**: [`goals/bind-lock-coherence/facts.md`](facts.md)
- **Execution Plan**: [`goals/bind-lock-coherence/plan.md`](plan.md)
- **Upstream Issue**: `https://github.com/muayyad-alsadi/fs-relay/issues/31`
- **Spec Section**: `SPEC.md`, sections `Coherence` and `Locks`

## Memory Plan

- **Prime**: run `sila prime` (CLI) or `prime_context` (MCP) at task start and record the briefing timestamp.
- **Recall**: run `sila recall "<symptom or design question>"` before implementation; record each query and match count.
- **Writes**: record at least one `memo`, `lesson`, or `case` grounded to a file in this goal; use a `case` when the goal introduces a reusable mechanism.
- **Facts**: run `sila facts verify` before certification and record the exit code and violation delta.
- **Handoff**: run `sila handoff "<summary>" --open="..."` at session end and record the handoff ID.
- **Evidence**: embed a `sila/memory-writes/v1` block under `## Memory Writes`, per `memory-protocol-goal-contract`.

## Done Condition

1. All tasks in `plan.md` are checked (`- [x]`); unchecked tasks block the `implemented` transition.
2. Implementation is committed with `(goals/bind-lock-coherence/goal.md)` or `[goal: bind-lock-coherence]` attribution under a `feat:`, `fix:`, `refactor:`, `perf:`, or `chore:` prefix.
3. Goal is certified (`goals/bind-lock-coherence/certification.json` with `verified: true` or a signed `audit-receipt.json`) and any markdown evidence contains an explicit standalone `Verdict: PASS` declaration.
4. `sila goals --json` reports `status: "implemented"`, `verification_status: "verified"`, and `unverified_count: 0` for delivered goals, with a clean `git status`.
