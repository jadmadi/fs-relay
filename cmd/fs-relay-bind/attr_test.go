package main

import (
	"testing"

	"fs-relay/internal/wire"

	"github.com/hanwen/go-fuse/v2/fuse"
)

func TestSetattrBitsMatchFUSE(t *testing.T) {
	pairs := []struct {
		ours uint32
		fuse uint32
	}{
		{wire.SetMode, fuse.FATTR_MODE},
		{wire.SetUID, fuse.FATTR_UID},
		{wire.SetGID, fuse.FATTR_GID},
		{wire.SetSize, fuse.FATTR_SIZE},
		{wire.SetAtime, fuse.FATTR_ATIME},
		{wire.SetMtime, fuse.FATTR_MTIME},
		{wire.SetFH, fuse.FATTR_FH},
		{wire.SetAtimeNow, fuse.FATTR_ATIME_NOW},
		{wire.SetMtimeNow, fuse.FATTR_MTIME_NOW},
	}
	for _, p := range pairs {
		if p.ours != p.fuse {
			t.Fatalf("%d != %d", p.ours, p.fuse)
		}
	}
}
