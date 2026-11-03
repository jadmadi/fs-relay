# Evidence: Validate the mountpoint, data directory and namespace

Date: 2026-11-03
Goal: `startup-validation`
Issues: #6, #12, #14

## What was built

1. `local.Prepare` creates the data root and rejects a path that is not a
   directory. `abs` no longer creates `data/<namespace>`.
2. `local.OpenNamespace` stats a namespace, caches a hit, and never caches a
   miss. `Call` checks it before dispatch.
3. `fs-relay` calls `Prepare` before listening and logs the resolved data
   path. `join`, `/ws/` and `/raw/` answer 404 for a missing namespace.
4. `fs-relay-bind` checks the mountpoint before dialing. A missing path, a
   file, or a non-empty directory stops with a direct message. A non-empty
   directory needs `-force-existing-mnt`.
5. The CLI refactor's `flag.Arg(0)` bug surfaced during the smoke test; the
   mount call and log now use `flags.Arg(0)`.

## Acceptance results

| Check | Result |
|-------|--------|
| AC-1.1 `Prepare` creates a missing root | pass |
| AC-1.2 `Prepare` rejects a file | pass |
| AC-1.3 `Call` on a missing namespace | `ENOENT` |
| AC-1.4 namespace created later works | pass |
| AC-2.1 `-data <file>` exits 1 | `data directory ...: not a directory`, exit 1 |
| AC-2.2 missing root created and logged | `data /tmp/.../newdata` |
| AC-2.3 missing namespace join | HTTP 404 |
| AC-2.4 existing namespace mounts | mounted, `cat` returns the file |
| AC-3.1 missing mountpoint | `mountpoint ... does not exist` |
| AC-3.2 file mountpoint | `mountpoint ... is not a directory` |
| AC-3.3 non-empty mountpoint | `... is not empty; use -force-existing-mnt` |
| AC-3.4 forced mount | mounted |
| AC-3.5 empty mountpoint | mounted |
| AC-4.1 tests and race | pass |
| AC-4.2 smoke | pass |
| AC-5.1 checks | pass |

## Commands

```
gofmt -l .
go vet ./...
go test ./...
go test -race ./...
fs-relay -data <file>                     # exit 1
fs-relay -data <missing> -token ...       # creates and logs
fs-relay-bind ... <missing mountpoint>    # exit 1
fs-relay-bind ... -force-existing-mnt ...
```

## Notes

- A missing namespace now fails at the join, so the mount stops before it
  can issue calls. The client surfaces the HTTP 404.
- A read-only data root fails when the first namespace is opened, not at
  startup. `Prepare` only creates and checks the root.
- `sila facts verify` exits 1 because the ledger already carries hash
  mismatches from other projects; the goal evidence is intact.

## Memory Writes

```json
{
  "schema": "sila/memory-writes/v1",
  "goal_slug": "startup-validation",
  "prime": {"ran": true},
  "recall": [{"query": "mountpoint namespace validation", "matches": 0}],
  "memos": [{"id": "fact_b8c70f58"}],
  "lessons": [{"id": "fact_2427549e"}],
  "cases": [],
  "facts_verify": {"exit_code": 1},
  "handoff": {"id": "session-end"}
}
```

Verdict: PASS
