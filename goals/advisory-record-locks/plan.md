# Plan: Advisory record locks across mounts

Read `facts.md` first. Names, ranges, record fields and errno values are
fixed there. Do not rename them without a memo and a `SPEC.md` edit.

Every task states files, requirements, acceptance and the failure mode most
likely to be implemented wrong. Run `sila gate` before each commit.

## Execution Steps

- [x] **T1 - Freeze the wait model and the contract in SPEC**
  - Goal: make the prose match the code that will exist, before coding.
  - Files: edit `SPEC.md` (Envelope, Locks, Testing, Cost); edit
    `ROADMAP.md` only if a claim is wrong.
  - Requirements:
    1. Replace the two conflicting waiting sentences with the server-held
       model from `goal.md`. Say that POST holds the response and the
       websocket sends `pending` then the final reply with the same id.
    2. State POSIX any-close: a `release` drops that owner's locks on that
       inode.
    3. State that `flock` shares the table as a whole-file entry.
    4. State that partial unlock of a canonical range splits into overflow.
    5. Correct the "go-fuse answers ENOSYS" sentence: the kernel never sends
       the call because the mount does not advertise `FUSE_POSIX_LOCKS`.
  - Constraint: documentation only. No code in this task.
  - Acceptance:
    - AC-1.1: `grep -n "does not hold" SPEC.md` returns nothing.
    - AC-1.2: `grep -n "any-close\|any close" SPEC.md` matches.
    - AC-1.3: `git diff --stat` shows only `SPEC.md` and `ROADMAP.md`.
  - Wrong-way: editing the wire comment in `wire.go` and calling it the
    contract. The contract lives in `SPEC.md`.

- [x] **T2 - Wire fields, methods and the lock result**
  - Goal: the message can carry a lock request and a `getlk` answer.
  - Files: edit `internal/wire/fsrelay.fbs`, regenerate
    `internal/wire/fsrelay_generated.go` with `just flatc`, edit
    `internal/wire/wire.go`, `internal/wire/fb.go`, add cases to
    `internal/wire/wire_test.go`.
  - Requirements:
    1. Add `owner:ulong`, `pid:uint`, `lock:uint` to `FbParams`.
    2. Add `table FbLock { typ:uint; start:ulong; end:ulong; pid:uint; }` and
       `lock:FbLock` to `FbResult`.
    3. Add `Getlk`, `Setlk`, `Setlkw` constants and map them in the encode
       and decode paths.
    4. Keep `FSR1` as the file identifier.
  - Constraint: `internal/wire` stays free of `net/http`, go-fuse and `os`.
  - Example: a `getlk` result with `typ=F_UNLCK` must decode to a result
    whose lock is present, not nil.
  - Acceptance:
    - AC-2.1: `go test ./internal/wire` passes.
    - AC-2.2: a round-trip test covers `setlk`, `setlkw` and `getlk`, and a
      `getlk` with all-zero lock fields.
    - AC-2.3: `just flatc` produces no diff after gofmt.
  - Wrong-way: reusing `mode` for the lock type and leaving `lock` in
    `FbResult` out. `setattr` already uses `mode`.

- [x] **T3 - Per-inode entry and record allocation**
  - Goal: an inode can carry one lock record, nil until first use.
  - Files: edit `internal/local/tree.go` (`names`, `assign`, `relOf`,
    `forget`, `drop`, `shift`), `internal/local/dir.go`,
    `internal/local/file.go`, `internal/local/publish.go`; extend
    `internal/local/local_test.go`.
  - Requirements:
    1. Replace `byIno map[uint64]string` with `byIno map[uint64]*entry`,
       where `entry{ rel string; locks *lockRec }`.
    2. `locks` stays nil until the first lock call on that inode.
    3. Rename and shift keep the same record on the same inode.
    4. A dropped inode drops its record.
    5. `fileChange` in `publish.go` keeps working through the new type.
  - Constraint: no behaviour change yet. No lock method is exposed.
  - Acceptance:
    - AC-3.1: `go test ./internal/local` passes before any lock test exists.
    - AC-3.2: a test asserts an entry keeps its `locks` pointer across a
      rename.
    - AC-3.3: a test asserts a dropped inode returns `ENOENT` from `relOf`
      and frees the record.
  - Wrong-way: a parallel `map[uint64]*lockRec`. The roadmap forbids a side
    table.

