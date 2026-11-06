# Facts: Reject unexpected positional arguments in fs-relay

## Architectural Invariants & Constraints
- Five packages only (`cmd/fs-relay`, `cmd/fs-relay-bind`, `internal/wire`, `internal/local`, `internal/client`).
- `errUsage` in `cmd/fs-relay/main.go` causes `main()` to exit with code `2` without printing a second `fs-relay: usage` line.
- `-version` and `-h` must continue to exit `0` and print to `os.Stdout`.
- Positional arguments must be rejected before `chooseToken` and `tree.Prepare()` so running `fs-relay help` does not create `./data` or print a generated token.

## File & Interface Contracts
- `cmd/fs-relay/cli.go`:
  - `checkNoArgs(args []string) error` returns `nil` when `len(args) == 0` and `fmt.Errorf("unknown argument %q", args[0])` when `len(args) > 0`.
- `cmd/fs-relay/main.go`:
  - `run()` calls `checkNoArgs(flags.Args())` immediately after `if *showVersion`.
  - When `checkNoArgs` returns an error, `run()` writes `fmt.Fprintf(os.Stderr, "%v\n\n%s", err, usageText(flags))` and returns `errUsage`.
- `cmd/fs-relay/cli_test.go`:
  - `TestCheckNoArgs(t *testing.T)` table-tests `nil`, `["help"]`, `["version"]`, and `["extra", "more"]`.
