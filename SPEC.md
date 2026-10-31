# fs-relay — design contract

`README.md` says what fs-relay is. This file is the contract: how the wire, the inodes and the locks behave. `ROADMAP.md` tracks progress. `AGENTS.md` says how the code is written.

## Problem

A directory on one host should be usable as a filesystem on another, over plain HTTP, without a kernel network-filesystem stack. The wire must carry binary file bytes, and each call must map onto a FUSE method with minimal translation, so that relaying stays cheap to write and cheap to reason about.

## Shape

Two processes:

- `fs-relay` is an HTTP server. It applies one filesystem call to the directory `data/<namespace>/`.
- `fs-relay-bind` is a FUSE3 mount (`github.com/hanwen/go-fuse/v2`, as in kvfs). It is one of the two clients below.

The path segment after `/fs/` is the namespace: `[A-Za-z0-9_-]+`, non-empty, at most 64 bytes. It selects the directory under `data/` and nothing else. There is no shard level, because there is one directory tree. Anything outside the grammar is refused before any file is touched.

## Clients

A mount is one of two clients. Both speak the same envelope. They differ in how calls and events travel. The mount defaults to the websocket. `-post` selects the other. `-ws` is the same as the default. Passing both flags is an error.

**Websocket.** One `GET /ws/<namespace>` stays open. The socket is bidirectional. Every call and every event is one binary message on it. There is no second stream, and there is no `Transfer-Encoding` header. The websocket frame is the framing, so an event has no length prefix.

```mermaid
sequenceDiagram
    participant Mount
    participant Server
    Mount->>Server: GET /ws/namespace
    Note over Mount,Server: one socket, both directions
    Mount->>Server: call
    Server-->>Mount: result for that call
    Server-->>Mount: event
```

**POST.** `-post` uses two directions on purpose.

- Client to server: each call is its own `POST /fs/<namespace>`. The response to that POST is the result of that call, one envelope, with a `Content-Length`. A POST has no request id. The HTTP exchange is the correlation.
- Server to client: at start the client sends `POST /fs/<namespace>` with method `join`. This is not a filesystem call. The response stays open for the life of the mount. Its headers are `Content-Type: application/x-flatbuffers` and `Transfer-Encoding: chunked`. The server does not write that transfer header itself: it sets no `Content-Length` and flushes after every event, and the HTTP server adds `Transfer-Encoding: chunked`. The body is one way, server to client. Each event is one chunk holding a little-endian uint32 length and then one envelope, not a text line. A length above the body limit closes the stream. The first event carries the client uuid. Later events are evictions or a lock release.

Calls never travel on the chunked response, and events never travel as the response of a filesystem POST.

```mermaid
sequenceDiagram
    participant Mount
    participant Server
    Mount->>Server: POST /fs/namespace join
    Note over Server,Mount: response stays open, Transfer-Encoding chunked
    Server-->>Mount: chunk, client uuid
    Mount->>Server: POST /fs/namespace call
    Server-->>Mount: result of that POST
    Server-->>Mount: chunk, event
```

## Envelope

FlatBuffers is the only filesystem wire. The content type is `application/x-flatbuffers`. The file identifier is `FSR1`. The schema is `internal/wire/fsrelay.fbs`. One buffer is one envelope.

- `POST /fs/<namespace>`. The body is one buffer.
- `params.namespace` must equal the URL segment.
- Success: HTTP 200 and a result.
- A filesystem failure: HTTP 200 and an error whose `code` is a Linux errno.
- HTTP 400: bad body, unknown method, namespace mismatch, params that do not match the method.
- HTTP 500: a fault that is not an errno.
- No request id on a POST. The HTTP exchange is the correlation.
- File bytes are a byte vector. One FUSE write is one call, at most 1 MiB, which is also the mount's maximum write.

`GET /ws/<namespace>` is the same envelope, one binary websocket message per call. The client sets `id` (a nonzero integer) on the request and the server copies it onto every message for that call. Calls overlap. A filesystem failure carries that `id` and the error. A fault that is not an errno carries that `id` and a fault string, and the socket stays up. A body that is not a call, an `id` of 0, a repeated `id`, or a namespace that does not match the URL closes the socket.

`setlkw` is an ordinary call. It does not hold a response open. The grant and the release arrive on the mount's one pubsub, described under Locks.

`namespace` is required and must match the URL. The other fields are the kernel arguments the method needs: `inode`, `inodes`, `name`, `new_inode`, `new_name`, `fh`, `mode`, `flags`, `offset`, `size`, `data`, `target`, `valid`, `atime`, `mtime`, `uid`, `gid`, `mask`. `inodes` is the list for `reads`. `valid` uses the Linux FUSE `FATTR_*` bits. Empty strings and empty byte vectors are absent. A numeric zero is present. `valid` still says which setattr fields count, so a size or owner of zero is applied. A missing required field is HTTP 400. A name of `.`, `..`, or one that contains `/` or NUL is `EINVAL`.

Under `FSR1`, field identity is declaration order. A table only grows by a field appended at the end. Reordering, inserting, deleting, or changing a type needs a new file identifier.

## Inodes and handles

- Root is inode `1` in every namespace.
- The server keeps a process-local map between paths and inodes. The host `st_ino` is not the protocol inode: the namespace directory is not inode 1, and a copy would change it.
- Any other inode is assigned on first `lookup`, `readdir`, `create`, `mkdir` or `symlink` and returned in the result. `readdir` assigns one so a directory entry never carries inode 0. It survives a rename inside the process.
- `create` and a writable `open` return an `fh`. `write`, `fsync`, `release`, `lseek` and the lock calls name that `fh`. A read-only `read` names the inode instead.
- The map and the open files die with the process. A transport error or a restart means the client must remount.
- `..` and absolute paths never leave `data/<namespace>/`. A name containing `/` or equal to `.` or `..` is refused with `EINVAL`.

