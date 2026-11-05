# Goal: SQLite rollback journal across two mounts

## Goal Description

Prove the lock table with the workload that needs it. Two mounts of one
namespace, two or more SQLite processes, rollback journal, no WAL. The
journal must serialize correctly across mounts, and both sides must report
`PRAGMA integrity_check` = `ok`.

This goal is the acceptance face of `advisory-record-locks`. It adds no lock
logic. It adds the documented, repeatable mount-level checks that prove the
table works where it matters, plus the negative cases that a unit test
cannot reach: interrupt, mount death, starvation, and `flock`.

## Why now

`ROADMAP.md` says the lock table exists so a rollback journal can work across
mounts. Today it works on one mount only. Until two mounts pass this check,
the implementation goal is not proven, no matter how many unit tests pass.

## Dependencies & Execution Order

- **Mode**: Dependent (blocked until the prerequisite is implemented)
- **Depends On**: `advisory-record-locks`
- **Sequence**: 2
- **Shape**: ship
- **Tier**: roadmap
- **Memory Protocol**: required

Start only when `sila goals --json` shows `advisory-record-locks` as
`implemented` and its mount methods are in `main`. A harness against an
incomplete table produces false failures.

## Success looks like

1. On a fresh namespace, mount the same share twice, under two mountpoints.
2. Open or create one database through mount A.
3. Four writer connections, split across A and B, commit 200 rows with
   `busy_timeout`, and the values are four distinct markers.
4. `PRAGMA integrity_check` returns `ok` from a connection on each mount.
5. A reader on A blocks a writer on B and the writer waits, then proceeds
   when the reader exits. No immediate `database is locked`.
6. `SIGINT` to a process parked in `F_SETLKW` returns `EINTR`, the process
   exits cleanly, and the next lock on that byte succeeds.
7. Killing mount A while it holds a write lock releases the lock after the
   8-second hold, and mount B then proceeds.
8. `flock` and `fcntl` on a single mount still behave as they did before the
   lock table.
9. Every run is recorded with the exact versions, flags and exit codes.

## What counts as proof

- The database file lives under the namespace on the server, not on either
  client's local disk.
- Each run starts from a fresh namespace directory and a fresh mountpoint
  pair. Reuse invalidates the result.
- Each command's exit code is recorded. A zero exit is not enough; the SQL
  assertions decide.
- The evidence file names the commit, the sila goal, SQLite version, kernel
  version and mount flags.

## Out of scope

- WAL. It needs a shared `-shm` mapping and stays refused.
- Network filesystems, SMB, NFS.
- Performance numbers that belong in the README tables. This goal records
  correctness and elapsed time, not a published benchmark.
- More than two mounts. Three are noted as a follow-up only.
- Windows, macOS.
- Testing through a process that opens `data/` directly.

## References

- [`goals/sqlite-rollback-journal/facts.md`](facts.md)
- [`goals/sqlite-rollback-journal/plan.md`](plan.md)
- [`goals/advisory-record-locks/goal.md`](../advisory-record-locks/goal.md)
- `ROADMAP.md`, section Advisory record locks, including the four-machine
  and two-open scenarios
- `README.md` smoke checklist

## Memory Plan

- **Prime**: run `sila prime` at task start.
- **Recall**: run `sila recall "sqlite database is locked"` and record the
  matches before concluding a failure is a bug.
- **Writes**: record a `lesson` for every unexpected SQLite failure, with
  cause and fix. Record a `case` for the two-mount harness shape.
- **Facts**: `sila facts verify` before certification.
- **Handoff**: at session end, with the open items.
- **Evidence**: embed a `sila/memory-writes/v1` block under
  `## Memory Writes`.

## Done Condition

1. All tasks in `plan.md` are checked.
2. Results are committed with `(goals/sqlite-rollback-journal/goal.md)` under
   `feat:`, `fix:`, `refactor:`, `perf:` or `chore:`.
3. `goals/sqlite-rollback-journal/evidence.md` contains every run, the
   assertion output, and a standalone `Verdict: PASS` line.
4. `ROADMAP.md` records the run date, the commit, SQLite version and the
   result, in the style of the existing M3 entry.
5. `sila goals --json` reports `status: "implemented"`,
   `verification_status: "verified"`, `unverified_count: 0`, and the tree is
   clean.
