# Facts: Close the CLI review findings on PR 19

## Verified current state

| Fact | Where |
|------|-------|
| `versionLine` reads `debug.ReadBuildInfo` and the revision | `cmd/fs-relay/cli.go`, `cmd/fs-relay-bind/cli.go` |
| Help is a hand-written string with a purpose, example and flags | same files |
| Help is pre-scanned from `os.Args` before `flag.Parse` | `run()` in both mains |
| A value `-h` after `-token` is therefore taken as help | reproduced with `fs-relay -token -h` |
| Health JSON has `status`, `time_ms`, `time` only | `cmd/fs-relay/root.go`, `writeHealth` |
| `just build` uses `-ldflags '-s -w'` only | `justfile`, `ldflags` |
| `just release` calls `_pack`, which calls `_bin` | `justfile` |
| A local build prints a pseudo-version plus the revision | `./fs-relay -version` |

## The four findings

1. Help omits the build version.
2. Health omits the version.
3. `-token -h` stops as if help was requested.
4. A tagged checkout built from source prints `(devel)`.

## Required behaviour

- `fs-relay -h` and `fs-relay-bind -h` print help to stdout, exit 0, and
  include one version line.
- `fs-relay -token -h` treats `-h` as the token value. The token is too
  short, so it stops with the token error, not help.
- `fs-relay -bogus` prints the error and usage to stderr, exit 2.
- `GET /` with `Accept: application/json` includes `"version":"..."`.
- `fs-relay -version` prints the ldflags value when set, else the build
  info, else `(devel)`.
- `just release` stamps the tag. A tarball built at `v1.2.3` prints
  `v1.2.3`.

## Error contract

| Condition | Result |
|-----------|--------|
| `-h`, `-help`, `--help` alone | help on stdout, exit 0 |
| unknown flag | error plus usage on stderr, exit 2 |
| missing flag value | error plus usage on stderr, exit 2 |
| bind with no mountpoint | usage on stderr, exit 2 |
| bind with a flag after the mountpoint | hint plus usage on stderr, exit 2 |

## Test surfaces

- `versionLine` with an empty ldflags value and a set value.
- Health JSON keys with a version.
- Help text contains the version.
- `-token -h` is not help (parse-level test).
- `just release` uses the stamped value (manual check).

## Open questions, closed

- One version helper or two? Two, one per main package. A shared package
  would add a sixth package for a few lines.
- Tag in health? Yes, the deployed version belongs there.
