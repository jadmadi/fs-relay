# Facts: Exit cleanly or with a busy error on SIGINT in fs-relay-bind

## Architectural Invariants & Constraints
- Every goroutine must have an owner and a shutdown path (`AGENTS.md`).
- When `server.Unmount()` fails with `EBUSY`, `server.Wait()` does not return while an open file descriptor holds the mount; returning an error from `run()` exits the process (closing `/dev/fuse`) after attempting `fusermount3 -uz <mnt>`.

## File & Interface Contracts
- Relevant files:
  - `cmd/fs-relay-bind/main.go`
  - `cmd/fs-relay-bind/cli.go`
  - `cmd/fs-relay-bind/cli_test.go`
