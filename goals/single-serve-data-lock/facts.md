# Facts: Guard the data directory with an exclusive lock unless forced

## Architectural Invariants & Constraints
- Only `internal/local` touches the filesystem (`AGENTS.md`).
- A boolean parameter that flips behavior is forbidden (`AGENTS.md`); `(*Tree).Lock()` is a dedicated method called when `!*forceMulti`.
- Sensitive local state files use mode `0600`.

## File & Interface Contracts
- Relevant files:
  - `internal/local/tree.go`
  - `internal/local/local_test.go`
  - `cmd/fs-relay/main.go`
  - `README.md`
  - `ROADMAP.md`
