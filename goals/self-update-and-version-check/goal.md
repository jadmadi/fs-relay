# Goal: Add natural-language `fs-relay update` self-upgrade and 12-hour cached version check

## Chain of Custody (Isnad)
- **Registered By**: @golang-implementer
- **Spec Audited By**: @golang-implementer
- **Implemented By**: @golang-implementer

## Goal Description
Implement release lifecycle discipline for `fs-relay` and `fs-relay-bind`:
1. Support natural-language CLI commands `fs-relay update`, `fs-relay version`, and `fs-relay help` (alongside `-update`, `-version`, and `-h`) in `cmd/fs-relay`, and `fs-relay-bind update` (alongside `-update`, `-version`, and `-h`) in `cmd/fs-relay-bind`, while still rejecting unknown positional arguments (`fs-relay extra`) with exit code `2`.
2. Perform a non-blocking version check at most once every 12 hours (`const updateCheckTTL = 12 * time.Hour`), caching the timestamp, latest tag, release URL, value-focused summary, and contributor handles in `<cacheDir>/fs-relay/update_check.json` (mode `0600`).
3. Format version announcements on `os.Stderr` around concrete user value (stripping raw commit hashes and internal goal tags) and credit contributors by `@handle`, offering `fs-relay update` as the direct upgrade command.
4. When `fs-relay update` runs, fetch the latest GitHub release metadata (`repos/muayyad-alsadi/fs-relay/releases/latest`), download `fs-relay-<GOOS>-<GOARCH>.tar.gz`, verify its SHA-256 digest against the release asset `digest` or `checksums.txt`, and atomically replace `fs-relay` (and a co-located `fs-relay-bind` when present in the same directory) using `.new` and `.old` rename with rollback on error.
5. Update `justfile` and `.github/workflows/release.yml` to generate and upload `dist/checksums.txt` alongside `dist/*.tar.gz`.

## Dependencies & Execution Order
- **Mode**: Sequential
- **Depends On**: single-serve-data-lock
- **Sequence**: 17
- **Shape**: ship
- **Tier**: roadmap
- **Memory Protocol**: required

## Context
- **Goal ID**: `self-update-and-version-check`
- **One-line why**: Sysadmins operating `fs-relay` need a 12-hour cached release check with value-focused release notes, contributor credits, and a single natural-language `fs-relay update` command that verifies SHA-256 checksums and atomically replaces the binary without third-party dependencies.
- **Repository / cwd**: `/mnt/Jad/github/lab/fs-relay` (`muayyad-alsadi/fs-relay`)
- **Prerequisite task IDs**: `single-serve-data-lock` (`goals/single-serve-data-lock/goal.md`)
- **Readiness**: READY

## Files to Touch
- cmd/fs-relay/main.go
- cmd/fs-relay/cli.go
- cmd/fs-relay/update.go
- cmd/fs-relay/cli_test.go
- cmd/fs-relay-bind/main.go
- cmd/fs-relay-bind/cli.go
- cmd/fs-relay-bind/cli_test.go
- justfile
- .github/workflows/release.yml

## File & Symbol Scope
- Edit:
  - `cmd/fs-relay/main.go` (`run`)
  - `cmd/fs-relay/cli.go` (`checkNoArgs`, `usageText`)
  - `cmd/fs-relay/cli_test.go` (`TestCheckNoArgs`, `TestUpdateCheckCacheTTLAndFormat`, `TestSelfUpdateAtomicSwapAndChecksum`)
  - `cmd/fs-relay-bind/main.go` (`run`)
  - `cmd/fs-relay-bind/cli.go` (`usageText`)
  - `cmd/fs-relay-bind/cli_test.go` (`TestBindSubcommands`)
  - `justfile` (`release`)
  - `.github/workflows/release.yml` (`files`)
- Create:
  - `cmd/fs-relay/update.go` (`updateCheckTTL`, `updateConfig`, `releaseInfo`, `checkUpdateCached`, `runSelfUpdate`, `formatValueSummary`, `compareSemver`)
- Read:
  - `AGENTS.md` (`## Dependencies`, `## Errors`, `## Functions`)
  - `SPEC.md` (`## Command line`)

## Inline Reference Excerpts

