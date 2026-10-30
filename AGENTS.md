# AGENTS.md

Working notes for agents in this repo. `SPEC.md` holds the design; this file holds how the code is written.

`fs-relay` is an HTTP-plus-MsgPack relay of FUSE calls to a local directory, in Go. Five packages, on purpose; do not add a sixth: `cmd/fs-relay`, `cmd/fs-relay-bind`, `internal/wire` (codec), `internal/local` (every `os` call, inode map, handles, locks, error sentinels), `internal/client` (the POST and the websocket).

## Who the words are for

The person reading the welcome page, the README, a flag, or an error is a sysadmin. They are the driver. A driver does not need to be an engineer to drive. Tell them what it is and what to run: a fast network filesystem, mounted with `fs-relay-bind`. Do not tell them how it was written. Encodings, package names, key layouts, and which call answered the request belong in `SPEC.md` and in this file.

## Commands

```bash
gofmt -l .   # must print nothing
go vet ./...
go test ./...
go build -o fs-relay ./cmd/fs-relay
go build -o fs-relay-bind ./cmd/fs-relay-bind
```

## Errors

- Return errors; never log an error and return it too.
- Wrap with context: `fmt.Errorf("mkdir %s: %w", name, err)`.
- Match with `errors.Is`/`errors.As`, never by comparing strings.
- Errno travels as a number. `internal/local` returns `syscall.Errno` or wraps it; one table in `cmd/fs-relay` turns an error into `error.code`, and one in `cmd/fs-relay-bind` turns the code back into an errno.
- No `panic` outside a startup invariant.

## Functions

- Guard clauses and early returns; no `else` after a `return`.
- One level of abstraction per function: it decides, or it does I/O, not both.
- Keep functions under roughly 40 lines and nesting under four levels.
- If a block needs a comment explaining what it does, extract it and use the comment as the name.
- A boolean parameter that flips behavior is two functions wearing one name.
- `switch` over a closed set (the method names), not an if-chain; dispatch through one table of method to handler.

## Types and seams

- Pass the concrete type until a second implementation exists; then declare the smallest interface at the consumer.
- No interface with one implementation and one caller. No `XxxManager`/`XxxService` that forwards calls.
- Dependencies arrive through the constructor. No package-level mutable state, no `init()`.
- `internal/wire` imports neither `net/http`, go-fuse nor `os`.
- Only `internal/local` touches the filesystem, and it resolves every path under `data/<namespace>/` in one function that refuses escape.

## Spaghetti and ravioli

- One entry point, one exit per branch; iterate once and collect, then act.
- Prefer a data table (method handlers, errno mappings) to a chain of special cases.
- Every indirection earns its keep: a second caller, a seam a test cannot get otherwise, or a real boundary (HTTP, FUSE, syscalls).
- If a wrapper's body is a single call, delete the wrapper.

## Concurrency

- `context.Context` is the first parameter and is never stored in a struct.
- Every goroutine has an owner and a shutdown path; `Close` waits for it.
- The inode map and handle table are guarded by one owner each; never hold their lock across file I/O or a network call.
- A lock waiter holds no mutex while it waits on its connection.

## Tests

- Table-driven; `t.Helper()` in helpers, `t.TempDir()` for paths, no `time.Sleep`.
- Assert through the public surface of `internal/local` against a real temp directory. Use `net/http/httptest` for the HTTP edge.
- Anything that needs a real mount is not a unit test; it belongs in the README smoke checklist.

## Dependencies

Standard library first. Expected: `go-fuse`, one msgpack library, and `golang.org/x/sys/unix` for the syscalls the standard library does not wrap. UUIDv7 is a small local function. Ask before adding another.

## Done means

- `gofmt -l .` is empty; `go vet ./...` and `go test ./...` are clean.
- New exported identifiers have doc comments; a new operation has a `local` test and a golden case in `wire`.
- Any benchmark number in a doc says which run produced it.
- A behavior change updates `SPEC.md` and `ROADMAP.md` in the same change.
