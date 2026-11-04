# Plan: Stop a granted lock outliving its handle

Read `facts.md`. Work on `main`, then publish from a clean branch.

## Execution Steps

- [x] **T1 - Mark the handle closed**
  - Goal: a grant can tell that its handle is gone.
  - Files: `cmd/fs-relay-bind/handle.go`.
  - Requirements: `Release` sets a `closed` flag under `omu` before it takes
    owners or sends anything; `markClosed` and `isClosed` are the only access.
  - Acceptance: `TestGrantAfterReleaseIsUndone` sees the flag.

- [x] **T2 - Wait for in-flight grants**
  - Goal: no grant lands after the release that follows a close.
  - Files: `cmd/fs-relay-bind/lock.go`, `cmd/fs-relay-bind/handle.go`.
  - Requirements: a grant registers as in flight; a grant that starts after
    the close is refused with `EINTR` without a call; `Release` waits for the
    in-flight grants, then releases.
  - Acceptance: `TestReleaseWaitsForInFlightGrant` and
    `TestGrantAfterCloseIsRefused` pass.

  A rollback unlock was rejected: it names the owner and range, and the server
  has no grant identity, so it could drop a newer grant from another handle of
  the same process.

- [x] **T3 - Release owners on a failed close**
  - Goal: a sticky writeback error does not keep locks.
  - Files: `cmd/fs-relay-bind/handle.go`.
  - Requirements: the read-only release path always calls `releaseOwners`; the
    sticky error still wins the return value.
  - Acceptance: `TestReleaseWithErrorStillDropsOwners` passes.

- [x] **T4 - Tests, SPEC and evidence**
  - Files: the tests above, `SPEC.md`, `goals/bind-grant-race/evidence.md`.
  - Acceptance: `gofmt -l .` empty, vet, test and race pass; evidence has
    `Verdict: PASS`.

## Definition of done

```
just check
just race
```
