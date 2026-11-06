# Evidence: self-update-and-version-check

Verdict: PASS

## Verification Summary
- Added `cmd/fs-relay/update.go` implementing:
  - 12-hour cached version check (`updateCheckTTL = 12 * time.Hour`) persisted atomically with mode `0600` at `<cacheDir>/fs-relay/update_check.json`.
  - Value-focused release summary (`formatValueSummary`) that strips raw commit hashes and `[goal: ...]` tags, extracts up to 3 benefit lines, and credits contributors by `@handle`.
  - Atomic self-update (`runSelfUpdate`) that downloads `fs-relay-<GOOS>-<GOARCH>.tar.gz`, verifies SHA-256 against the GitHub release asset `digest` or `checksums.txt`, and atomically swaps `fs-relay` and any co-located `fs-relay-bind` via `.new` and `.old` rename with rollback.
- Wired natural-language subcommands (`fs-relay update`, `fs-relay version`, `fs-relay help`, `fs-relay-bind update`, `fs-relay-bind version`, `fs-relay-bind help`) and updated `justfile` and `.github/workflows/release.yml` to emit `dist/checksums.txt`.

## Commands Executed
- `gofmt -l .` -> exit `0` (empty output)
- `go vet ./...` -> exit `0`
- `CGO_ENABLED=0 go test -v -run 'Test(UpdateCheckCacheTTLAndFormat|SelfUpdateAtomicSwapAndChecksum|CheckNoArgs|VersionLine|UsageTextVersion)' ./cmd/fs-relay` -> exit `0` (`PASS`)
- `CGO_ENABLED=0 go test -v -run 'Test(BindSubcommands|MountArg|VersionLine|UsageTextVersion)' ./cmd/fs-relay-bind` -> exit `0` (`PASS`)
- `go test -race ./...` -> exit `0` (`PASS` across all packages)
- `ripwire . --quality-delta --legend=compact` -> exit `0` (`gating="0"`)
