package main

import (
	"context"
	"syscall"

	"fs-relay/internal/wire"
)

func (d *dirHandle) prefetch(ctx context.Context, entries []wire.Dirent) {
	if d.parent == nil || d.parent.k == nil {
		return
	}
	var batch []wire.Dirent
	total := 0
	for _, e := range entries {
		if !smallFile(e) {
			continue
		}
		if len(batch) == wire.MaxReads || total+int(e.Size) > wire.MaxWrite {
			d.fetch(ctx, batch)
			batch = nil
			total = 0
		}
		batch = append(batch, e)
		total += int(e.Size)
	}
	d.fetch(ctx, batch)
}

func smallFile(e wire.Dirent) bool {
	if e.Mode&syscall.S_IFMT != syscall.S_IFREG {
		return false
	}
	return e.Size > 0 && e.Size <= uint64(wire.MaxPrefetch)
}

func (d *dirHandle) fetch(ctx context.Context, batch []wire.Dirent) {
	if len(batch) == 0 {
		return
	}
	inos := make([]uint64, len(batch))
	name := make(map[uint64]string, len(batch))
	for i, e := range batch {
		inos[i] = e.Ino
		name[e.Ino] = e.Name
	}
	res, err := d.c.Call(ctx, wire.Reads, wire.Params{Inodes: inos})
	if err != nil {
		return
	}
	for _, b := range res.Blobs {
		d.parent.k.rememberBlob(d.ino, name[b.Inode], b.Inode, b.Data)
	}
}
