# Facts: Harden the lock release path

## Architectural Invariants & Constraints
- Ground all implementations in official repository standards.
- Ensure deterministic verification before closing goal.

## File & Interface Contracts
- Relevant files:
  - `internal/client`
  - `internal/local`
  - `cmd/fs-relay-bind`
