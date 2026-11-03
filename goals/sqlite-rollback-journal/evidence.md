# Evidence: SQLite rollback journal across two mounts

Date: 2026-11-03
Goal: `sqlite-rollback-journal`
Depends on: `advisory-record-locks` at `681826e`
Environment: Linux 6.12.113+deb13-amd64, SQLite 3.46.1, Go 1.27.2,
two mounts of one namespace, default websocket transport, `-raw` on.

## What was checked

Two mounts of one namespace, one database, rollback journal mode, SQLite
processes split across the mounts. The lock table from goal 1 is the only
thing under test; no lock code changed here except the abandoned-grant fix
listed below.

## Acceptance results

| Check | Result |
|-------|--------|
| T1 preflight: `EnableLocks` and the three node methods | pass |
| AC-2.2 single mount, 4 writers, 200 rows | 200 rows, 4 markers, `ok` |
| AC-2.3 journal mode | `delete`, never `wal` |
| AC-2.4 file written on one mount is read on the other | pass |
| AC-2.5 two distinct signed client ids | pass, see logs below |
| AC-2.6 two mounts, 4 writers, 200 rows | 200 rows, 4 markers |
| AC-2.7 `integrity_check` from both mounts | `ok` |
| AC-2.8 no `database is locked` | pass |
| AC-2.9 reader blocks writer across mounts | writer waited 1443 ms, then committed |
| AC-2.10 no immediate lock error | pass |
| AC-2.11 pending byte holds a new reader | `pread` EAGAIN while the writer waited 3599 ms |
| AC-2.12 no stall beyond `busy_timeout` plus 2 s | pass |
| AC-2.13 `SIGINT` returns `EINTR` | `fcntl: Interrupted system call` |
| AC-2.14 no stale lock after the interrupt | next `try` was granted |
| AC-2.15 waiter count returns to zero | pass via the cancel unit test |
| AC-2.16 dead mount holds up to 8 s | EAGAIN at 0 to 7 s, granted at 8 s |
| AC-2.17 parked waiter wakes after the hold | pass |
| AC-2.18 rejoin at 2 s keeps the lock | pass via `TestHoldKeepsLocksOnRejoin` |
| AC-2.19 `flock` conflicts and releases | pass, same mount and cross mount |
| AC-2.20 no `EOPNOTSUPP` | pass |
| AC-2.21 every run recorded | this file |
| AC-2.22 `Verdict: PASS` present | yes |

## Observed output

Cross-mount `fcntl`, run 1:

```
try: resource temporarily unavailable
wait: granted 2.499s
getlk: type=2 start=1073741826 len=510 pid=0
```

Two mounts, four writers, first run:

```
200 rows, 4 writers
W1:50
W2:50
W3:50
W4:50
ok
```

Reader blocks writer across mounts:

```
200
writer ok
elapsed_ms=1443
```

Pending byte and starvation:

```
pread: resource temporarily unavailable
rreleased
exclusive granted 3.599s
after writer done:
pread: granted
```

Mount death:

```
t=0s ... t=7s try: resource temporarily unavailable
t=8s try: granted
```

Interrupt at the mount:

```
fcntl: Interrupted system call
waiter_exit=0
after holder release:
try: granted
```

Distinct clients, from the mount logs:

```
client 01a1a0ea-f1aa-7000-98f0-7db603da187f.8W1yDcDUqTwN9yBhuQglOme8M2lqkcD2g2l01mPZepY
client 01a1a0eb-0744-7000-9ae8-9abf51c1d422.gPkW0KDWQHv9vNqOEcb-GP1jz46ufV6R-V1BoQXR2Gw
```

## Defect found while proving it

The mount-level interrupt first leaked a write lock: a parked `setlkw` was
granted after the caller's open had already been released. The server now
checks that the handle still exists after a grant, unlocks the range and
returns `EINTR` when it is gone. The rerun above shows no stale lock.

## Commands

```
sqlite3 --version
mount -t fuse.fs-relay ...           # two mounts, one namespace
sqlite3 <mount>/db "... CREATE TABLE ..."
sqlite3 <mount>/db "PRAGMA busy_timeout=5000; BEGIN IMMEDIATE; ..."
```

Mount-level probe programs live in `/tmp`, not the repository. The repository
builds no test harness for this; `AGENTS.md` says a real mount is a checklist,
not a unit test.

## Limits

- Three mounts were not run.
- WAL stays refused; only the rollback journal is in scope.
- The pending-byte run used `fcntl` probes with the SQLite offsets, not a
  concurrent SQLite starvation test.

## Memory Writes

```json
{
  "schema": "sila/memory-writes/v1",
  "goal_slug": "sqlite-rollback-journal",
  "prime": {"ran": true},
  "recall": [{"query": "sqlite database is locked", "matches": 0}],
  "memos": [],
  "lessons": [{"id": "fact_c14bda9a"}],
  "cases": [{"id": "fact_9c64d44b"}],
  "facts_verify": {"exit_code": 0},
  "handoff": {"id": "session-end"}
}
```

Verdict: PASS
