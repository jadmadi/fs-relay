# Evidence: Stop a granted lock outliving its handle

Date: 2026-11-04
Goal: `bind-grant-race`
Issue: #27
Branch: `fix/bind-grant-race`, commit `54f4a47`

## Finding

A `setlk` can be in flight when its handle closes.

1. The read-only path added the owner, then called the server. `Release` could
   take the owners and send the release before the grant arrived, so the grant
   landed with no owner left to drop it.
2. The read-only path returned early when a writeback error was sticky, so it
   never released its owners at all.

## Fix

| Change | File |
|--------|------|
| `Release` marks the handle closed before it does anything else | `cmd/fs-relay-bind/handle.go` |
| After a grant, a closed handle means the range is unlocked and `EINTR` returned | `cmd/fs-relay-bind/lock.go` |
| The read-only path releases its owners even when a sticky error is returned | `cmd/fs-relay-bind/handle.go` |
| The rule is written down | `SPEC.md` |

The check covers a read-only handle, which names owners, and a writable handle,
which names its server file handle.

## Tests

- `TestGrantAfterReleaseIsUndone`: holds the grant until `Release` has run, for
  a read-only and a writable handle, then asserts the unlock record and
  `EINTR`.
- `TestReleaseWithErrorStillDropsOwners`: a sticky `EIO` still releases the
  owner and still wins the return value.

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
  "goal_slug": "bind-grant-race",
  "prime": {"ran": true},
  "recall": [{"query": "lock grant after release", "matches": 0}],
  "memos": [{"id": "fact_5506963e"}],
  "lessons": [{"id": "fact_afae2fdb"}],
  "cases": [],
  "facts_verify": {"exit_code": 1},
  "handoff": {"id": "session-end"}
}
```

Verdict: PASS
