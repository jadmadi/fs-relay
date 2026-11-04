# Plan: Validate the mountpoint, data directory and namespace

Read `facts.md` first. Work on a branch from `upstream/main`. The PR carries
code, tests and docs only.

## Execution Steps

- [x] **T1 - Prepare the data root in local**
  - Goal: create the root once, reject a non-directory, never create a
    namespace.
  - Files: `internal/local/tree.go`, `internal/local/path.go`,
    `internal/local/local_test.go`.
  - Requirements:
    1. Add `func (t *Tree) Prepare() error`: `os.Stat` first, so a file path
       returns `ENOTDIR` directly, then `os.MkdirAll(root, 0o755)`.
    2. Remove `os.MkdirAll(root/ns)` from `abs`.
    3. Add `func (t *Tree) OpenNamespace(ns string) error`: `CheckNS`, stat
       `root/ns`, require a directory, cache a hit, never cache a miss.
    4. `Call` calls `OpenNamespace` before dispatch.
  - Acceptance:
    - AC-1.1: `Prepare` creates a missing root.
    - AC-1.2: `Prepare` rejects a regular file.
    - AC-1.3: `Call` on a missing namespace returns `ENOENT`.
    - AC-1.4: `OpenNamespace` succeeds after the directory is created later.

- [x] **T2 - Server startup and missing namespace**
  - Goal: fail early on a bad data root and refuse a missing namespace.
  - Files: `cmd/fs-relay/main.go`, `serve.go`, `join.go`, `ws.go`,
    tests.
  - Requirements:
    1. Call `tree.Prepare()` before listening.
    2. Log the resolved absolute data path beside the listen URL.
    3. `Call` uses `OpenNamespace`. Join and websocket admission use an
       uncached `CheckNamespace`; on `ENOENT` they answer HTTP 404
       `namespace <ns> does not exist`.
    4. Do not change the token or envelope paths.
  - Acceptance:
    - AC-2.1: `fs-relay -data <file>` exits 1 before listening.
    - AC-2.2: a missing `-data` is created and the resolved path is logged.
    - AC-2.3: `join` for a missing namespace is 404.
    - AC-2.4: an existing namespace still mounts.

- [x] **T3 - Mountpoint checks in bind**
  - Goal: check the mountpoint before dialing, and refuse a non-empty one
    unless forced.
  - Files: `cmd/fs-relay-bind/main.go`, a small `mountpoint.go`, tests.
  - Requirements:
    1. Add `-force-existing-mnt`.
    2. Before `open`, stat the mountpoint; missing or not a directory stops.
    3. Read the directory; a non-empty result stops unless the flag is set.
    4. Error text names the path and, for non-empty, the flag.
  - Acceptance:
    - AC-3.1: missing mountpoint exits 1 before a client id is logged.
    - AC-3.2: a file mountpoint exits 1.
    - AC-3.3: a non-empty mountpoint exits 1 and names
      `-force-existing-mnt`.
    - AC-3.4: the flag allows the mount.
    - AC-3.5: an empty mountpoint still mounts.

- [x] **T4 - Tests and smoke**
  - Files: `internal/local/local_test.go`,
    `cmd/fs-relay/serve_test.go` or `join_test.go`,
    `cmd/fs-relay-bind/mountpoint_test.go`.
  - Requirements:
    1. Unit tests for `Prepare`, `OpenNamespace`, and `mountpoint`.
    2. Server test for a 404 join.
    3. A two-mount smoke run: create `data/demo` first, mount, read, write.
  - Acceptance:
    - AC-4.1: `go test ./...` and `go test -race ./...` pass.
    - AC-4.2: the smoke run exits 0 and the file round-trips.

- [x] **T5 - Docs and evidence**
  - Files: `README.md`, `SPEC.md`, `goals/startup-validation/evidence.md`.
  - Requirements:
    1. README says the server creates `data` but not `data/<namespace>`,
       and the mount checks the mountpoint.
    2. SPEC says a missing namespace is refused.
    3. Evidence records commands, results and `Verdict: PASS`.
  - Acceptance:
    - AC-5.1: `gofmt -l .` empty, vet and tests pass.
    - AC-5.2: PR opened with the goal attribution.

## Order

T1 before T2. T3 independent but after T1 for the smoke. T4 after T1 to T3.
T5 last.

- [x] **T6 - Close the Qodo startup findings**
  - Goal: fix the five review findings on PR #20.
  - Files: `internal/local/tree.go`, `cmd/fs-relay/serve.go`, `ws.go`,
    `raw.go`, `cmd/fs-relay-bind/cli.go`, `internal/client/client.go`,
    `join.go`, `ws.go`, tests.
  - Requirements:
    1. `misplacedFlag` returns empty for fewer than two args.
    2. Connections use an uncached `CheckNamespace`; calls keep the cache.
    3. A 404 join is terminal for the POST client.
    4. The websocket error carries the server's 404 body.
    5. `Prepare` uses the absolute root in every error.
  - Acceptance:
    - AC-6.1: `fs-relay-bind` with no args exits 2.
    - AC-6.2: a deleted namespace is refused.
    - AC-6.3: `-post` with a missing namespace stops.
    - AC-6.4: the websocket error says the namespace does not exist.
    - AC-6.5: a data-root error names the absolute path.

- [x] **T7 - Close the two low findings**
  - Goal: name a namespace that is a file, and make the data-root message
    reachable.
  - Files: `internal/local/tree.go`, `cmd/fs-relay/serve.go`, tests.
  - Requirements:
    1. `Prepare` stats before `MkdirAll`, so a file yields
       `data directory <path> is not a directory`.
    2. `writeNamespaceErr` maps `ENOTDIR` to 404 with a message, permission
       to 403, and includes the error on 500.
  - Acceptance:
    - AC-7.1: a file root prints the not-a-directory message.
    - AC-7.2: a namespace that is a file is 404 `is not a directory`.

## Definition of done

```
gofmt -l .
go vet ./...
go test ./...
go test -race ./...
```
