# Evidence: Drop the namespace cache that nothing reads

Date: 2026-11-04
Goal: `namespace-cache`
Issue: #28
Branch: `fix/namespace-cache`, commit `1d1f4ed`

## Finding

`Tree.OpenNamespace` called `CheckNamespace` and then wrote `t.dirs[ns] = true`.
Nothing read that map. So `Call` stats the namespace on every request while the
comment says a hit is cached.

## Fix

| Change | File |
|--------|------|
| Delete the `dirs` field, its initializer and `OpenNamespace` | `internal/local/tree.go` |
| `Call` calls `CheckNamespace`, whose comment says why it runs each time | `internal/local/tree.go` |
| Replace the wrapper test and add a per-call test | `internal/local/local_test.go` |

The cache was the wrong goal: a hit would keep letting calls through after the
namespace directory is removed, and the mount would fail deeper instead of
refusing the call.

## Tests

- `TestCallChecksNamespaceEveryTime`: a call succeeds, the directory is removed,
  the next call fails with `ENOENT`.
- `TestCheckNamespace` replaces `TestOpenNamespace`.

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
  "goal_slug": "namespace-cache",
  "prime": {"ran": true},
  "recall": [{"query": "namespace cache stat per call", "matches": 0}],
  "memos": [{"id": "fact_6269194f"}],
  "lessons": [{"id": "fact_6bcf63a5"}],
  "cases": [],
  "facts_verify": {"exit_code": 0},
  "handoff": {"id": "session-end"}
}
```

Verdict: PASS
