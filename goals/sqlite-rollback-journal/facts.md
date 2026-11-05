# Facts: SQLite rollback journal across two mounts

## Verified baseline

From `ROADMAP.md`, the M3 run on 2026-10-30 with SQLite 3.51.2:

- One mount, one connection: insert and `integrity_check` = `ok`.
- One mount, four writers with `busy_timeout`: 200 rows committed, four
  distinct values, `integrity_check` = `ok`.
- The kernel's own locks on that mount did the waiting. The relay lock table
  was not built.
- WAL mode fails with `disk I/O error`, and stays out.

That run proves the workload and the assertions. It does not prove two
mounts. This goal repeats it across mounts and adds the negative cases.

Environment on the machine used to write this goal (record yours instead if
it differs):

| Item | Value |
|------|-------|
| SQLite CLI | `3.46.1 2024-08-13` |
| Kernel | `Linux 6.12.113+deb13-amd64` |
| Go | `go1.27.2` |
| Mount flags | default websocket, `-raw` on |
| Server | `fs-relay`, one namespace, one token |

## Why rollback journal and not WAL

WAL needs every process to map the `-shm` file with `MAP_SHARED`. A mapping
is private to one mount, so WAL fails across mounts and is out of scope. The
rollback journal uses only byte-range locks, which is exactly the mechanism
this pair of goals builds.

## SQLite lock layout used by the assertions

| Name | Offset | Length | Type |
|------|--------|--------|------|
| PENDING_BYTE | `0x40000000` | 1 | write |
| RESERVED_BYTE | `0x40000001` | 1 | write |
| SHARED_FIRST | `0x40000002` | 510 | read |

Expected observable behaviour across mounts:

- Two readers on different mounts coexist.
- A writer holds RESERVED while readers hold SHARED.
- A committer takes PENDING then waits for EXCLUSIVE on the shared range.
- A new reader that arrives while a writer waits is held at the pending byte
  for a short time, then proceeds when the writer commits or backs off.
- No process sees `database is locked` before `busy_timeout` expires.

## Harness shape

Two mountpoints, one namespace, one server. Each mount is its own client
because the server signs a new id for each `join`.

```
/tmp/fs-relay-lockcheck/
  data/                 server data dir, contains <namespace>/
  mnt-a/                mount A
  mnt-b/                mount B
  a.db                  the database, created through mnt-a
  run.log               captured stdout, stderr, exit codes
```

Rules:

- Fresh `data/` and fresh mountpoints per run.
- `sqlite3` CLI, one process per writer, so locks come from separate
  processes on possibly different mounts.
- `PRAGMA busy_timeout=5000` on every connection.
- `PRAGMA journal_mode=delete` explicitly, to prove WAL is not in play.
- Capture `sqlite3 --version` once per run.

## SQL used by the acceptance tests

Setup, through mount A:

```sql
PRAGMA journal_mode=delete;
CREATE TABLE t (id INTEGER PRIMARY KEY, who TEXT NOT NULL, n INTEGER NOT NULL);
```

Writer, one process per marker (`A`, `B`, `C`, `D`):

```sql
PRAGMA busy_timeout=5000;
BEGIN IMMEDIATE;
INSERT INTO t (who, n)
  SELECT 'A', value FROM generate_series(1, 50);
COMMIT;
```

If `generate_series` is unavailable, use a recursive CTE:

```sql
WITH RECURSIVE s(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM s WHERE i<50)
INSERT INTO t (who, n) SELECT 'A', i FROM s;
```

Assertions:

```sql
SELECT count(*) FROM t;                       -- 200
SELECT count(DISTINCT who) FROM t;            -- 4
SELECT who, count(*) FROM t GROUP BY who;     -- 50 each
PRAGMA integrity_check;                       -- ok
```

Reader block test:

1. Process R on mount A: `BEGIN; SELECT count(*) FROM t;` and stay open.
2. Process W on mount B: `BEGIN IMMEDIATE; INSERT ...;` and commit.
3. W must still be running after 1s, then finish within `busy_timeout` once
   R commits. If W fails at once with `database is locked`, the shared range
   is not being honored.

Starvation test:

1. W on mount B holds RESERVED and waits for EXCLUSIVE.
2. New reader on mount A starts. It must not be granted SHARED instantly
   while W waits, and must not be blocked forever either.
3. Record which happens, and the elapsed time. Either a short delay or an
   immediate grant is acceptable only if `integrity_check` stays `ok` and W
   eventually commits. A permanent stall is a failure.

## Interrupt and death cases

Interrupt, with a throwaway program under `/tmp` (not committed):

- Open the file, take `F_WRLCK` on byte `0x40000002` from mount A.
- From mount B, block in `F_SETLKW` on the same byte.
- Send `SIGINT` to the blocked process.
- Expect `EINTR`, a clean exit, and a later `F_SETLK` by a third process to
  succeed once A releases.

Mount death:

- Mount A takes `F_WRLCK` and holds it.
- `fusermount3 -u mnt-a` or `kill -9` the mount process.
- Mount B `F_SETLK` must fail `EAGAIN` for up to 8s, then succeed.
- If B is parked in `F_SETLKW`, it must wake and be granted after the hold
  expires. It must not hang forever.

## Failure signatures and what they mean

| Symptom | Likely cause |
|---------|--------------|
| `database is locked` at once, before `busy_timeout` | shared range or reserved byte not honored across mounts |
| `integrity_check` not `ok` | two writers entered a transaction |
| Both A and B report commit success with overlapping ids | lock grant raced; table mutex or publish path |
| `EINTR` never returns, process hangs until killed | park ignores the cancel channel |
| Lock still held after the mount dies | client-death release not wired to the hold expiry |
| `flock` fails on one mount | `EnableLocks` without `LK_FLOCK` handling |
| WAL attempt succeeds | journal mode not forced to delete |

## Numbers to record

For each run: date and local time with offset, commit, SQLite version,
kernel, mount flags, row count, distinct writers, `integrity_check` output,
elapsed wall time, and every exit code. `ROADMAP.md` gets a short entry in
the same style as the existing M3 paragraph.
