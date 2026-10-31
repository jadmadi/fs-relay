package main

import (
	"context"
	"sync"

	"fs-relay/internal/wire"

	"github.com/hanwen/go-fuse/v2/fs"
)

const maxHeld = 64 << 20

// maxDirs caps directory listings kept from readdirs.
const maxDirs = 8192

type ownKey struct {
	parent uint64
	name   string
}

// kernel remembers protocol inodes so an eviction can drop the kernel cache.
// The loop is the only caller of NotifyEntry and NotifyContent.
// body holds small files prefetched from a listing, dropped with the name.
// dirs holds child directory listings. ownN counts names this mount just
// changed, so the matching eviction does not drop that name here.
type kernel struct {
	mu   sync.Mutex
	byID map[uint64]*fs.Inode
	body map[uint64][]byte
	who  map[uint64]map[string]uint64
	dirs map[uint64][]wire.Dirent
	ownN map[ownKey]int
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

func (k *kernel) own(parent uint64, name string) {
	if k == nil || parent == 0 || name == "" {
		return
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.ownN == nil {
		k.ownN = map[ownKey]int{}
	}
	k.ownN[ownKey{parent, name}]++
}

func (k *kernel) disown(parent uint64, name string) {
	if k == nil || parent == 0 || name == "" {
		return
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	k.dropOwn(ownKey{parent, name})
}

func (k *kernel) take(parent uint64, name string) bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	key := ownKey{parent, name}
	if k.ownN[key] == 0 {
		return false
	}
	k.dropOwn(key)
	return true
}

func (k *kernel) dropOwn(key ownKey) {
	n := k.ownN[key]
	if n <= 1 {
		delete(k.ownN, key)
		return
	}
	k.ownN[key] = n - 1
}

func (k *kernel) rememberDir(ino uint64, entries []wire.Dirent) {
	if k == nil || ino == 0 {
		return
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.dirs == nil {
		k.dirs = map[uint64][]wire.Dirent{}
	}
	if _, ok := k.dirs[ino]; !ok && len(k.dirs) >= maxDirs {
		return
	}
	if entries == nil {
		entries = []wire.Dirent{}
	}
	k.dirs[ino] = entries
}

func (k *kernel) dir(ino uint64) ([]wire.Dirent, bool) {
	if k == nil {
		return nil, false
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	e, ok := k.dirs[ino]
	return e, ok
}

func (k *kernel) drop(ch wire.Change) {
	k.forgetName(ch.Parent, ch.Name)
	ours := k.take(ch.Parent, ch.Name)
	k.mu.Lock()
	delete(k.dirs, ch.Parent)
	parent := k.byID[ch.Parent]
	k.mu.Unlock()
	if parent == nil {
		return
	}
	if !ours {
		parent.NotifyEntry(ch.Name)
	}
	child := parent.GetChild(ch.Name)
	if child == nil {
		return
	}
	child.NotifyContent(0, 0)
	n, ok := child.Operations().(*node)
	if !ok {
		return
	}
	k.mu.Lock()
	delete(k.dirs, n.ino)
	k.mu.Unlock()
	if !ours {
		n.forgetAttr()
	}
}
