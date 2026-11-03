# Plan: SQLite rollback journal across two mounts

This goal adds no lock logic. If a test fails because of grant, release or
wait behaviour, stop and return the failure to `advisory-record-locks` with
the evidence. Do not patch the table here.

Read `facts.md` for versions, SQL and failure signatures. Start every run
from a fresh `data/` and fresh mountpoints.

## Execution Steps

- [ ] **T1 - Preflight the prerequisite**
  - Goal: fail fast if the lock table is not actually shipped.
  - Files: read `cmd/fs-relay-bind/main.go`, `cmd/fs-relay-bind/lock.go`,
    `internal/local/lock.go`.
  - Requirements:
    1. `EnableLocks` is true.
    2. The three node methods exist and are wired.
    3. The three methods are in the `operations` table.
    4. `go report` is not needed; read the code.
  - Example: `grep -n "EnableLocks" cmd/fs-relay-bind/main.go` matches.
  - Acceptance:
    - AC-2.1: all four checks pass. If any fails, stop with status
      `blocked_on_prerequisite`.
  - Wrong-way: starting the mount anyway. `EOPNOTSUPP` on every lock call
    will look like a table bug.

- [ ] **T2 - Single-mount baseline**
  - Goal: prove the change did not break the case that works today.
  - Files: none. Run by hand, record in `evidence.md`.
  - Requirements:
    1. One mount, one connection, rollback journal, insert, commit.
    2. Four writers with `busy_timeout=5000`, 200 rows, four markers.
    3. `integrity_check` = `ok`.
  - Acceptance:
    - AC-2.2: 200 rows, 4 distinct `who`, `integrity_check` = `ok`, every
      exit code 0.
    - AC-2.3: `PRAGMA journal_mode` returns `delete`, never `wal`.

- [ ] **T3 - Two mounts, same namespace, distinct clients**
  - Goal: both mounts see one file, and the server sees two clients.
  - Files: none.
  - Requirements:
    1. Start the server on a fresh `data/`, one namespace, one token.
    2. Mount A and mount B to the same namespace.
    3. Create `a.db` through A, read it through B.
    4. Read the server join records or the signed client ids and confirm
       they differ.
  - Example: `stat -c %i mnt-a/a.db mnt-b/a.db` returns the same protocol
    inode, and `ls -l mnt-b/a.db` shows the file.
  - Acceptance:
    - AC-2.4: the file written through A is readable through B with the same
      size and contents.
    - AC-2.5: two distinct client ids appear in the server's output, and
      neither is refused.
  - Wrong-way: mounting two different namespaces and calling it a two-mount
    test.

- [ ] **T4 - Cross-mount writer serialization**
  - Goal: four writers, split across mounts, cannot enter a transaction at
    the same time.
  - Files: none.
  - Requirements:
    1. Four writer processes, two per mount, with `BEGIN IMMEDIATE`.
    2. 50 rows each, distinct markers.
    3. `busy_timeout=5000`.
  - Acceptance:
    - AC-2.6: `count(*)` is 200, `count(distinct who)` is 4, 50 each.
    - AC-2.7: `integrity_check` = `ok` from a connection on A and from a
      connection on B.
    - AC-2.8: no process exits with `database is locked`.
  - Wrong-way: running all four writers on one mount. That passes today and
    proves nothing.

- [ ] **T5 - Reader blocks a writer across mounts**
  - Goal: the shared range is honored between mounts.
  - Files: none.
  - Requirements:
    1. Reader on A holds a transaction open.
    2. Writer on B runs `BEGIN IMMEDIATE` and must not fail at once.
    3. Reader commits; writer commits.
  - Acceptance:
    - AC-2.9: the writer is still running after 1s, then commits inside
      `busy_timeout`.
    - AC-2.10: `database is locked` does not appear in the writer's output.
  - Wrong-way: interpreting a 5s wait followed by success as a failure. That
    is the correct behaviour; the failure is an immediate error.