### `justfile` (`lines 31-36`)
```just
# linux tarballs in dist/, one per architecture
release:
    mkdir -p dist
    just _pack amd64
    just _pack arm64
    (cd dist && sha256sum fs-relay-linux-*.tar.gz > checksums.txt)
```

### `.github/workflows/release.yml` (`lines 25-28`)
```yaml
      - uses: softprops/action-gh-release@v2
        with:
          files: |
            dist/*.tar.gz
            dist/checksums.txt
          generate_release_notes: true
```

## Decisions & Requirements Fixed Before Dispatch

| Decision | Fixed Value | Rationale |
|----------|-------------|-----------|
| Natural-language subcommands | `fs-relay update`, `fs-relay version`, `fs-relay help` (plus `-update`, `-version`, `-h`) | Matches user preference (`fs-relay update`) and Issue `#8` (`or implement help and version`) while rejecting unknown positionals with exit `2` |
| Version check TTL | `const updateCheckTTL = 12 * time.Hour` | Requested 12-hour interval; persisted in `<cacheDir>/fs-relay/update_check.json` (`0600`) |
| Non-blocking startup behavior | When cache is fresh (`< 12h`), read locally with zero network I/O; when stale, query with a `1500ms` timeout and ignore network failures | Never blocks `fs-relay` startup when offline |
| Opt-out & dev builds | Skip automatic check when `os.Getenv("FS_RELAY_NO_UPDATE_CHECK") != ""` or `buildVersion()` starts with `(devel)` / `devel` | Prevents network calls during unit tests and local dev builds |
| Checksum verification | Verify downloaded `.tar.gz` SHA-256 against GitHub asset `digest` (`sha256:<hex>`) or `checksums.txt` before extraction | Prevents installing a corrupted or tampered archive |
| Atomic replacement | Write to `<exe>.new.<pid>` (`0755`), `os.Rename(exe, exe+".old")`, `os.Rename(tmp, exe)`, remove `.old` (or restore `.old` on rename failure); also update sibling `fs-relay-bind` in `filepath.Dir(exe)` if present | Leaves existing binary byte-identical on any failure |
| Value changelog & contributor credit | Strip raw commit SHAs and `[goal: ...]` tags; list concrete highlights and credit `@handle` authors/contributors in update notices and `fs-relay update` output | Enforces Go Application Lifecycle and Update Standards |

## Interfaces / Signatures (Fixed)
```go
// In cmd/fs-relay/update.go:
const updateCheckTTL = 12 * time.Hour

type updateConfig struct {
	CurrentVersion string
	APIBase        string
	CacheFile      string
	ExePath        string
	GOOS           string
	GOARCH         string
	Client         *http.Client
	Stderr         io.Writer
	Stdout         io.Writer
}

// checkUpdateCached checks for a newer release at most once every 12 hours and
// writes a value-focused upgrade notice to cfg.Stderr when a newer tag exists.
func checkUpdateCached(ctx context.Context, cfg updateConfig) error

// runSelfUpdate checks the latest release, verifies the tarball SHA-256 digest,
// and atomically replaces cfg.ExePath (and sibling fs-relay-bind if present).
func runSelfUpdate(ctx context.Context, cfg updateConfig) error
```

## Constraints and Non-Goals
- **Five-package ceiling**: Do not add a sixth Go package (`AGENTS.md`); place `update.go` in `cmd/fs-relay`.
- **Zero third-party dependencies**: Use Go standard library (`net/http`, `encoding/json`, `archive/tar`, `compress/gzip`, `crypto/sha256`, `encoding/hex`, `os`, `path/filepath`).
- **Preserve `-version` stdout**: `fs-relay -version` and `fs-relay version` print `fs-relay <version>` on `os.Stdout`; any update notice is written exclusively to `os.Stderr`.
- **No network in unit tests**: All tests in `cmd/fs-relay/cli_test.go` must use `httptest.NewServer` and `t.TempDir()`.

## Concrete Input / Output Examples & Boundary Cases
- **Example 1 (`fs-relay update` when newer version `v0.2.0` exists)**:
  - Input: Current version `v0.1.2`; `httptest.Server` serves `v0.2.0` release with `fs-relay-linux-amd64.tar.gz` and `digest: "sha256:<hex>"`.
  - Output: Replaces `cfg.ExePath` atomically with the `v0.2.0` binary (`0755`), prints value-focused highlights and contributor credits (`Thanks to: @muayyad-alsadi, @jadmadi`), and updates `update_check.json`.