- [x] **T4 - Grant, conflict, release, getlk, overflow**
  - Goal: the table answers every POSIX case in the grant matrix.
  - Files: new `internal/local/lock.go`; new `internal/local/lock_test.go`;
    register the three methods in `internal/local/call.go`.
  - Requirements:
    1. Implement the grant table from `facts.md`.
    2. Self is never a blocker. Upgrade from read to write on the shared
       range grants when every reader is self.
    3. Partial unlock of a canonical range splits into overflow and
       decrements the canonical count.
    4. `getlk` reports the stored holder, or the first other owner in
       `owners`, or `F_UNLCK`.
    5. `flock` (`LK_FLOCK`) is a whole-file entry in the same table.
    6. Unknown lock type and unknown flag bits return `EINVAL`.
  - Constraint: hold the table mutex only for the state change. No I/O.
  - Example: owner A reads the shared range, owner B reads it, A upgrades to
    write. `setlk` returns `EAGAIN` while B holds. B releases. A retries and
    is granted.
  - Acceptance:
    - AC-4.1: table-driven test covers every grant row, both directions.
    - AC-4.2: partial unlock test: lock 0..100, unlock 40..60, then another
      owner conflicts on 0..39 and on 61..100 but not on 40..60.
    - AC-4.3: `getlk` returns `F_UNLCK` on a free range and the blocking
      owner's type, start, end and pid on a held range.
    - AC-4.4: `testing.AllocsPerRun` shows zero allocations on the second
      uncontended lock of the same inode.
  - Wrong-way: counting self as a reader, so an upgrade never grants.

- [x] **T5 - Waiter bookkeeping and the sequence counter**
  - Goal: a parked call cannot miss a release that lands before it parks.
  - Files: `internal/local/lock.go`, `internal/local/lock_test.go`.
  - Requirements:
    1. `waiting` increments on park, decrements on wake, cancel or grant.
    2. `seq` bumps on every state change on the record.
    3. Expose `Waiters(ns, ino) uint32` and a way to read `seq` before and
       after a failed grant.
    4. A release returns whether any waiter exists, so the server can skip
       the broadcast.
  - Acceptance:
    - AC-5.1: a test interleaves release and park and asserts the waiter
      retries, never sleeping forever.
    - AC-5.2: a test asserts `Waiters` returns to zero after cancel.
    - AC-5.3: a test asserts no broadcast is requested when `waiting == 0`.
  - Wrong-way: checking `waiting` outside the table mutex. That reopens the
    lost wake-up.

- [x] **T6 - Server wait loop and namespace broadcast**
  - Goal: `setlkw` blocks in the server and wakes on any namespace release.
  - Files: edit `cmd/fs-relay/serve.go`, `cmd/fs-relay/ws.go`,
    `cmd/fs-relay/hub.go`; extend `cmd/fs-relay/serve_test.go` and
    `cmd/fs-relay/ws_test.go`.
  - Requirements:
    1. POST: try, on `EAGAIN` park in the handler, retry on wake, then reply.
       Respect `r.Context()` and return `EINTR`.
    2. Websocket: send `pending` with a ticket, then the final reply with the
       same id.
    3. Add `broadcastRelease(ns, id)` modelled on `h.evict` (`fan` plus
       `holdAbsent`), not on `oweRelease`.
    4. Broadcast only when the release says a waiter exists.
    5. On client disconnect, cancel the parked call and decrement `waiting`.
  - Constraint: do not hold the hub mutex while writing to a member channel.
  - Acceptance:
    - AC-6.1: an `httptest` POST `setlkw` returns after a second client
      unlocks, within 1s.
    - AC-6.2: a websocket `setlkw` sends `pending` then a result with the
      same id, in that order.
    - AC-6.3: a release with no waiter writes no event to any subscriber.
    - AC-6.4: canceling the request returns `EINTR` and leaves `Waiters` at
      zero.
  - Wrong-way: using `oweRelease`, which targets one client. Every mount
    must be able to retry.