## Methods

Names are the go-fuse methods, lowercased. Arguments are the kernel's, plus `namespace` and, where a call is about a name, the parent `inode`. The signatures mirror FUSE so that a relay is a thin translation.

M1 surface, which is also what `tar` needs: `lookup`, `getattr`, `setattr`, `access`, `statfs`, `mkdir`, `create`, `open`, `read`, `reads`, `write`, `flush`, `fsync`, `release`, `opendir`, `readdir`, `releasedir`, `unlink`, `rmdir`, `rename`, `symlink`, `readlink`. `reads` returns the whole body of each named inode that is a regular file of at most 64 KiB, at most 32 inodes and 1 MiB of bytes. A larger file, an empty file, or a missing inode is omitted from that result.

`rename` follows `rename(2)`: a file replaces a file, a directory replaces an empty directory, and the other pairings fail with `EISDIR`, `ENOTDIR` or `ENOTEMPTY`. `RENAME_NOREPLACE` is honored. `RENAME_EXCHANGE` and `RENAME_WHITEOUT` fail with `EINVAL`.

## Coherence

Attribute and entry timeouts are 1s. The negative timeout is 500ms. A read-only open keeps the kernel page cache (`FOPEN_KEEP_CACHE`). A create, and any writable open, bypasses it (`FOPEN_DIRECT_IO`).

A read-only open does not take a server file handle. `read` names the inode, and that open's `flush` and `release` stay on the mount. `flush` does not sync; `fsync` does. `readdir` carries each name's attributes. The mount answers the lookup and getattr that follow from those, until an eviction drops them. Opening a directory does not take a server handle either.

After that listing, the mount calls `reads` for the regular files in it that are at most 64 KiB. One call carries at most 32 names and 1 MiB of bytes. A larger file stays a normal `read`. The mount keeps those bodies until an eviction for that name, and stops storing them once 64 MiB is held. A read the cache cannot answer uses `read`.

A mount is one subscriber for its namespace. There is no per-file socket and no per-lock socket.

- If the chunked `join` response drops, the POST client joins again at once with the same uuid. If that join drops too, it waits 100ms and tries again with the same uuid. The server treats the uuid as the same mount and repeats any lock release still owed to it. A uuid the server has never seen is a new mount. Inodes still die with the server process.
- A websocket client does not call `join`. The same events are envelopes on the call socket, with no call id. The websocket frame is the framing, so those events have no length prefix. The connection is the client. A dropped socket abandons that mount's waits.

The server sends an eviction after a successful `create`, `mkdir`, `symlink`, `write`, `setattr`, `unlink`, or `rmdir`, and after a rename it sends the old name and then the new name. A refused call sends nothing. The mount reads those events on a background loop, never on a FUSE handler, and calls `NotifyEntry` and `NotifyContent`. A missed eviction leaves cached file bytes stale until the kernel drops the pages. Names and attributes recover within the 1s timeout.

## Locks

Specified now, implemented for M3.

- POSIX record locks: `getlk`, `setlk`, `setlkw`. The owner is the FUSE owner plus a per-mount client id, so two mounts never share an owner number.
- The server's lock table is the authority. `setlk` is an ordinary envelope and returns `EAGAIN` on a conflict.
- `setlkw` is an ordinary call. The FUSE handler waits on the pubsub reader. It does not hold its own response.
- A release is an envelope on that pubsub whose release field is the lock id, sent to every subscriber of the namespace. On `/ws/` it is one binary message with no call id. The waiting mount wakes and acquires. If two waiters wake together, one `setlk` wins and the other waits again.
- A dropped `join` is the rejoin above. The server repeats a release that this client has not seen, so a missed event cannot leave `setlkw` blocked.
- The server is the only writer under `data/`. A process that opens those files directly is outside the lock table.

This is what SQLite's rollback journal needs (SHARED, RESERVED and EXCLUSIVE byte ranges). WAL needs shared memory between processes and is not a goal.

## Packages

- `cmd/fs-relay`: flags, HTTP decode and encode, calls `internal/local`.
- `cmd/fs-relay-bind`: flags, FUSE nodes and the errno mapping, calls `internal/client`.
- `internal/wire`: FlatBuffers types, the schema, and the generated file. No HTTP, no FUSE, no `os`.
- `internal/local`: the directory, the inode map, handles, locks. Owns every `os` call and the error sentinels.
- `internal/client`: the POST, the websocket, and the `join` stream.

## Testing

- `internal/local` against `t.TempDir()`: every M1 operation, path escape, errno, inode stability across rename.
- `internal/wire`: a round trip of one request and both response shapes. The buffer carries the `FSR1` file identifier.
- Lock grant, conflict, release fan-out and rejoin are tested with the lock table.
- A real mount is not a unit test. It belongs in the README smoke checklist.
- Cost: one POST per FUSE call, or one websocket frame per FUSE call on a shared socket, except `reads`, which carries up to 32 small files from one listing. Each POST mount also holds one `join` stream. `setlkw` does not hold a stream of its own.

## Benchmarks

Against the baseline filesystem, the Go kvfs, and SMB or NFS with and without FUSE. Numbers say which run produced them.

## Out of scope

Hardlinks (`ansible.tar` has none), extended attributes, ACLs, device nodes, FUSE 2, Windows, macOS, authentication, TLS, a durable inode map, WAL.

## Vocabulary

- **namespace**: the tenant directory name under `data/`.
- **inode**: the protocol's number for a path, assigned by the server, root is `1`.
- **fh**: the server's number for an open file.
- **method**: the lowercased FUSE operation named in the envelope.
- **relay**: applying one request to the local tree.
