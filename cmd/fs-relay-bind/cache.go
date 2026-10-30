package main

import (
	"context"
	"sync"

	"fs-relay/internal/wire"

	"github.com/hanwen/go-fuse/v2/fs"
)

const maxHeld = 64 << 20

// kernel remembers protocol inodes so an eviction can drop the kernel cache.
// The loop is the only caller of NotifyEntry and NotifyContent.
// body holds small files prefetched from a listing, dropped with the name.
type kernel struct {
	mu   sync.Mutex
	byID map[uint64]*fs.Inode
	body map[uint64][]byte
	who  map[uint64]map[string]uint64
	held int
}

func newKernel() *kernel {
	return &kernel{byID: map[uint64]*fs.Inode{}}
}

func (k *kernel) track(id uint64, ino *fs.Inode) {
	k.mu.Lock()
	k.byID[id] = ino
	k.mu.Unlock()
}

func (k *kernel) untrack(id uint64) {
	k.mu.Lock()
	delete(k.byID, id)
	k.mu.Unlock()
}

func (k *kernel) loop(ctx context.Context, events <-chan wire.Change) {
	for {
		select {
		case <-ctx.Done():
			return
		case ch, ok := <-events:
			if !ok {
				return
			}
			k.drop(ch)
		}
	}
}

func (k *kernel) rememberBlob(parent uint64, name string, ino uint64, data []byte) {
	if ino == 0 || len(data) == 0 {
		return
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	k.ensure()
	old := len(k.body[ino])
	if k.held-old+len(data) > maxHeld {
		return
	}
	k.held = k.held - old + len(data)
	k.body[ino] = append([]byte(nil), data...)
	if k.who[parent] == nil {
		k.who[parent] = map[string]uint64{}
	}
	k.who[parent][name] = ino
}

func (k *kernel) blob(ino uint64, off int64, n int) ([]byte, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	src, ok := k.body[ino]
	if !ok || off < 0 || off > int64(len(src)) {
		return nil, false
	}
	end := off + int64(n)
	if end > int64(len(src)) {
		end = int64(len(src))
	}
	out := make([]byte, end-off)
	copy(out, src[off:end])
	return out, true
}

func (k *kernel) forgetName(parent uint64, name string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	ino := k.who[parent][name]
	if ino == 0 {
		return
	}
	delete(k.who[parent], name)
	k.dropBody(ino)
}

func (k *kernel) forgetIno(ino uint64) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.dropBody(ino)
}

func (k *kernel) ensure() {
	if k.body == nil {
		k.body = map[uint64][]byte{}
		k.who = map[uint64]map[string]uint64{}
	}
}

func (k *kernel) dropBody(ino uint64) {
	k.held -= len(k.body[ino])
	delete(k.body, ino)
}

func (k *kernel) drop(ch wire.Change) {
	k.forgetName(ch.Parent, ch.Name)
	k.mu.Lock()
	parent := k.byID[ch.Parent]
	k.mu.Unlock()
	if parent == nil {
		return
	}
	parent.NotifyEntry(ch.Name)
	if child := parent.GetChild(ch.Name); child != nil {
		child.NotifyContent(0, 0)
		if n, ok := child.Operations().(*node); ok {
			n.forgetAttr()
		}
	}
}