- [x] **T7 - Client death releases locks on the hold expiry**
  - Goal: a mount that never rejoins cannot block the namespace forever.
  - Files: `cmd/fs-relay/hub.go` (the `arm` and `dropHold` path), new hook
    to `internal/local`, tests in `cmd/fs-relay/join_test.go`.
  - Requirements:
    1. On hold expiry for a client, release every lock that client owns in
       that namespace and broadcast if waiters exist.
    2. A rejoin inside 8s keeps the locks.
    3. Existing release replay behaviour is unchanged.
  - Acceptance:
    - AC-7.1: a fake client takes a write lock, its stream drops, 8s pass,
      another client's `setlk` is granted.
    - AC-7.2: the same test with a rejoin at 2s keeps the lock and the
      second client still gets `EAGAIN`.
  - Wrong-way: releasing on detach. That breaks rejoin.

- [x] **T8 - Mount methods, EnableLocks, flock, errno**
  - Goal: the kernel forwards locks and the mount handles every one.
  - Files: new `cmd/fs-relay-bind/lock.go`; edit `node.go` or `handle.go`
    to satisfy the three interfaces; edit `main.go` `mountOptions`; new
    `cmd/fs-relay-bind/lock_test.go`; edit `errno.go` if a mapping is
    missing.
  - Requirements:
    1. Set `EnableLocks: true`.
    2. Implement `Getlk`, `Setlk`, `Setlkw` with the exact signatures.
    3. Normalise `End` to inclusive; treat the maximum offset as "to EOF".
    4. Pass `LK_FLOCK` through as a whole-file lock.
    5. Convert `EAGAIN` unchanged, and map cancel to `EINTR`.
  - Constraint: a failure in one of these must not break `open`, `read`,
    `write` or `release`.
  - Acceptance:
    - AC-8.1: `go test ./cmd/fs-relay-bind` passes.
    - AC-8.2: a table test maps `LkIn` to `wire.Params` for all three lock
      types and for to-EOF.
    - AC-8.3: the mount starts with `EnableLocks` and a single-mount
      `fcntl` test still succeeds (manual, README checklist).
  - Wrong-way: setting `EnableLocks` before the methods exist. Every lock
    call then fails with `EOPNOTSUPP`, worse than today.

- [x] **T9 - Docs, smoke checklist, evidence**
  - Goal: an operator can verify the feature without reading the code.
  - Files: `README.md` smoke checklist, `SPEC.md`, `ROADMAP.md`, new
    `goals/advisory-record-locks/evidence.md`.
  - Requirements:
    1. README gains a two-mount lock check, and a line saying a lock is
       advisory and a process that opens `data/` directly is outside it.
    2. `ROADMAP.md` moves the item from Yet to be done to Done, naming this
       goal and the commit.
    3. `evidence.md` states commands, results and a standalone
       `Verdict: PASS`.
  - Acceptance:
    - AC-9.1: `gofmt -l .` prints nothing.
    - AC-9.2: `go vet ./...`, `go test ./...`, `go test -race ./...` pass.
    - AC-9.3: `sila goals --json` shows all tasks met.

## Definition of done for the goal

Run these from the repository root and paste the exit codes into
`evidence.md`:

```
just check
go test -race ./...
sila gate
sila facts verify
```

## Order and dependencies

T1 before T2. T2 before T3 and T4. T3 before T4. T4 and T5 before T6. T6
before T7. T2, T6 and T7 before T8. Everything before T9.

T8 must not land before T4, T6 and T7 are green. Flipping `EnableLocks` with
an incomplete table turns working single-mount locks into failures.
