# Facts: Reject unexpected positional arguments in fs-relay

## Architectural Invariants & Constraints
- Five packages only (`cmd/fs-relay`, `cmd/fs-relay-bind`, `internal/wire`, `internal/local`, `internal/client`).
- `errUsage` in `cmd/fs-relay/main.go` causes `main()` to exit with code `2` without printing a second `fs-relay: usage` line.
- `-version` and `-h` must continue to exit `0` and print to `os.Stdout`.

## File & Interface Contracts
- Relevant files:
  - `cmd/fs-relay/main.go`
  - `cmd/fs-relay/cli.go`
  - `cmd/fs-relay/cli_test.go`
