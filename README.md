# fs-relay

Share a directory over HTTP and mount it like a local filesystem.

The server runs on the machine that holds the files. Mount it from as many hosts as you need, at the same time. Each mount host needs fuse3 (`fusermount3`).

## What you get

- A normal directory: create, read, write, rename, symlink, and delete.
- One connection for the mount, which is the faster way to run it. `-post` sends each operation as its own request instead.
- Several hosts can mount the same name at once and see the same files.
- SQLite works with its default rollback journal. `PRAGMA journal_mode=WAL` fails, because WAL needs a shared-memory file this filesystem cannot provide.
- Writes reach other mounts when the file is closed or synced.

## How it compares

On each filesystem we unpacked an archive, packed the tree again, copied the archive onto the mount, and listed that copy with `tar -tf`. A local directory is the floor. [kvfs](https://github.com/muayyad-alsadi/kvfs) is our other filesystem. [s3fs](https://github.com/s3fs-fuse/s3fs-fuse) is mounted on [rustfs](https://github.com/rustfs/rustfs). [fuse-nfs](https://github.com/sahlberg/fuse-nfs) is NFS through FUSE.

`ansible.tar` is 27 MiB and 5,868 files.

| | local directory | kvfs | fs-relay | fs-relay `-post` | s3fs on rustfs | fuse-nfs |
|---|---:|---:|---:|---:|---:|---:|
| unpack | 0.14 s | 25.76 s | 15.47 s | 18.11 s | 203.56 s | 107.89 s |
| pack | 0.10 s | 12.58 s | 3.76 s | 4.18 s | 57.53 s | 39.68 s |
| copy the archive | 0.02 s | 0.48 s | 0.12 s | 0.11 s | 0.38 s | 0.45 s |
| list the archive | 0.03 s | 0.82 s | 0.16 s | 0.19 s | 0.16 s | 0.48 s |

The default mount unpacks about thirteen times faster than s3fs (15.47 s against 203.56 s) and about seven times faster than fuse-nfs (15.47 s against 107.89 s). Packing is about fifteen times faster than s3fs (3.76 s against 57.53 s) and about ten and a half times faster than fuse-nfs (3.76 s against 39.68 s).

`Antigravity.tar.gz` is mostly a few large files, about 500 MiB unpacked.

| | local directory | kvfs | fs-relay | fs-relay `-post` | s3fs on rustfs | fuse-nfs |
|---|---:|---:|---:|---:|---:|---:|
| unpack | 5.60 s | 30.71 s | 9.78 s | 9.36 s | 21.01 s | 36.13 s |
| pack | 0.49 s | 19.77 s | 2.38 s | 2.40 s | 5.31 s | 10.37 s |
| copy the archive | 0.13 s | 3.12 s | 0.65 s | 0.79 s | 1.14 s | 2.65 s |
| list the archive | 5.62 s | 7.21 s | 6.13 s | 6.12 s | 6.74 s | 10.30 s |

On this archive the default mount unpacks ahead of s3fs (9.78 s against 21.01 s) and ahead of fuse-nfs (9.78 s against 36.13 s). Packing is 2.38 s on the default mount, 5.31 s on s3fs, and 10.37 s on fuse-nfs.

## Run

```bash
go build -o fs-relay ./cmd/fs-relay
go build -o fs-relay-bind ./cmd/fs-relay-bind

mkdir -p data /mnt/fs-relay
./fs-relay -listen 127.0.0.1:8080 -data data
./fs-relay-bind -url http://127.0.0.1:8080 -ns demo /mnt/fs-relay
```

`-listen` defaults to `127.0.0.1:8080` and `-data` to `data`. Each name under `data` is one independent tree. `-url` defaults to `http://127.0.0.1:8080` and `-ns` to `default`. Add `-post` to send each operation as its own request. `-raw=false` sends large reads and writes on the main connection. `-allow-other` lets other users on the mount host use the mount. `-debug` logs FUSE traffic.

Open the server URL in a browser for a welcome page. `curl` against the same URL returns a short status. Unmount with `fusermount3 -u /mnt/fs-relay`.

On a new mount, create a directory, write and read a small file, make a symlink, rename, and delete. Mount the same name a second time, write on one, and read on the other. Copy a file from one path on the mount to another. Append with `>>`. Write about 10 MiB with `dd bs=10k` and read it back. Repeat that write with `-raw=false` and compare the bytes.

## Earlier designs

This section is for people changing the server. Do not bring these back. They were measured, and they stayed out.

MessagePack was the first wire format. FlatBuffers replaced it. On the Ansible tree, `-ws` with FlatBuffers was 6% to 15% faster than `-ws` with MessagePack: about 15% unpacking and about 6% packing. On the Antigravity archive it was 11% faster at both.

JSON can carry file bytes as `multipart/form-data`: one `application/json` part for the call, and one `application/octet-stream` part for the bytes. On the Ansible tree that plain-text body kept up with MessagePack and FlatBuffers. It is not in this server. One body shape is the smaller program.

The protocol is in [`SPEC.md`](./SPEC.md). What was tried and set aside is also in [`ROADMAP.md`](./ROADMAP.md).
