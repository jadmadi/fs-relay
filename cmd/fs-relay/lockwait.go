package main

import (
	"fs-relay/internal/local"
	"fs-relay/internal/wire"
)

func isLock(method string) bool {
	switch method {
	case wire.Getlk, wire.Setlk, wire.Setlkw:
		return true
	}
	return false
}

// lockInode names the inode a lock call is about.
func lockInode(tree *local.Tree, p wire.Params) uint64 {
	if p.Inode != 0 {
		return p.Inode
	}
	if p.Fh != 0 {
		if ino, ok := tree.InodeOf(p.Fh); ok {
			return ino
		}
	}
	return 0
}
