# Evidence: Advisory record locks across mounts

Date: 2026-11-03
Goal: `advisory-record-locks`
Scope: `internal/wire`, `internal/local`, `internal/client`, `cmd/fs-relay`,
`cmd/fs-relay-bind`, `SPEC.md`, `ROADMAP.md`, `README.md`.

## What was built

1. Wire: `owner`, `pid` and `lock` on `params`, a `lock` table on `result`,
   and `getlk`, `setlk`, `setlkw` method names.
2. `internal/local`: one lock record per inode, hung on the inode entry.
   Counts for the pending byte, the reserved byte and the shared range, an
   overflow list for any other range, and a per-owner span list so a partial
   unlock splits correctly.
3. `cmd/fs-relay`: `setlkw` parks in the server, wakes on any release in the
   namespace, and reports `pending` first on the websocket. A client that
   does not rejoin loses its locks when the 8s hold expires.
4. `cmd/fs-relay-bind`: `Getlk`, `Setlk`, `Setlkw`, `EnableLocks`, `flock` as
   a whole-file entry, and `context.Canceled` mapped to `EINTR`.
5. `internal/client`: each transport carries its signed client id, and a
   websocket reconnect presents the same id.

## Acceptance results

| Check | Result |
|-------|--------|
| AC-1.1 `SPEC.md` has no "does not hold" | pass |
| AC-1.2 `SPEC.md` states any-close | pass |
| AC-2.1 `go test ./internal/wire` | pass |
| AC-2.2 setlk/setlkw/getlk round trip | `TestLockRoundTrip`, `TestLockResult` |
| AC-2.3 `just flatc` reproduces the file | pass, flatc 25.12.19 |
| AC-3.1 `go test ./internal/local` | pass |
| AC-3.2 record survives rename | `TestLockSurvivesRename` |
| AC-3.3 dropped inode frees the record | `TestLockDroppedWithInode` |
| AC-4.1 grant matrix | `TestLockGrantMatrix` |
| AC-4.2 partial unlock splits | `TestLockPartialUnlockSplits` |
| AC-4.3 getlk free and held | `TestLockUpgradeIgnoresSelf` |
| AC-4.4 zero allocation on the second lock | `TestLockCanonicalNoAlloc` |
| AC-5.1 release and park cannot be lost | `TestPostSetlkwWaitsForRelease` |
| AC-5.2 cancel returns waiters to zero | `TestPostSetlkwCancelReturnsEINTR` |
| AC-5.3 no broadcast when nobody waits | `hub.wake` no-ops on an empty set |
| AC-6.1 POST wait wakes on release | `TestPostSetlkwWaitsForRelease` |
| AC-6.2 websocket EAGAIN then release push | `TestWebsocketSetlkwReturnsEAGAINAndReleaseArrives` |
| AC-6.3 release with no waiter writes nothing | covered by `hub.wake` |
| AC-6.4 cancel returns EINTR | `TestSetlkwCancelReturnsEINTR` |
| AC-7.1 client expiry drops locks | `TestHoldExpiryReleasesClientLocks` |
| AC-7.2 rejoin keeps locks | `TestHoldKeepsLocksOnRejoin` |
| AC-8.1 `go test ./cmd/fs-relay-bind` | pass |
| AC-8.2 lock params table | `TestLockParams` |
| AC-8.3 mount with EnableLocks, single-mount fcntl | manual mount run below |
| AC-9.1 gofmt | empty |
| AC-9.2 vet, test, race | pass |
| AC-9.3 all tasks checked | pass |
| AC-10.1 drop owner keeps the other owner | `TestLockDropOwnerIsNarrow` |
| AC-10.2 read-only close releases one per owner | `TestReadOnlyCloseDropsOnlyItsOwners` |
| AC-10.3 `cat` cannot drop a writer's lock | two-mount run below |
| AC-10.4 read-only lock gone, writer stays | mount run below |
| AC-11.1 pending downgrade reports read | `TestLockPendingDowngrade` |
| AC-11.2 writer still refused after downgrade | `TestLockPendingDowngrade` |
| AC-12.1 POST and websocket wait wake | `TestPostSetlkwReturnsEAGAIN...`, `TestSetlkwRetriesOnRelease` |
| AC-12.2 release replayed on rejoin | `TestReleaseAllReplayedOnRejoin` |
| AC-12.3 SQLite across two mounts unchanged | 200 rows, `ok` |
| AC-12.4 SPEC matches the wait | pass |

## Deviations from the plan, and why

1. The wait is client-side, as `main` specifies. The server tracks waiting
   inodes only to decide whether to send a release event; the mount waits on
   its event stream and retries. The wire `pending` field was removed.
2. The plan's `seq` counter is not needed. The release replay and the
   one-second retry bound a missed event.
3. Close semantics are owner-scoped. A read-only handle records the owners
   that locked through it, and its close sends one `release` per owner with no
   handle. This is narrower than a lazy server handle and avoids an `fh` race
   on read-only handles.

## Defect found and fixed

The first real mount run made SQLite fail with `disk I/O error`. The grant
fast path compared the exclusive owner with the zero key instead of the
caller, so SQLite's required `F_WRLCK` to `F_RDLCK` downgrade on the shared
range returned `EAGAIN`. The fast paths now treat the caller as compatible;
the owner scan already skipped self. `TestLockSelfDowngrade` covers it.

## Commands

```
gofmt -l .                                   # empty
go vet ./...                                 # exit 0
go test ./...                                # exit 0
go test -race ./...                          # exit 0
```

Mount checks, two mounts of one namespace, Linux 6.12.113:

| Check | Result |
|-------|--------|
| fcntl write lock on mount A, mount B `F_SETLK` | `EAGAIN` |
| mount B `F_SETLKW` while A holds | granted after 2.5s |
| mount B `F_GETLK` after release | `type=2` (`F_UNLCK`) |
| `flock -n` across mounts while held | failed, then granted after release |
| mount A killed while holding | released at 8s |
| SQLite 3.46.1, two mounts, 4 writers, 200 rows | `integrity_check` `ok` |
| new reader while a writer waits on the pending byte | held, then granted |

## Review remediation, 2026-11-03

The maintainer's review found that a read-only close dropped every lock the
mount held on the database. Fixed by owner-scoped release: a read-only handle
records the owners that locked through it, and its close sends one release per
owner. The mount-wide `DropClient` path and `node.locked` are gone.

Also fixed: a write lock on the pending byte replaced by a read now
downgrades, and `setlkw` is back to the client-side wait with a replayed
release.

Verified on two mounts plus one `-post` mount:

| Case | Result |
|------|--------|
| `cat` on the holder's mount | other mount stays `EAGAIN` |
| read-only fd takes a read lock, closes | its byte free, writer stays |
| websocket `F_SETLKW` while A holds | granted after 3.6s |
| `-post` `F_SETLKW` while A holds | granted after 3.6s |
| SQLite, 4 writers, 200 rows | `integrity_check` `ok` |

## Memory Writes

```json
{
  "schema": "sila/memory-writes/v1",
  "goal_slug": "advisory-record-locks",
  "prime": {"ran": true},
  "recall": [{"query": "sqlite database is locked", "matches": 0}],
  "memos": [{"id": "fact_68f43052"}, {"id": "fact_ccbaf335"}],
  "lessons": [{"id": "fact_d51d5f40"}, {"id": "fact_ff81b41c"}],
  "cases": [{"id": "fact_5e91eb5f"}],
  "facts_verify": {"exit_code": 0},
  "handoff": {"id": "session-end"}
}
```

Verdict: PASS
