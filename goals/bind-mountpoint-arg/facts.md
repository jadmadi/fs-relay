# Facts: Cover the bind mountpoint argument with unit tests and a mount guard

## Architectural Invariants & Constraints
- `fs-relay-bind` requires exactly one non-empty positional argument (`<mountpoint>`).
- Flags placed after `<mountpoint>` must continue to report `flags must come before the mountpoint: <flag>` on stderr and exit `2`.

## File & Interface Contracts
- Relevant files:
  - `cmd/fs-relay-bind/main.go`
  - `cmd/fs-relay-bind/cli.go`
  - `cmd/fs-relay-bind/cli_test.go`
  - `README.md`
