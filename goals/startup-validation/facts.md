# Facts: Validate the mountpoint, data directory and namespace

## Verified current state

| Fact | Where |
|------|-------|
| `abs` calls `os.MkdirAll(root/ns)` on every path resolve | `internal/local/path.go:45` |
| A missing namespace is therefore created implicitly | reproduced with a fresh `data/` |
| `rawLimits` swallows the `BlockSize` error and uses 4096 | `cmd/fs-relay/raw.go:51` |
| A `-data` file starts cleanly, then access fails with ENOTDIR | `fs-relay -data notadir` |
| The bind connects and logs a client id before mounting | `cmd/fs-relay-bind/main.go`, `open()` before `fs.Mount` |
| A missing mountpoint fails from fusermount | `fs-relay-bind ... /nope` |
| A non-empty mountpoint is mounted over silently | `fs-relay-bind ... /occupied` |
| The maintainer prefers aborting over warning for #12 | issue #12 comment, 15:08 |
| The maintainer wants `-force-*` for discouraged behaviour | issue #7 comment, 15:15 |

## Required behaviour

### Server

- `fs-relay` creates the `-data` root at startup if it is missing.
- It fails at startup when the path exists and is not a directory. It does not
  test writability; a read-only root fails on the first write.
- It logs the resolved absolute data path beside the listen URL.
- It never creates `data/<namespace>`.
- A `join`, a websocket, or a `/fs/` call for a namespace whose directory is
  missing is refused. The join and websocket get HTTP 404 with a short
  message; the mount stops instead of retrying.
- An existing namespace behaves exactly as today.

### Mount

- `fs-relay-bind` stats the mountpoint before opening any connection.
- A missing mountpoint or a non-directory stops with a direct message.
- A non-empty mountpoint stops with a message that names
  `-force-existing-mnt`, unless that flag is set.
- `-force-existing-mnt` allows the mount and hides the existing entries, as
  today.
- The connection and mount code is unchanged after those checks.

## Error contract

| Condition | Result |
|-----------|--------|
| data root missing | create it, log the resolved path |
| data root is a file | exit 1: `data directory <path> is not a directory` |
| namespace missing on join | HTTP 404: `namespace <ns> does not exist` |
| mountpoint missing | exit 1: `mountpoint <path> does not exist` |
| mountpoint is a file | exit 1: `mountpoint <path> is not a directory` |
| mountpoint non-empty, no flag | exit 1: `mountpoint <path> is not empty; use -force-existing-mnt` |
| mountpoint non-empty with flag | mount succeeds |

## Tests

- `local`: `Prepare` creates a missing root; rejects a file; `OpenNamespace`
  reports true/false; `abs` no longer creates a namespace.
- server: `join` for a missing namespace is 404; an existing namespace
  works.
- bind: `checkMount` for missing, file, empty and non-empty, with and without
  the force flag.
- smoke: two mounts still read and write when the namespace exists.

## Open questions, closed

- Create the data root or the namespace? The root only.
- Cache namespaces? Cache positives; never cache a miss.
- Warn or abort on non-empty? Abort unless the force flag is set.
