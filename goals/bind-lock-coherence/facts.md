# Facts: Drain write-behind before unlock and drop cached reads on lock grant

## Verified Current State

Facts observed at `upstream/main` (`90e87b0`). Every row names the file and
symbol where the implementer can verify it.

| Fact | Where |
| ---- | ----- |
| Writes smaller than `writeMin` (`rawMin` or `writeBehindMin = 131072`) are buffered in `h.wb.buf` and recorded in `n.dirty[h]` without a network call | `cmd/fs-relay-bind/writeback.go`, `(*handle).Write` (lines 93-134) |
| `(*node).drain(ctx)` flushes every dirty handle in `n.dirty` and returns the first non-zero `syscall.Errno` | `cmd/fs-relay-bind/writeback.go`, `(*node).drain` (lines 203-220) |
| `(*handle).Read`, `(*node).setattr`, `(*handle).setattr`, and `(*node).CopyFileRange` all call `n.drain(ctx)` before issuing their server RPC | `cmd/fs-relay-bind/handle.go` (lines 59, 163), `cmd/fs-relay-bind/node.go` (lines 170, 284-287) |
| `(*node).lock` sends `wire.Getlk`, `wire.Setlk`, and `wire.Setlkw` without calling `n.drain(ctx)` | `cmd/fs-relay-bind/lock.go`, `(*node).lock` (lines 65-83) |
| On `close(2)`, Linux VFS calls `locks_remove_posix` (`FUSE_SETLK` with `F_UNLCK`) synchronously before queuing `FUSE_RELEASE` | Linux `fs/locks.c`, `locks_remove_posix`; `cmd/fs-relay-bind/lock.go` |
| `(*node).forgetAttr()` clears `n.known = false` under `n.mu` | `cmd/fs-relay-bind/node.go`, `(*node).forgetAttr` (lines 133-137) |
| `(*kernel).forgetIno(ino)` drops the prefetched small-file body `k.body[ino]` under `k.mu` | `cmd/fs-relay-bind/cache.go`, `(*kernel).forgetIno` (lines 114-118) |
| `(*handle).dropCache()` already calls `n.forgetAttr()` and `n.k.forgetIno(h.ino)` after local writes and `CopyFileRange` | `cmd/fs-relay-bind/writeback.go` (lines 247-255), `cmd/fs-relay-bind/node.go` (lines 299-302) |
| `SPEC.md` line 214 lists `fsync`, `release`, `read`, `setattr`, and `copyfilerange` as write-behind send triggers, omitting unlock | `SPEC.md`, section `Coherence` (line 214) |

## Architectural Invariants & Constraints

1. Five packages only (`cmd/fs-relay`, `cmd/fs-relay-bind`, `internal/wire`,
   `internal/local`, `internal/client`); do not add a sixth.
2. Never hold `n.mu` or `h.omu` across `n.drain(ctx)` or `n.c.Call(ctx, ...)`.
3. If `n.drain(ctx)` fails with a non-zero `syscall.Errno` during an unlock,
   `(*node).lock` must still execute `n.c.Call(ctx, method, p)` to release the
   server lock, then return the drain errno if the unlock RPC succeeded (matching
   `(*handle).Release` in `cmd/fs-relay-bind/handle.go:107-140`).
4. Keep functions under roughly 40 lines and cyclomatic complexity well below the
   ripwire bar (`bar=15`).
5. Update `SPEC.md` in the same change to document that unlocking a file sends
   held write-behind bytes first.

## File & Interface Contracts

Target function in `cmd/fs-relay-bind/lock.go` (current excerpt at `90e87b0`):

```go
func (n *node) lock(ctx context.Context, method string, f fs.FileHandle, owner uint64, lk *fuse.FileLock, flags uint32) (wire.Result, syscall.Errno) {
	p, errno := lockParams(n, f, owner, lk, flags)
	if errno != 0 {
		return wire.Result{}, errno
	}
	h, _ := f.(*handle)
	grant := method != wire.Getlk && p.Lock != wire.LockUnlock
	if h != nil && grant {
		if !h.beginOp() {
			return wire.Result{}, syscall.EINTR
		}
		defer h.endOp()
	}
	if h != nil && h.fh == 0 && grant {
		h.addOwner(owner)
	}
	res, err := n.c.Call(ctx, method, p)
	return res, toErrno(err)
}
```

Required behavior of `(*node).lock`:

1. When `method != wire.Getlk && p.Lock == wire.LockUnlock`:
   - Call `drainErr := n.drain(ctx)` before `n.c.Call(ctx, method, p)`.
   - Send `res, err := n.c.Call(ctx, method, p)` regardless of `drainErr`.
   - If `drainErr != 0`, return `res, drainErr`; otherwise return `res, toErrno(err)`.
2. When `grant` is true (`method != wire.Getlk && p.Lock != wire.LockUnlock`)
   and `err == nil`:
   - Call `n.forgetAttr()`.
   - If `n.k != nil`, call `n.k.forgetIno(n.ino)`.
   - Return `res, 0`.
