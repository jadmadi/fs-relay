# Goal: Close lock wake-up races across EAGAIN registration, downgrades, and multi-waiters

## Chain of Custody (Isnad)
- **Registered By**: Pending Registration
- **Spec Audited By**: Pending Spec Audit
- **Implemented By**: Pending Implementation
- **Mystery Audited By**: @mystery-auditor | Report: `receipt_lock-wake-races_1793935644`
## Goal Description

Eliminate the three server-side lock wake-up races in `internal/local` and
`cmd/fs-relay` that can leave contending mounts asleep until the one-second
fallback poll in `Setlkw` (upstream issue `muayyad-alsadi/fs-relay#32`):

1. Gap between `EAGAIN` and `markWaiting`: today `tree.Call` releases `t.mu`
   and returns `syscall.EAGAIN` before `serve` or `readCalls` records the
   waiting client in `hub.markWaiting`. An unlock or `release` that lands in
   that window consumes `e.locks.wake` via `tree.TookWake` while the waiter set
   is still empty and sends no release event.
2. Multi-waiter wipe on first unlock: `hub.clearWaiting` deletes the entire
   `h.waiting[ns][ino]` map when the first release happens. When three or more
   mounts contend on one inode and the first woken mount acquires and unlocks
   before a slower waiter's retry re-registers, the second unlock sees an empty
   waiter set and sends no release event.
3. Silent lock downgrade: in `internal/local/lock.go`, downgrading a held
   `LockWrite` to `LockRead` in `apply`, `put`, or `flockSet` does not set
   `r.wake = true`, leaving readers parked in `Setlkw` until the 1s poll.

## Why now

When a swarm of agents or worker processes contends on a shared SQLite database
or lockfile, multiple mounts queue in `Setlkw` simultaneously and SQLite
connections downgrade write locks (`PENDING` or `SHARED`) or release locks in
rapid succession. Any missed release broadcast adds a 1,000 ms stall per lock
handoff.

## Dependencies & Execution Order

- **Mode**: Dependent (sequential)
- **Depends On**: `bind-lock-coherence`
- **Sequence**: 10
- **Shape**: ship
- **Tier**: roadmap
- **Memory Protocol**: required

## Files to Touch

- internal/local/lock.go
- internal/local/call.go
- internal/local/lock_test.go
- cmd/fs-relay/hub.go
- cmd/fs-relay/lockwait.go
- cmd/fs-relay/serve.go
- cmd/fs-relay/ws.go
- cmd/fs-relay/lock_test.go
- SPEC.md

## Decisions Fixed Before Dispatch

| Decision | Value | Reason |
| -------- | ----- | ------ |
| Waiter authority | Move waiter tracking into `lockRec.waiters []lockWaiter` in `internal/local/lock.go` under `t.mu` | Closes the gap between `EAGAIN` and waiter registration without a second mutex |
| `setlk` vs `setlkw` dispatch | Split `operations[wire.Setlk]` (`setlk`) and `operations[wire.Setlkw]` (`setlkw`) via `setlkMode(ctx, t, p, wait bool)` | Only `Setlkw` conflicts register a waiter; non-blocking `Setlk` probes never register one |
| Waiter lifecycle | Add `{key, fh}` on `Setlkw` `EAGAIN`; remove `key` on lock grant, explicit unlock, `DropOwner`, `dropFH`, or `dropClient` | Keeps remaining waiters registered across back-to-back unlocks when 3+ mounts contend |
| Downgrade wake-up | Set `r.wake = true` whenever an owner replaces its own `LockWrite` with `LockRead` in `apply`, `put`, or `flockSet` | Wakes blocked `Setlkw` readers immediately when a writer downgrades to shared |
| Uncontended release cost | `TookWake` returns `true` only when `r.wake` was set and `len(r.waiters) > 0` | Preserves the zero-broadcast invariant when nobody is waiting |

## Out of Scope

- Changing the wire protocol or FlatBuffers schema (`internal/wire`).
- Changing the client-side `Setlkw` retry loop in `cmd/fs-relay-bind/lock.go`.
- Fair queuing or FIFO lock reservation across mounts.

## References

- **Shared Understanding & Fact Sheet**: [`goals/lock-wake-races/facts.md`](facts.md)
- **Execution Plan**: [`goals/lock-wake-races/plan.md`](plan.md)
- **Upstream Issue**: `https://github.com/muayyad-alsadi/fs-relay/issues/32`
- **Prerequisite Goal**: [`goals/bind-lock-coherence/goal.md`](../bind-lock-coherence/goal.md)

## Memory Plan

- **Prime**: run `sila prime` (CLI) or `prime_context` (MCP) at task start and record the briefing timestamp.
- **Recall**: run `sila recall "<symptom or design question>"` before implementation; record each query and match count.
- **Writes**: record at least one `memo`, `lesson`, or `case` grounded to a file in this goal; use a `case` when the goal introduces a reusable mechanism.
- **Facts**: run `sila facts verify` before certification and record the exit code and violation delta.
- **Handoff**: run `sila handoff "<summary>" --open="..."` at session end and record the handoff ID.
- **Evidence**: embed a `sila/memory-writes/v1` block under `## Memory Writes`, per `memory-protocol-goal-contract`.

## Done Condition

1. All tasks in `plan.md` are checked (`- [x]`); unchecked tasks block the `implemented` transition.
2. Implementation is committed with `(goals/lock-wake-races/goal.md)` or `[goal: lock-wake-races]` attribution under a `feat:`, `fix:`, `refactor:`, `perf:`, or `chore:` prefix.
3. Goal is certified (`goals/lock-wake-races/certification.json` with `verified: true` or a signed `audit-receipt.json`) and any markdown evidence contains an explicit standalone `Verdict: PASS` declaration.
4. `sila goals --json` reports `status: "implemented"`, `verification_status: "verified"`, and `unverified_count: 0` for delivered goals, with a clean `git status`.
