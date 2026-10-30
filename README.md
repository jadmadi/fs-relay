# fs-relay

A fast network filesystem. Run the server on the host that holds the files, and mount it with `fs-relay-bind`.

Two binaries:

- `fs-relay`: the server.
- `fs-relay-bind`: the mount.

The design is in [`SPEC.md`](./SPEC.md). Progress, and what was tried and dismissed, is in [`ROADMAP.md`](./ROADMAP.md). Code conventions are in [`AGENTS.md`](./AGENTS.md).

Requires `fusermount3` (fuse3) for the mount.

## Build

```bash
go build -o fs-relay ./cmd/fs-relay
go build -o fs-relay-bind ./cmd/fs-relay-bind
```

## Run

```bash
mkdir -p data /mnt/fs-relay
./fs-relay -listen 127.0.0.1:8080 -data data
./fs-relay-bind -url http://127.0.0.1:8080 -ns demo /mnt/fs-relay
```

`-listen` defaults to `127.0.0.1:8080`, `-data` to `data`, `-url` to `http://127.0.0.1:8080`, `-ns` to `default`. `-ws` makes the mount use one websocket at `/ws/<namespace>` instead of one POST per call. `-allow-other` and `-debug` are mount flags.

`curl http://127.0.0.1:8080/` returns `status` `ok`, `time_ms` as epoch milliseconds, and `time` as UTC `2026-10-30T20:18:00Z`. A browser gets the welcome page.

## Smoke

On one mount: `mkdir`, write and read a small file, `ln -s`, `mv`, `rm`, `rmdir`. On two mounts of one namespace: write a file on the first and read it on the second. Then `fusermount3 -u` each mount. This checklist is not a unit test.

## Benchmark

We packed a git clone of Ansible into `ansible.tar` (27 MiB, 5,868 files). On each filesystem we unpacked that archive, packed the tree again, copied the archive onto the mount, and listed it with `tar -tf` without unpacking. The fs-relay times are from 2026-10-31 01:42 +03. The other columns are earlier runs of the same archive.

| | local directory | kvfs | fs-relay POST | fs-relay `-ws` | s3fs on rustfs | fuse-nfs |
|---|---:|---:|---:|---:|---:|---:|
| extract | 0.14 s | 23.64 s | 23.38 s | 19.48 s | 211.10 s | 108.04 s |
| archive | 0.10 s | 14.26 s | 6.26 s | 5.49 s | 58.61 s | 48.12 s |
| copy the tar | 0.02 s | 0.48 s | 0.22 s | 0.37 s | | 0.40 s |
| `tar -tf` | 0.03 s | 0.19 s | 0.29 s | 0.27 s | | 0.54 s |

The websocket mount is about ten times faster than s3fs on rustfs, on both extract and archive, and about ten times faster than fuse-nfs on archive (5.49 s against 48.12 s). The fuse-nfs extract is about five times slower. kvfs is ours as well, so it is left out of that comparison.

Local and kvfs were measured 2026-10-30 22:49 +03. s3fs is the 2026-10-30 18:46 +03 run, extract and archive only. fuse-nfs is the 2026-10-31 01:31 +03 run. kvfs is the Go filesystem at namespace level against Valkey 9.0.6, default 1s timeouts. fs-relay uses the same attribute and entry timeout, a 500ms negative timeout, and keeps a read-only file in the page cache. A writable open stays direct I/O. After `readdir`, the mount fetches the regular files in that directory that are at most 64 KiB, up to 32 names and 1 MiB per `reads` call, and serves those bytes from memory until an eviction. A larger file stays a normal `read`. s3fs 1.97 mounts rustfs 1.0.4 at `127.0.0.1:9000` with path-style requests. `diff -rq` of each fs-relay extract against the local tree was empty. s3fs exited 2: `tar` reported `Cannot close: Permission denied` on 29 paths, and the archive has 8,609 members. fuse-nfs mounts unfsd 0.11.0, NFS v3 over TCP on port 12049, with `-p 0` and an 8 KiB read and readahead cap. The server owns new directories, so the kernel permission check is off. That read cap is the size this server answers; the fuse-nfs archive has 8,638 members.

On the 01:42 run both fs-relay archives are about three times faster than the extract on that transport. Copying and listing the 27 MiB tar stay under half a second; those two do not use the small-file prefetch. MessagePack on either transport is the binary encoding only, with no compression.

## SQLite

A database on one mount works with the rollback journal, which is the default (`PRAGMA journal_mode=DELETE`). `PRAGMA journal_mode=WAL` fails with `disk I/O error`. WAL keeps a shared-memory file beside the database and maps it into every connection. This mount does not provide that mapping.
