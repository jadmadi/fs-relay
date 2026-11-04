# Plan: Harden the lock release path

Read `facts.md`. Work on `main`, then publish from a clean branch.

## Execution Steps

- [x] **T1 - Coalescing release broadcast**
  - Goal: a release wakes every waiter and never blocks the socket or join
    reader.
  - Files: `internal/client/client.go`, `join.go`, `ws.go`,
    `cmd/fs-relay-bind/lock.go`, `node.go`, tests.
  - Requirements: `Releases()` returns the current channel; a release closes
    it and installs a new one; the bind captures it before the grant attempt.
  - Acceptance: 100 releases with no waiter do not block; two waiters both
    wake; `TestPostSetlkwReturnsEAGAINAndReleaseWakesClient` passes.

- [x] **T2 - Failed close drops locks**
  - Goal: a release that fails to write still drops its locks.
  - Files: `internal/local/file.go`, tests.
  - Requirements: compute the inode before the handle is dropped; call
    `DropFH` on every release path, including write error and short write.
  - Acceptance: `TestReleaseFailureDropsLocks` passes.

- [x] **T3 - Any close drops the owner**
  - Goal: closing one of two writable descriptors drops the process owner.
  - Files: `internal/local/lock.go`, tests.
  - Requirements: an owner keeps two handles inline plus an overflow slice;
    `dropFH` removes the owner on any matching handle; no allocation on the
    one-handle path.
  - Acceptance: `TestLockAnyCloseDropsOwner` and
    `TestLockCanonicalNoAlloc` pass.

- [x] **T4 - Tests and evidence**
  - Files: tests above, `goals/lock-robustness/evidence.md`.
  - Acceptance: `gofmt -l .` empty, vet, test and race pass; evidence has
    `Verdict: PASS`.

## Definition of done

```
gofmt -l .
go vet ./...
go test ./...
go test -race ./...
```
