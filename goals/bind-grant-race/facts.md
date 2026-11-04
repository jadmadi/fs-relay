# Facts: Stop a granted lock outliving its handle

## Architectural Invariants & Constraints
- Ground all implementations in official repository standards.
- Ensure deterministic verification before closing goal.

## File & Interface Contracts
- Relevant files:
  - `cmd/fs-relay-bind/handle.go`
  - `cmd/fs-relay-bind/lock.go`
  - `cmd/fs-relay-bind/lock_release_test.go`
