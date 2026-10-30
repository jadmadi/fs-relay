# Roadmap

## Done

- Design documents: `SPEC.md`, `AGENTS.md`, this file.
- M1 code and a real mount: mkdir, write, symlink, rename, read, unlink.
- M2: extract and re-archive `ansible.tar` through a mount. The tree matched the local extract.
- The same tar workload against a local directory, kvfs, fs-relay over POST and over the websocket, and s3fs on rustfs. The README table carries the 2026-10-30 22:49 +03 local and kvfs numbers, including copying the tar onto the mount and `tar -tf`. The fs-relay columns were retimed 2026-10-31 01:42 +03. s3fs is the 18:46 run, extract and archive only, and did not match: 29 paths failed to close. fuse-nfs was retimed 2026-10-31 01:31 +03 after `share` was labeled `container_file_t`.
- M3 on one mount, rollback journal (SQLite 3.51.2, 2026-10-30): one connection inserts and `integrity_check` is `ok`. Four writers with `busy_timeout` committed 200 rows, four distinct values, `integrity_check` `ok`. The kernel's own locks on that mount did the waiting. The relay lock table is not built. WAL mode fails with `disk I/O error`. Retested the same evening after `reads`: 202 rows, `integrity_check` `ok`, including a read after listing the directory. WAL still fails with `disk I/O error`.
- A multiplexed `/ws/<namespace>` socket. The mount selects it with `-ws`. Replies carry the call id.
- Kernel cache with the same timeouts and open flags as kvfs. A POST mount joins once and reads JSON events on that chunked response. A websocket mount reads the same events on its call socket. A read-only pass reuses the attributes from `readdir` and reads by inode. Small regular files in a listing are fetched together by `reads` and served from memory until an eviction. The fs-relay columns in the README are the 2026-10-31 01:42 +03 run of this build.

## Yet to be done

1. The relay lock table, so two mounts exclude each other. One mount does not need it. A release already travels on the mount's pubsub, and a rejoin repeats one the client has not seen.
2. Benchmarks against SMB or NFS, with and without FUSE.

## Tried and dismissed

Design choices. None of these was implemented and then removed.

- JSON bodies: cannot carry binary file bytes.
- A different HTTP status per errno: the errno belongs in `error.code`; statuses stay 200, 400, 500.
- Host `st_ino` as the protocol inode: the namespace directory is not inode 1, and a copy would change it.
- A durable inode map: a restart is a remount.
- A path-only protocol: the wire mirrors FUSE inodes so relaying stays thin.
- JSON-RPC ids on the POST body: the HTTP exchange already correlates. The websocket carries its own id because many calls share the socket.
- MsgPack frames on the lock stream: a partial frame cannot resync; lines can.
- kvfs shard levels: there is one directory tree.
- Blocking `setlkw` inside one plain request: a dropped connection could not rejoin the wait.
- WAL for M3: it needs shared memory between processes, which the mount does not provide.
- A generic `batch` method, including one with `$id.field` references. The mount already knows the inodes a listing will read, and one batch response would wait on its slowest call. `reads` is the narrow form that matches that listing.
- Hardlinks, xattrs, ACLs, devices, FUSE 2, Windows, macOS, auth, TLS.
