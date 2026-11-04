# Goal: Advisory record locks across mounts

## Goal Description

Make `fs-relay` the authority for POSIX advisory byte-range locks, so two
mounts of one namespace exclude each other. Today the kernel keeps a private
lock table per mount, and two mounts can both hold a write lock on the same
bytes.

The goal covers the whole chain: the FlatBuffers fields, the method names,
the per-inode lock record in `internal/local`, the server-side wait and
release broadcast, the go-fuse node methods, the `flock` case, the errno
mapping, tests at every layer, and the `SPEC.md` and `ROADMAP.md` updates.

It does not cover proving SQLite. That is the dependent goal
`sqlite-rollback-journal`, which consumes this one.

## Why now

`ROADMAP.md` names this as the only remaining call in the one-token mount
surface. `SPEC.md` already specifies the behaviour under Locks. The
`release` event, the `pending` notice and the release replay are already in
the wire and the hub, but nothing produces them. Without this goal the spec
describes a table that does not exist.

## Dependencies & Execution Order

- **Mode**: Independent (disjoint, parallelizable)
- **Depends On**: none
- **Sequence**: 1
- **Shape**: ship
- **Tier**: roadmap
- **Memory Protocol**: required

## Success looks like

1. Two mounts of one namespace cannot both hold a conflicting byte range while
   the server recognizes the lock. After the 8-second hold expires, a mount
   that lost its stream can still write through an open handle, so exclusion
   is not unconditional.
2. One mount behaves exactly as it does today: SQLite rollback journal still
   works, single-mount `flock` still works, no new failure on any lock call.
3. `getlk`, `setlk` and `setlkw` round-trip over both transports, POST and
   websocket.
4. A waiting lock is released by a release on another mount, and wakes even
   if the release lands in the gap between the failed grant and the park.
5. A dead mount releases its locks after the existing 8-second hold window,
   not immediately, so a rejoin keeps them.
6. The commit path of an uncontended lock allocates nothing in the lock
   table, and writes no release event when nobody waits.
7. `gofmt -l .` is empty, `go vet ./...` is clean, `go test ./...` and
   `go test -race ./...` pass.

## Decisions fixed before dispatch

These are settled for the implementer. Change one only with a recorded memo
and a `SPEC.md` edit in the same change.

| Decision | Value | Reason |
|----------|-------|--------|
| Wait model | ~~Server holds the call~~ Superseded by T12: `EAGAIN` and a client retry | Historical: one state machine was the plan. T12 replaced it with `EAGAIN`, a client retry and a replayed release. |
| POST wait | ~~The response is held until grant~~ Superseded by T12: returns `EAGAIN` at once | One POST is one exchange, so there is no second reply |
| Websocket wait | ~~Send `pending` with a ticket~~ Superseded by T12: returns `EAGAIN` at once | `wire.MarshalPending` and `Reply.Pending` already define this |
| Owner key | `(client id, kernel owner id)` | The scrambled 64-bit owner is not unique across machines |
| Close semantics | A `release` names a handle or an owner. A read-only handle records the owners that locked through it; its close drops only those | Never drop by client: a `cat` must not take a worker's locks |
| Partial unlock | Split, keep the remainder on the overflow list, decrement the canonical count | Counts stay honest; no silent coverage loss |
| Overflow storage | Small slice on the record, allocated on first use | Only arbitrary-range clients pay; the Django path never enters it |
| `flock` | Handle in the same table as a whole-file entry | `EnableLocks` also advertises `CAP_FLOCK_LOCKS`, so refusing would regress single mounts |
| Deadlock detection | None | POSIX does not require `EDEADLK`; the kernel only detects inside one mount |
| Client death | Release after the existing 8s hold expires | A rejoin inside the window keeps its locks |
| Two-inode calls | Take record locks in inode order | Avoids a deadlock between lock and rename |
| Lock table mutex | One, never held across I/O, a channel send, or a wake-up | `AGENTS.md` concurrency rule |

## Out of scope

- WAL. It needs a shared `-shm` mapping and stays refused.
- Mandatory locks, leases, `F_OFD_*` locks initiated by the server.
- Deadlock detection, lock upgrade fairness, priority inheritance.
- Durable locks across a server restart. A restart is a remount.
- `fcntl` on a process that opens `data/` directly. It is outside the table.
- Chroot, TLS, accounts, Windows, macOS.

## References

- [`goals/advisory-record-locks/facts.md`](facts.md)
- [`goals/advisory-record-locks/plan.md`](plan.md)
- `SPEC.md`, sections Locks, Envelope, Coherence, Packages, Testing
- `ROADMAP.md`, section Advisory record locks
- [`goals/sqlite-rollback-journal/goal.md`](../sqlite-rollback-journal/goal.md)

## Memory Plan

- **Prime**: run `sila prime` at task start and record the briefing timestamp.
- **Recall**: run `sila recall "<symptom>"` before implementation and record
  each query and match count.
- **Writes**: at least one `memo`, `lesson` or `case` grounded to a file in
  this goal. Use a `case` for the record layout because it is a reusable
  mechanism.
- **Facts**: run `sila facts verify` before certification and record the exit
  code and violation delta.
- **Handoff**: run `sila handoff "<summary>" --open="..."` at session end and
  record the handoff ID.
- **Evidence**: embed a `sila/memory-writes/v1` block under
  `## Memory Writes`, per `memory-protocol-goal-contract`.

## Done Condition

1. All tasks in `plan.md` are checked (`- [x]`); unchecked tasks block the
   `implemented` transition.
2. Implementation is committed with `(goals/advisory-record-locks/goal.md)`
   or `[goal: advisory-record-locks]` under `feat:`, `fix:`, `refactor:`,
   `perf:` or `chore:`.
3. Goal is certified (`certification.json` with `verified: true`, or a
   signed `audit-receipt.json`) and evidence contains a standalone
   `Verdict: PASS` line.
4. `sila goals --json` reports `status: "implemented"`,
   `verification_status: "verified"`, `unverified_count: 0`, and
   `git status` is clean.
5. `go test -race ./...` passes, and the README smoke checklist gains the
   two-mount case that goal 2 will run.
