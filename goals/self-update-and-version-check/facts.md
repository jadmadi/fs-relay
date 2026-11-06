# Facts: Add natural-language `fs-relay update` self-upgrade and 12-hour cached version check

## Architectural Invariants & Constraints
- Five packages only (`cmd/fs-relay`, `cmd/fs-relay-bind`, `internal/wire`, `internal/local`, `internal/client`); do not add a sixth package (`AGENTS.md`).
- Pure Go standard library (`net/http`, `encoding/json`, `archive/tar`, `compress/gzip`, `crypto/sha256`, `encoding/hex`, `os`, `path/filepath`).
- Version check TTL is 12 hours (`const updateCheckTTL = 12 * time.Hour`), persisted in `<cacheDir>/fs-relay/update_check.json` with mode `0600`.
- `just release` packs both `fs-relay` and `fs-relay-bind` into `dist/fs-relay-linux-amd64.tar.gz` and `dist/fs-relay-linux-arm64.tar.gz`.
- GitHub Releases API (`https://api.github.com/repos/muayyad-alsadi/fs-relay/releases/latest`) provides `tag_name`, `html_url`, `body`, `author.login`, and `assets[].digest` (`sha256:<hex>`).

## File & Interface Contracts
- `cmd/fs-relay/update.go`:
  - `updateCheckTTL = 12 * time.Hour`
  - `checkUpdateCached(ctx context.Context, cfg updateConfig) error`
  - `runSelfUpdate(ctx context.Context, cfg updateConfig) error`
  - `formatValueSummary(body string, author string) (string, []string)`
  - `compareSemver(cur, latest string) int`
- `cmd/fs-relay/cli.go` & `cmd/fs-relay/main.go`:
  - Recognizes `fs-relay update`, `fs-relay version`, and `fs-relay help` (plus `-update`, `-version`, `-h`), rejects any other positional argument with `unknown argument "<arg>"` and exit code `2`, and runs the non-blocking 12-hour cached version check.
- `cmd/fs-relay-bind/cli.go` & `cmd/fs-relay-bind/main.go`:
  - Recognizes `fs-relay-bind update`, `fs-relay-bind version`, and `fs-relay-bind help` (when `flags.NFlag() == 0`) plus `-update`, `-version`, and `-h`.
- `justfile` & `.github/workflows/release.yml`:
  - Emits and uploads `dist/checksums.txt` (`sha256sum`) alongside `dist/*.tar.gz`.
