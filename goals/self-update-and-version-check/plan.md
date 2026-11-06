# Plan: Add natural-language `fs-relay update` self-upgrade and 12-hour cached version check

Read `facts.md` and `goal.md` before executing any step.

## Execution Steps

- [x] **T1 - Implement 12-hour cached version check, value changelog formatter, and atomic self-updater (`AC-001`, `AC-002`)**
  - CWD: `/mnt/Jad/github/lab/fs-relay`
  - Create: `cmd/fs-relay/update.go`.
  - Requirements:
    1. Define `updateCheckTTL = 12 * time.Hour`, `updateConfig`, and `updateCache` persisted atomically with mode `0600` at `<cacheDir>/fs-relay/update_check.json`.
    2. Implement `formatValueSummary(body, author string) (string, []string)` to strip raw commit hashes and `[goal: ...]` tags, extract concrete benefit lines, and collect `@handle` contributor credits.
    3. Implement `checkUpdateCached(ctx context.Context, cfg updateConfig) error` that respects the 12-hour TTL, skips dev builds (`(devel)`/`devel`) or `FS_RELAY_NO_UPDATE_CHECK != ""`, and writes the upgrade banner with `Run: fs-relay update` to `cfg.Stderr`.
    4. Implement `runSelfUpdate(ctx context.Context, cfg updateConfig) error` that fetches the latest release, verifies the `.tar.gz` SHA-256 digest against the release asset `digest` or `checksums.txt`, and atomically swaps `fs-relay` (and sibling `fs-relay-bind` if present in `filepath.Dir(cfg.ExePath)`) via `.new` and `.old` rename with rollback on error.
  - Verification: `go vet ./cmd/fs-relay` (expected exit `0`).

- [x] **T2 - Wire natural-language `update`, `version`, and `help` commands and update release packaging (`AC-003`)**
  - CWD: `/mnt/Jad/github/lab/fs-relay`
  - Edit: `cmd/fs-relay/main.go`, `cmd/fs-relay/cli.go`, `cmd/fs-relay-bind/main.go`, `cmd/fs-relay-bind/cli.go`, `justfile`, `.github/workflows/release.yml`.
  - Requirements:
    1. Support `fs-relay update`, `fs-relay version`, and `fs-relay help` (plus `-update`, `-version`, `-h`) in `cmd/fs-relay/main.go` and `cmd/fs-relay/cli.go`, while rejecting any other positional argument with `unknown argument "<arg>"` and exit code `2`.
    2. Support `fs-relay-bind update`, `fs-relay-bind version`, and `fs-relay-bind help` (when `flags.NFlag() == 0`) plus `-update` in `cmd/fs-relay-bind/main.go` and `cmd/fs-relay-bind/cli.go`.
    3. Update `justfile` and `.github/workflows/release.yml` to generate and upload `dist/checksums.txt`.
  - Verification: `go build ./cmd/fs-relay ./cmd/fs-relay-bind` (expected exit `0`).

- [x] **T3 - Add table-driven unit tests and certify goal (`AC-001`, `AC-002`, `AC-003`)**
  - CWD: `/mnt/Jad/github/lab/fs-relay`
  - Edit: `cmd/fs-relay/cli_test.go`, `cmd/fs-relay-bind/cli_test.go`, `goals/self-update-and-version-check/evidence.md`.
  - Requirements:
    1. Add `TestUpdateCheckCacheTTLAndFormat` and `TestSelfUpdateAtomicSwapAndChecksum` in `cmd/fs-relay/cli_test.go` using `httptest.NewServer` and `t.TempDir()`.
    2. Add `TestBindSubcommands` in `cmd/fs-relay-bind/cli_test.go`.
    3. Verify `gofmt -l .`, `go vet ./...`, `CGO_ENABLED=0 go test ./...`, `go test -race ./...`, record `evidence.md` with `Verdict: PASS`, and pass `sila goals audit self-update-and-version-check` and `sila gate`.
