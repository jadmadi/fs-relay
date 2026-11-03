# Plan: Close the CLI review findings on PR 19

Read `facts.md` first. Work on `feat/cli-ergonomics`; the PR is open.

## Execution Steps

- [x] **T1 - Parse help with a FlagSet**
  - Goal: `-token -h` is a token value, not a help request.
  - Files: `cmd/fs-relay/main.go`, `cmd/fs-relay-bind/main.go`,
    `cmd/fs-relay/cli.go`, `cmd/fs-relay-bind/cli.go`.
  - Requirements:
    1. Build a `flag.NewFlagSet(name, flag.ContinueOnError)` in each
       `run()`; register the same flags on it.
    2. On `flag.ErrHelp`, print the help to stdout and return nil.
    3. On another parse error, print the error and help to stderr and
       exit 2.
    4. Remove the `helpRequested` pre-scan.
  - Acceptance:
    - AC-1.1: `fs-relay -token -h` reports the token length error, exit 1.
    - AC-1.2: `fs-relay -h` prints help to stdout, exit 0.
    - AC-1.3: `fs-relay -bogus` exits 2 with usage on stderr.
    - AC-1.4: the same three for `fs-relay-bind`.

- [x] **T2 - Put the version in help**
  - Goal: both help screens show the build version.
  - Files: the `cli.go` files, tests.
  - Requirements: add one `Version: <line>` line after the purpose.
  - Acceptance:
    - AC-2.1: `fs-relay -h` contains `Version: fs-relay`.
    - AC-2.2: `fs-relay-bind -h` contains `Version: fs-relay-bind`.

- [x] **T3 - Put the version in health**
  - Goal: the deployed version is visible over HTTP.
  - Files: `cmd/fs-relay/root.go`, `cmd/fs-relay/root_test.go`.
  - Requirements:
    1. Add `Version string \`json:"version"\`` to the health struct.
    2. Fill it from `versionLine("fs-relay")` without the binary name, or
       a dedicated `buildVersion()`.
    3. Keep `status`, `time_ms`, `time`.
  - Acceptance:
    - AC-3.1: `GET /` with `Accept: application/json` has all four keys.
    - AC-3.2: `version` is non-empty in a normal build.

- [x] **T4 - Stamp the release version**
  - Goal: a tagged build does not report `(devel)`.
  - Files: `justfile`, both `cli.go` files (a `var version string`).
  - Requirements:
    1. `versionLine` returns the ldflags value when non-empty.
    2. `just _bin` passes
       `-X main.version=$(git describe --tags --always --dirty)`.
    3. A dirty worktree is marked `-dirty`.
  - Acceptance:
    - AC-4.1: `just build` then `./fs-relay -version` prints the describe
      value.
    - AC-4.2: checking out a tag and building prints the tag.
    - AC-4.3: a local change appends `-dirty`.

- [x] **T5 - Tests, docs and evidence**
  - Files: `cmd/fs-relay/cli_test.go`, `cmd/fs-relay-bind/cli_test.go`,
    `cmd/fs-relay/root_test.go`, `README.md`, `SPEC.md` only if the health
    shape changes, `goals/cli-review-fixes/evidence.md`.
  - Requirements:
    1. Table tests for `versionLine` with and without ldflags.
    2. Health JSON test.
    3. README mentions `-version` and the version in health.
    4. Evidence with commands and `Verdict: PASS`.
  - Acceptance:
    - AC-5.1: `gofmt -l .` empty, `go vet`, `go test`, `go test -race`
      pass.
    - AC-5.2: PR #19 updated, `test` green.

## Order

T1 before T2 and T3. T4 before T5. T5 last.

## Definition of done

```
gofmt -l .
go vet ./...
go test ./...
go test -race ./...
```
