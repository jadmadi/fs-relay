package local

import (
	"context"

	"fs-relay/internal/wire"
)

type op func(context.Context, *Tree, wire.Params) (wire.Result, error)

var operations = map[string]op{
	wire.Lookup:        lookup,
	wire.Getattr:       getattr,
	wire.Setattr:       setattr,
	wire.Access:        access,
	wire.Statfs:        statfs,
	wire.Mkdir:         mkdir,
	wire.Create:        create,
	wire.Open:          open,
	wire.Read:          read,
	wire.Reads:         reads,
	wire.Write:         write,
	wire.Flush:         flush,
	wire.Fsync:         fsync,
	wire.Release:       release,
	wire.Opendir:       opendir,
	wire.Readdir:       readdir,
	wire.Releasedir:    release,
	wire.Unlink:        unlink,
	wire.Rmdir:         rmdir,
	wire.Rename:        rename,
	wire.Symlink:       symlink,
	wire.Readlink:      readlink,
	wire.CopyFileRange: copyRange,
}