- [ ] **T6 - Pending byte and starvation**
  - Goal: record what a new reader does while a writer waits.
  - Files: none.
  - Requirements:
    1. Writer on B holds RESERVED and waits for EXCLUSIVE.
    2. New reader on A starts.
    3. Record the delay and the outcome for both.
  - Acceptance:
    - AC-2.11: the writer commits, the reader completes, and
      `integrity_check` = `ok` afterwards.
    - AC-2.12: no party stalls longer than `busy_timeout` plus 2s.
  - Note: either a short reader delay or an immediate grant is acceptable.
    A permanent stall, or a second writer, is not.

- [ ] **T7 - Interrupt a parked lock**
  - Goal: `EINTR` works and leaves no stale state.
  - Files: a throwaway program under `/tmp`, not committed.
  - Requirements:
    1. A takes `F_WRLCK` on the shared range.
    2. B blocks in `F_SETLKW` on the same byte.
    3. `SIGINT` B.
  - Acceptance:
    - AC-2.13: B exits with `EINTR` and does not hold a lock.
    - AC-2.14: after A releases, a third `F_SETLK` succeeds on the first
      try.
    - AC-2.15: `Waiters` for that inode returns to zero (check the server's
      own log or a small test hook).
  - Wrong-way: sending `SIGKILL`. That tests death, not interrupt.

- [ ] **T8 - Mount death releases locks**
  - Goal: a dead mount cannot block the namespace.
  - Files: none.
  - Requirements:
    1. A holds `F_WRLCK`.
    2. Unmount or kill A.
    3. B tries `F_SETLK`, then a parked `F_SETLKW`.
  - Acceptance:
    - AC-2.16: B fails `EAGAIN` for up to 8s, then succeeds.
    - AC-2.17: a parked B wakes and is granted after the hold expires, with
      no manual intervention.
    - AC-2.18: a rejoin at 2s keeps A's lock, so B still gets `EAGAIN`.
  - Wrong-way: expecting an immediate release. The 8s hold is deliberate.

- [ ] **T9 - flock and fcntl regression on one mount**
  - Goal: enabling lock forwarding did not regress local locking.
  - Files: a throwaway program under `/tmp`, not committed.
  - Requirements:
    1. `flock(LOCK_EX|LOCK_NB)` on two processes: second gets `EWOULDBLOCK`.
    2. `flock(LOCK_UN)` releases.
    3. `fcntl` read and write locks still conflict and release.
  - Acceptance:
    - AC-2.19: all three behave as on a local disk.
    - AC-2.20: no `EOPNOTSUPP` appears in the output.
  - Wrong-way: testing only `fcntl`. `EnableLocks` also turns on
    `CAP_FLOCK_LOCKS`, so `flock` is the regression risk.

- [ ] **T10 - Record the run**
  - Goal: a future session can reproduce and trust the result.
  - Files: `goals/sqlite-rollback-journal/evidence.md`, `ROADMAP.md`,
    `README.md` smoke checklist.
  - Requirements:
    1. Evidence table: date with offset, commit, SQLite, kernel, flags, each
       command and exit code, assertion outputs.
    2. `ROADMAP.md` gets a short entry in the style of the M3 paragraph.
    3. README checklist gains the two-mount SQLite steps.
    4. Evidence ends with a standalone `Verdict: PASS`.
  - Acceptance:
    - AC-2.21: every row of AC-2.2 to AC-2.20 appears in evidence with a
      result.
    - AC-2.22: `grep -n "Verdict: PASS" evidence.md` matches.
    - AC-2.23: `git status` is clean after the commit.

## Failure handling

If any part of T4 to T9 fails:

1. Capture the exact command, output and exit code.
2. Determine which goal owns the defect: table (goal 1), wait (goal 1),
   mount wiring (goal 1), or harness (this goal).
3. If the defect is in goal 1, record a `lesson` with the symptom, cause and
   fix, reopen `advisory-record-locks`, and stop this goal.
4. Do not weaken an assertion to make the run pass. Record the failure and
   the decision needed.

## Definition of done

```
just check
go test -race ./...
sila gate
```

Plus a complete `evidence.md` and a `ROADMAP.md` entry with the versions and
date of the accepted run.
