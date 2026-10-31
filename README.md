# fs-relay

Share a directory over HTTP and mount it like a local filesystem.

The server runs on the machine that holds the files. Mount it from as many hosts as you need, at the same time. Each mount host needs fuse3 (`fusermount3`).

## What you get

- A normal directory: create, read, write, rename, symlink, and delete.
- One connection for the mount, which is the faster way to run it. `-post` sends each operation as its own request instead.
- Several hosts can mount the same name at once and see the same files.
- SQLite works with its default rollback journal. `PRAGMA journal_mode=WAL` fails, because WAL needs a shared-memory file this filesystem cannot provide.

## How it compares

On each filesystem we unpacked an archive, packed the tree again, copied the archive onto the mount, and listed that copy with `tar -tf`. A local directory is the floor. [kvfs](https://github.com/muayyad-alsadi/kvfs) is our other filesystem. [s3fs](https://github.com/s3fs-fuse/s3fs-fuse) is mounted on [rustfs](https://github.com/rustfs/rustfs). [fuse-nfs](https://github.com/sahlberg/fuse-nfs) is NFS through FUSE.

`ansible.tar` is 27 MiB and 5,868 files.

| | local directory | kvfs | fs-relay | fs-relay `-post` | s3fs on rustfs | fuse-nfs |
|---|---:|---:|---:|---:|---:|---:|
| unpack | 0.14 s | 25.76 s | 16.56 s | 20.04 s | 203.56 s | 107.89 s |
| pack | 0.10 s | 12.58 s | 4.82 s | 5.78 s | 57.53 s | 39.68 s |
| copy the archive | 0.02 s | 0.48 s | 0.34 s | 0.26 s | 0.38 s | 0.45 s |
| list the archive | 0.03 s | 0.82 s | 0.26 s | 0.29 s | 0.16 s | 0.48 s |

The default mount unpacks about twelve times faster than s3fs (16.56 s against 203.56 s) and about six and a half times faster than fuse-nfs (16.56 s against 107.89 s). Packing is about twelve times faster than s3fs (4.82 s against 57.53 s) and about eight times faster than fuse-nfs (4.82 s against 39.68 s).

`Antigravity.tar.gz` is mostly a few large files, about 500 MiB unpacked.

| | local directory | kvfs | fs-relay | fs-relay `-post` | s3fs on rustfs | fuse-nfs |
|---|---:|---:|---:|---:|---:|---:|
| unpack | 5.60 s | 30.71 s | 26.19 s | 30.26 s | 21.01 s | 36.13 s |
| pack | 0.49 s | 19.77 s | 4.85 s | 4.19 s | 5.31 s | 10.37 s |
| copy the archive | 0.13 s | 3.12 s | 1.88 s | 1.54 s | 1.14 s | 2.65 s |
| list the archive | 5.62 s | 7.21 s | 7.22 s | 7.30 s | 6.74 s | 10.30 s |

On this archive s3fs unpacks ahead of the default mount (21.01 s against 26.19 s). Packing is close: 4.85 s on the default mount, 5.31 s on s3fs, and 10.37 s on fuse-nfs.

## Run

```bash
go build -o fs-relay ./cmd/fs-relay
go build -o fs-relay-bind ./cmd/fs-relay-bind

mkdir -p data /mnt/fs-relay
./fs-relay -listen 127.0.0.1:8080 -data data
./fs-relay-bind -url http://127.0.0.1:8080 -ns demo /mnt/fs-relay
```

`-listen` defaults to `127.0.0.1:8080` and `-data` to `data`. Each name under `data` is one independent tree. `-url` defaults to `http://127.0.0.1:8080` and `-ns` to `default`. Add `-post` to send each operation as its own request. `-allow-other` lets other users on the mount host use the mount. `-debug` logs FUSE traffic.

Open the server URL in a browser for a welcome page. `curl` against the same URL returns a short status. Unmount with `fusermount3 -u /mnt/fs-relay`.

On a new mount, create a directory, write and read a small file, make a symlink, rename, and delete. Mount the same name a second time, write on one, and read on the other.

## Earlier designs

This section is for people changing the server. Do not bring these back. They were measured, and they stayed out.

MessagePack was the first wire format. FlatBuffers replaced it. On the Ansible tree, `-ws` with FlatBuffers was 6% to 15% faster than `-ws` with MessagePack: about 15% unpacking and about 6% packing. On the Antigravity archive it was 11% faster at both.

JSON can carry file bytes as `multipart/form-data`: one `application/json` part for the call, and one `application/octet-stream` part for the bytes. On the Ansible tree that plain-text body kept up with MessagePack and FlatBuffers. It is not in this server. One body shape is the smaller program.

The protocol is in [`SPEC.md`](./SPEC.md). What was tried and set aside is also in [`ROADMAP.md`](./ROADMAP.md).
