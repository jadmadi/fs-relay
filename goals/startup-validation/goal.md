# Goal: Validate the mountpoint, data directory and namespace

## Goal Description

Close the three assigned startup issues:

- #12: a non-empty mountpoint is mounted over silently.
- #14: the mount connects before checking the mountpoint, the server does not
  prepare its data root, and a missing namespace is created on first use.
- #6: a `-data` path that is a file is accepted, then every access fails.

The work spans both binaries. `fs-relay` prepares and validates its data root
but never creates `data/<namespace>`. `fs-relay-bind` checks the mountpoint
before any network connection, and refuses a non-empty mountpoint unless
forced.

## Dependencies & Execution Order

- **Mode**: Independent
- **Depends On**: none
- **Sequence**: 2
- **Shape**: ship
- **Tier**: roadmap
- **Memory Protocol**: required

## Decisions fixed before dispatch

| Decision | Value | Reason |
|----------|-------|--------|
| Force flag | `-force-existing-mnt` | Matches the maintainer's `-force-*` convention in #7 |
| Data root | Create it at startup with mode 0755 | #14 says "assert the creation of data" |
| Namespace | Never create it; require the operator to create it | #14 says "not data/namespace" |
| Missing namespace | HTTP 404 with a one-line message on join and websocket admission; `/fs/` calls keep the encoded filesystem error | It is a configuration error, not a call error |
| Namespace check | Once per namespace, positive results cached (not implemented; #28) | Avoid a stat per call |
| Mountpoint order | Stat and emptiness check before dialing | #14 asks for the check first |
| Empty check | `ReadDir`; an empty dir is fine | Hidden files count as non-empty |
| Logging | Log the resolved data path beside the listen URL | #6 asks for a clear startup |
| #7 | Out of scope | Not assigned; only its naming convention is used |

## Out of scope

- #7 multi-serve lock. Not assigned.
- #8 unknown positional arguments. Not assigned.
- #5 environment token. Answered separately; closed by #17.
- WAL, ACLs, hardlinks.
- Turning a missing namespace into an automatic creation.

## References

- `internal/local/path.go`, `tree.go`, `call.go`
- `cmd/fs-relay/main.go`, `serve.go`, `join.go`, `ws.go`
- `cmd/fs-relay-bind/main.go`
- Issues #6, #12, #14 and the maintainer's comments
- #7 for the `-force-*` / `-allow-*` convention

## Done Condition

1. All tasks in `plan.md` are checked.
2. The change lands in the PR branch with
   `(goals/startup-validation/goal.md)`.
3. `gofmt -l .`, `go vet ./...`, `go test ./...` and `go test -race ./...`
   pass.
4. A two-mount smoke run shows no regression.
5. Evidence has a standalone `Verdict: PASS`.
