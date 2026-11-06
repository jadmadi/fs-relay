# Facts: Guard the data directory with an exclusive lock unless forced

## Architectural Invariants & Constraints
- Only `internal/local` touches the filesystem (`AGENTS.md`: "Only `internal/local` touches the filesystem").
- A boolean parameter that flips behavior is forbidden (`AGENTS.md`: "A boolean parameter that flips behavior is two functions wearing one name"); `(*Tree).Lock()` is a dedicated method called when `!*forceMulti`.
- Sensitive local state files use mode `0600`.
- A behavior change updates `SPEC.md` and `ROADMAP.md` in the same change (`AGENTS.md`).
- `wire.CheckNS` only allows `[A-Za-z0-9_-]+`, so `<root>/.fs-relay.lock` (leading dot) can never collide with a valid namespace name.

## Source Excerpts (Baseline `upstream/main` `42a430b`)

### `internal/local/tree.go`
```go
type Tree struct {
	root string

	mu sync.Mutex
	ns map[string]*names

	fmu    sync.Mutex
	files  map[uint64]*openFile
	nextFH uint64
}
```

### `cmd/fs-relay/main.go`
```go
	tree := local.New(*data)
	defer tree.Close()
	if err := tree.Prepare(); err != nil {
		return err
	}
```

## File & Interface Contracts
- `internal/local/tree.go`:
  - `Tree.lockFile *os.File` guarded by `t.fmu`.
  - `(*Tree).Lock() error`: opens `<root>/.fs-relay.lock` (`0600`), calls `unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)`, writes `<pid>\n`, and saves `t.lockFile` under `t.fmu`.
  - `(*Tree).Close() error`: swaps `t.lockFile` to `nil` under `t.fmu` and closes it via `errors.Join`.
- `cmd/fs-relay/main.go`:
  - Registers `-force-multi-serve` (`false` by default) and calls `tree.Lock()` when `!*forceMulti`.
- `internal/local/local_test.go`:
  - `TestTreeLockRefusesSecondServerAndReleasesOnClose(t *testing.T)` verifies contention failure with PID message and subsequent acquisition after `Close()`.
- `SPEC.md`, `ROADMAP.md`, `README.md`:
  - Document the data directory lock and `-force-multi-serve`.
