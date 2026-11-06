# Evidence: Drain write-behind before unlock and drop cached reads on lock grant

Date: 2026-11-06
Goal: `bind-lock-coherence`
Issue: #31
Branch: `fix/lock-swarm-coherence`

## Finding

`(*node).lock` in `cmd/fs-relay-bind/lock.go` had two lock-boundary coherence gaps:

1. An explicit unlock (`fcntl(F_SETLK, F_UNLCK)` or `flock(LOCK_UN)`) or `close(2)`
   unlock (`locks_remove_posix` before `FUSE_RELEASE`) called `n.c.Call` with
   `LockUnlock` without first calling `n.drain(ctx)`. Writes smaller than
   `writeBehindMin` (128 KiB without `/raw`, 4 KiB with `/raw`) stayed in
   `h.wb.buf` while the server released the lock and woke the next writer.
2. When `Setlk` or `Setlkw` granted a read or write lock, `(*node).lock` did not
   drop `n.known` (`n.forgetAttr()`) or `n.k.body[n.ino]` (`n.k.forgetIno(n.ino)`),
   allowing `Getattr` or `Read` immediately after lock grant to serve stale
   cached state if the synchronous RPC beat the asynchronous eviction stream.

## Fix

| Change | File |
|--------|------|
| Call `n.drain(ctx)` before `n.c.Call` on `LockUnlock`, still send the unlock when `drain` fails, and drop `n.forgetAttr()` + `n.k.forgetIno(n.ino)` on lock grant | `cmd/fs-relay-bind/lock.go` |
| Add `TestUnlockDrainsWriteBehindBeforeSetlk`, `TestUnlockWithDrainErrorStillReleasesLock`, and `TestLockGrantDropsCachedAttrAndBlob` | `cmd/fs-relay-bind/lock_release_test.go` |
| Document unlock write-behind drain and grant cache drop under `Coherence` and `Locks` | `SPEC.md` |

## Tests

- `TestUnlockDrainsWriteBehindBeforeSetlk`: buffers a 3-byte write in `h.wb.buf`
  and verifies `n.Setlk(..., F_UNLCK)` sends `wire.Write` before `wire.Setlk`.
- `TestUnlockWithDrainErrorStillReleasesLock`: sets `h.wb.err = syscall.EIO` on
  a dirty handle and verifies `n.Setlk(..., F_UNLCK)` returns `syscall.EIO`
  while still sending `wire.Setlk` (`LockUnlock`).
- `TestLockGrantDropsCachedAttrAndBlob`: verifies `Getlk` and a denied `Setlk`
  preserve cached `n.attr` and `k.body[5]`, while a granted `Setlk` drops both.

## Commands

```
gofmt -l .          # empty
go vet ./...        # exit 0
go test ./...       # exit 0
go test -race ./... # exit 0
```

## Memory Writes

```json
{
  "schema": "sila/memory-writes/v1",
  "goal_slug": "bind-lock-coherence",
  "prime": {"ran": true},
  "recall": [{"query": "write-behind drain unlock cache forgetIno", "matches": 5}],
  "memos": [{"id": "fact_687fb910"}],
  "lessons": [{"id": "fact_3315951e"}],
  "cases": [],
  "facts_verify": {"exit_code": 0},
  "handoff": {"id": "hoff_83df0afe4bac"}
}
```

Verdict: PASS
