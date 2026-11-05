# Facts: Advisory record locks across mounts

## Verified current state

Facts observed in the tree at `main` (`81f2d55`) and in the pinned
dependencies. Each row names the file where you can confirm it.

| Fact | Where |
|------|-------|
| The mount never sets `EnableLocks`, so the kernel keeps local locks | `cmd/fs-relay-bind/main.go`, `mountOptions` |
| Go-fuse only advertises `FUSE_POSIX_LOCKS` when `EnableLocks` is true | `go-fuse/fuse/opcode.go`, INIT reply |
| The kernel sets `fc->no_lock = 1` when the capability is absent | Linux `fs/fuse/inode.c`, `process_init_reply` |
| With `no_lock`, the kernel serves locks locally per mount | Linux `fs/fuse/file.c`, `fuse_file_lock` |
| Go-fuse dispatches to `NodeGetlker`, `NodeSetlker`, `NodeSetlkwer` | `go-fuse/fs/bridge.go` |
| Without those methods the bridge returns `ENOTSUP` | `go-fuse/fs/bridge.go`, `defaultRawFileSystem` |
| The wire has `pending` and `release` fields | `internal/wire/fsrelay.fbs`, `FbEnvelope` |
| `MarshalPending` exists and is called only by `wire_test.go` | `internal/wire/wire.go` |
| `oweRelease` and `ackRelease` exist, called by `join_test.go` | `cmd/fs-relay/hub.go`, `cmd/fs-relay/join.go` |
| The inode map is `map[uint64]string` with no per-inode record | `internal/local/tree.go`, `names.byIno` |
| The method table has no lock entry | `internal/local/call.go`, `operations` |
| `Params` has no owner, lock type or pid | `internal/wire/wire.go` |
| `fuse.FileLock` is `{Start, End uint64; Typ, Pid uint32}` | `go-fuse/fuse/types.go` |
| `syscall.F_RDLCK`, `F_WRLCK`, `F_UNLCK` are `0`, `1`, `2` | Go `syscall` zerrors linux amd64 |
| Go-fuse sets `LK_FLOCK` in `LkFlags` for `flock` calls | `go-fuse/fuse/types.go`, `LK_FLOCK` |

## Architectural invariants

1. Only `internal/local` touches files and owns lock state. `internal/wire`
   never imports `net/http`, go-fuse or `os`.
2. The server is the only writer under `data/`. A process that opens those
   files itself is outside the table, and the docs must say so.
3. One mutex guards the lock table. It is never held across file I/O, a
   channel send, or a wake-up.
4. A lock table entry lives on the inode and dies with it. No side table, no
   second flag, per `ROADMAP.md`.
5. No allocation on the uncontended canonical path once the record exists.
6. No release broadcast when `waiting == 0`.
7. Errno travels as a number in `error.code`. No new HTTP status.
8. A behaviour change updates `SPEC.md` and `ROADMAP.md` in the same change.

## Data layout

Canonical ranges, from SQLite `os_unix.c`:

| Name | Start | Length |
|------|-------|--------|
| PENDING | `0x40000000` | 1 |
| RESERVED | `0x40000001` | 1 |
| SHARED | `0x40000002` | 510 |

Record shape. Names are fixed; the implementer may add fields but not remove
these.

```go
type lockKey struct {
	client string
	owner  uint64
}

type lockRec struct {
	shared     uint32    // readers on the shared range
	reserved   lockKey   // zero value when free
	pendingR   uint32    // readers on the pending byte
	pending    lockKey   // zero value when free
	exclusive  lockKey   // zero value when free
	waiting    uint32    // parked setlkw calls
	holder     lockKey   // one representative holder for getlk
	owners     []lockKey // dense, high-water mark, kept
	overflow   []span    // ranges outside the three canonical ones
	seq        uint64    // bumped on every change, for lost wake-ups
}

type span struct {
	key   lockKey
	start uint64 // inclusive
	end   uint64 // inclusive
	typ   uint32 // F_RDLCK or F_WRLCK
}
```

Rules that the shape implies:

- One owner appears at most once per canonical range. POSIX merges that
  owner's overlapping locks, so `shared > 1` always means another reader.
- `holder` may name a released owner. `owners` is the honest list; `getlk`
  walks it when `holder` is the caller or stale.
- `seq` is bumped on grant, release and waiter entry. A waiter reads it
  before the grant attempt and re-reads it after parking.

## File and interface contracts

`internal/wire/wire.go`, new constants:

```go
const (
	Getlk  = "getlk"
	Setlk  = "setlk"
	Setlkw = "setlkw"
)
```

`internal/wire/fsrelay.fbs`, `FbParams` additions:

```
owner:ulong;   // kernel owner id, scrambled by the kernel
pid:uint;      // kernel pid for getlk reporting
lock:uint;     // F_RDLCK, F_WRLCK, F_UNLCK
```

`start` and `length` reuse the existing `offset` and `size`. `fh` names the
open. `FbResult` gains:

```
table FbLock {
  typ:uint;
  start:ulong;
  end:ulong;
  pid:uint;
}
```

`internal/local`, exported surface. One entry per method in `operations`.

```go
// Call already exists. These are the internal hooks the server needs.
func (t *Tree) Waiters(ns string, ino uint64) uint32
func (t *Tree) Wait(ns string, ino uint64) (seq uint64)   // optional
```

`internal/client` needs no new call type. `Setlkw` is an ordinary call that
may return `pending` first on the websocket.

`cmd/fs-relay-bind`, mount methods fixed by go-fuse v2.11:

```go
Getlk(ctx context.Context, f fs.FileHandle, owner uint64, lk *fuse.FileLock,
      flags uint32, out *fuse.FileLock) syscall.Errno
Setlk(ctx context.Context, f fs.FileHandle, owner uint64, lk *fuse.FileLock,
      flags uint32) syscall.Errno
Setlkw(ctx context.Context, f fs.FileHandle, owner uint64, lk *fuse.FileLock,
       flags uint32) syscall.Errno
```

## Errno contract

| Condition | Errno |
|-----------|-------|
| `F_SETLK` conflict | `EAGAIN` |
| wait interrupted by cancel | `EINTR` |
| `flags` has an unknown bit | `EINVAL` |
| locked inode missing | `ENOENT` |
| `lock` type not one of the three | `EINVAL` |
| `flock` request that must be refused | `ENOTSUP` (only if the decision flips) |

`getlk` on a free range returns `F_UNLCK` in the result table, not an error.

## Open questions, now closed

1. Wait model: server holds the call. See `goal.md`, Decisions.
2. `flock`: same table, whole-file entry.
3. Overflow: a small slice, allocated on first arbitrary-range lock.
4. Namespace table lifetime: freed with the inode, so a restart is a remount.

## Glossary

- **owner**: a kernel lock owner, per process for classic POSIX locks.
- **canonical range**: one of the three SQLite byte ranges above.
- **overflow**: any lock outside the canonical ranges.
- **waiting**: a `setlkw` parked on one inode.
- **pending**: the websocket interim reply for a parked call.
