# Roadmap

## Done

- Design documents: `SPEC.md`, `AGENTS.md`, this file.
- M1 code and a real mount: mkdir, write, symlink, rename, read, unlink.
- M2: extract and re-archive `ansible.tar` through a mount. The tree matched the local extract.
- The same tar workload against a local directory, kvfs, fs-relay (websocket default and `-post`), s3fs on rustfs, and fuse-nfs. Retimed every column on `ansible.tar` and `Antigravity.tar.gz` from 2026-10-31 15:04 +03 to 15:22 +03, production binaries (`-trimpath -ldflags='-s -w'`). Those numbers are the README tables, except the Antigravity fs-relay columns. Those two columns were retimed on 2026-10-31 18:24 +03 to 18:25 +03, same production flags, after writes are sent in 1 MiB chunks and large reads and writes use the raw connection. Unpack 9.78 s and 9.36 s, pack 2.38 s and 2.40 s, copy 0.65 s and 0.79 s, list 6.13 s and 6.12 s, default mount then `-post`, all four steps exited 0. The other Antigravity columns are still the 15:04 +03 run. The Ansible fs-relay columns were retimed on 2026-10-31 23:07 +03 to 23:08 +03, same flags: unpack 17.72 s and 20.47 s, pack 5.64 s and 6.09 s, copy 0.11 s and 0.12 s, list 0.22 s and 0.24 s, default mount then `-post`, all four steps exited 0. The other Ansible columns are still the 15:04 +03 run. On `ansible.tar`, s3fs unpack and pack exited 2 because symlinks failed to close (8609 members packed); copy and list exited 0. Antigravity s3fs exited 0 on all four steps. fuse-nfs used `-p 0 -R 8192 -H 8192` against the running unfsd.
- M3 on one mount, rollback journal (SQLite 3.51.2, 2026-10-30): one connection inserts and `integrity_check` is `ok`. Four writers with `busy_timeout` committed 200 rows, four distinct values, `integrity_check` `ok`. The kernel's own locks on that mount did the waiting. The relay lock table is not built. WAL mode fails with `disk I/O error`. Retested the same evening after `reads`: 202 rows, `integrity_check` `ok`, including a read after listing the directory. WAL still fails with `disk I/O error`.
- A multiplexed `/ws/<namespace>` socket. The mount uses it by default. `-post` selects one POST per call. Replies on the socket carry the call id.
- Kernel cache with the same timeouts and open flags as kvfs. A POST mount joins once and reads length-prefixed FlatBuffers frames on that chunked response. A websocket mount reads the same events on its call socket. A read-only pass reuses the attributes from `readdir` and reads by inode. Small regular files in a listing are fetched together by `reads` and served from memory until an eviction. The non-fs-relay columns in both README tables are the 2026-10-31 15:04 +03 run recorded above.

- `copyfilerange` relays `copy_file_range(2)`. The server opens host files without `O_APPEND`, so an append offset arrives with the write.
- The mount holds contiguous writes and sends them at 1 MiB, at a gap, or on flush, fsync, release, read, setattr, or `copyfilerange`.
- Large reads and writes travel on `GET /raw/<namespace>`, upgraded to a fixed-header stream the server splices. The size window is `32 × st_blksize` of the data directory, clamped, and the mount flag `-raw` defaults on.

## Yet to be done

1. The relay lock table, so two mounts exclude each other. One mount does not need it. A release already travels on the mount's pubsub, and a rejoin repeats one the client has not seen.
2. Benchmarks against SMB or NFS, with and without FUSE.

## Tried and dismissed

MessagePack was the wire, and it was replaced by FlatBuffers. A JSON body that carried file bytes as multipart was implemented on a side branch and left out. The README records both trials. The rest of this list was not built.

- JSON as the call body, including `multipart/form-data` with an `application/json` part and an `application/octet-stream` part. It can carry file bytes. It stayed out so the server has one encoding.
- A different HTTP status per errno: the errno belongs in `error.code`; statuses stay 200, 400, 500.
- Host `st_ino` as the protocol inode: the namespace directory is not inode 1, and a copy would change it.
- A durable inode map: a restart is a remount.
- A path-only protocol: the wire mirrors FUSE inodes so relaying stays thin.
- JSON-RPC ids on the POST body: the HTTP exchange already correlates. The websocket carries its own id because many calls share the socket.
- MessagePack as the wire. FlatBuffers is faster on the same mounts and is the only encoding now.
- A line-oriented join stream. The join response is length-prefixed FlatBuffers frames. A short read ends the stream, and the mount rejoins.
- kvfs shard levels: there is one directory tree.
- Blocking `setlkw` inside one plain request: a dropped connection could not rejoin the wait.
- WAL for M3: it needs shared memory between processes, which the mount does not provide.
- A generic `batch` method, including one with `$id.field` references. The mount already knows the inodes a listing will read, and one batch response would wait on its slowest call. `reads` is the narrow form that matches that listing.
- Hardlinks, xattrs, ACLs, devices, FUSE 2, Windows, macOS, auth, TLS.
- `tee`: nothing fans file bytes out to more than one reader.
- `vmsplice`: the Go garbage collector may reuse gifted pages, and the FUSE device copies them anyway.
- `MSG_ZEROCOPY`: go-fuse reuses its request buffer when the handler returns.
- Splice of a websocket write: the client must mask every frame it sends, and a splice cannot XOR.
- Splice out of a chunked HTTP body: the chunk parser has already copied the bytes into a buffer.
