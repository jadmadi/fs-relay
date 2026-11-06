# Facts: Exit cleanly or with a busy error on SIGINT in fs-relay-bind

## Architectural Invariants & Constraints
- Five packages only (`cmd/fs-relay`, `cmd/fs-relay-bind`, `internal/wire`, `internal/local`, `internal/client`).
- Every goroutine must have an owner and a shutdown path (`AGENTS.md`).
- When `server.Unmount()` fails with `EBUSY`, `server.Wait()` does not return while an open file descriptor holds the mount; returning an error from `run()` exits the process (closing `/dev/fuse`) after attempting `fusermount3 -uz <mnt>`.
- Tests must use deterministic channels (`entered`, `waitCh`) and never `time.Sleep`.

## File & Interface Contracts
- `cmd/fs-relay-bind/cli.go`:
  - `mountServer` interface (`Unmount() error`, `Wait()`).
  - `lazyUnmount(mnt string) error` tries `exec.Command("fusermount3", "-uz", mnt).Run()` and falls back to `syscall.Unmount(mnt, syscall.MNT_DETACH)`.
  - `waitMount(mnt string, srv mountServer, detach func(string) error, sigc <-chan os.Signal) error` waits for `srv.Wait()` or unmounts on first signal, and detaches lazily + returns `fmt.Errorf("mount %s is busy; close open files or run fusermount3 -uz %s", mnt, mnt)` on unmount error or second signal.
- `cmd/fs-relay-bind/main.go`:
  - `run()` registers `SIGINT` and `SIGTERM` on a buffered channel (`cap 2`), defers `signal.Stop(sigc)`, and returns `waitMount(mnt, server, lazyUnmount, sigc)`.
- `cmd/fs-relay-bind/cli_test.go`:
  - `TestWaitMount(t *testing.T)` covers external unmount, clean signal unmount, busy unmount (`EBUSY`), and second signal forced exit.