- **Example 2 (12-hour TTL cache hit)**:
  - Input: `update_check.json` written `2 hours` ago (`< 12h`) with `LatestTag: "v0.2.0"`, current `v0.1.2`.
  - Output: `checkUpdateCached` prints the upgrade notice to `cfg.Stderr` without making any HTTP request to `cfg.APIBase`.
- **Boundary Case (SHA-256 mismatch during `fs-relay update`)**:
  - Input: Downloaded `.tar.gz` bytes do not match the asset `sha256:` digest.
  - Expected: `runSelfUpdate` returns a checksum mismatch error, removes any temporary file, and leaves `cfg.ExePath` byte-for-byte unchanged.

## Acceptance Criteria
- **AC-001 (12-hour TTL cache, value changelog, and contributor credit unit tests)**:
  - Command (`cwd`: `/mnt/Jad/github/lab/fs-relay`): `go test -race -v -run TestUpdateCheckCacheTTLAndFormat ./cmd/fs-relay`
  - Expected: Exit `0`; verifies fresh cache skips HTTP (<12h), stale cache (>12h) refreshes, raw commit hashes are stripped from value summary, and `@handle` contributors are credited.
- **AC-002 (Atomic self-update and negative SHA-256 mismatch test)**:
  - Command (`cwd`: `/mnt/Jad/github/lab/fs-relay`): `go test -race -v -run TestSelfUpdateAtomicSwapAndChecksum ./cmd/fs-relay`
  - Expected: Exit `0`; verifies atomic swap of `fs-relay` and co-located `fs-relay-bind` on valid SHA-256, and verifies existing binary is untouched on SHA-256 mismatch or already-up-to-date version.
- **AC-003 (Natural-language CLI commands and repo hygiene)**:
  - Command (`cwd`: `/mnt/Jad/github/lab/fs-relay`): `test -z "$(gofmt -l .)" && go vet ./... && CGO_ENABLED=0 go test ./... && go test -race ./...`
  - Expected: Exit `0` with empty `gofmt -l .` output.

## If Blocked
Stop affected work; preserve valid changes and report evidence and the decision needed. Do not guess or widen scope.

## Return Schema
```json
{
  "status": "ready_for_review",
  "summary": "string",
  "changed_files": [
    "cmd/fs-relay/main.go",
    "cmd/fs-relay/cli.go",
    "cmd/fs-relay/update.go",
    "cmd/fs-relay/cli_test.go",
    "cmd/fs-relay-bind/main.go",
    "cmd/fs-relay-bind/cli.go",
    "cmd/fs-relay-bind/cli_test.go",
    "justfile",
    ".github/workflows/release.yml"
  ],
  "case_results": [
    {"id": "AC-001", "status": "pass"},
    {"id": "AC-002", "status": "pass"},
    {"id": "AC-003", "status": "pass"}
  ],
  "commands_exits": [{"command": "go test -race ./...", "exit_code": 0}],
  "evidence": "goals/self-update-and-version-check/evidence.md",
  "candidate": "commit-sha",
  "blocker": null
}
```

## References
- **Shared Understanding & Fact Sheet**: [`goals/self-update-and-version-check/facts.md`](facts.md)
- **Execution Plan**: [`goals/self-update-and-version-check/plan.md`](plan.md)

## Memory Plan
- **Prime**: run `sila prime` at task start.
- **Recall**: run `sila recall` before implementation.
- **Writes**: record a grounded `memo` or `lesson` for `cmd/fs-relay/update.go`.
- **Facts**: run `sila facts verify` before certification.
- **Handoff**: run `sila handoff` at session end.
- **Evidence**: embed a `sila/memory-writes/v1` block under `## Memory Writes`.

## Done Condition
1. All tasks in `plan.md` are checked (`- [x]`).
2. Implementation is committed with `[goal: self-update-and-version-check]` attribution.
3. Goal is certified (`goals/self-update-and-version-check/certification.json` and `audit-receipt.json`) and `evidence.md` contains a standalone `Verdict: PASS` line.
4. `sila goals --json` reports `status: "implemented"`, `verification_status: "verified"`, and `unverified_count: 0`.
