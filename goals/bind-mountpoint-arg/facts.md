# Facts: Cover the bind mountpoint argument with unit tests and a mount guard

## Architectural Invariants & Constraints
- Five packages only (`cmd/fs-relay`, `cmd/fs-relay-bind`, `internal/wire`, `internal/local`, `internal/client`).
- `fs-relay-bind` requires exactly one non-empty positional argument (`<mountpoint>`).
- Flags placed after `<mountpoint>` must continue to report `flags must come before the mountpoint: <flag>` on stderr and exit `2`.
- Unit tests in `cmd/fs-relay-bind` must not require `/dev/fuse` or root privileges.

## File & Interface Contracts
- `cmd/fs-relay-bind/cli.go`:
  - `mountArg(fs *flag.FlagSet) (string, string, error)` returns `(fs.Arg(0), "", nil)` when `fs.NArg() == 1 && fs.Arg(0) != ""` and `("", misplacedFlag(fs.Args()), errUsage)` otherwise.
- `cmd/fs-relay-bind/main.go`:
  - `run()` calls `mnt, bad, err := mountArg(flags)` once and passes `mnt` to `checkMountpoint`, `if mnt == ""`, `fs.Mount`, `log.Printf`, and `waitMount`.
- `cmd/fs-relay-bind/cli_test.go`:
  - `TestMountArg(t *testing.T)` table-tests `["/tmp/mnt"]`, `["-ns", "demo", "/tmp/mnt"]`, `[]`, `[""]`, `["/tmp/mnt", "-post"]`, and `["/tmp/a", "/tmp/b"]`.
- `README.md`:
  - `## Run` smoke paragraph includes verifying that `mount` shows the intended directory.
