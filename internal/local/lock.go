package local

import (
	"context"
	"syscall"

	"fs-relay/internal/wire"
	"golang.org/x/sys/unix"
)

// The three byte ranges SQLite locks. They are the hot path.
const (
	pendingByte  uint64 = 0x40000000
	reservedByte uint64 = 0x40000001
	sharedStart  uint64 = 0x40000002
	sharedEnd    uint64 = sharedStart + 510 - 1
	endOfFile    uint64 = ^uint64(0)
)

// lockKey names one holder. The kernel owner alone is not unique across mounts.
type lockKey struct {
	client string
	owner  uint64
}

// lockSpan is one byte range held by one owner. end is inclusive.
type lockSpan struct {
	start uint64
	end   uint64
	typ   uint32
	pid   uint32
}

type lockOwner struct {
	key   lockKey
	fh    uint64
	flock uint32
	spans []lockSpan
}

// lockRec is one inode's lock state. It is allocated on the first lock and
// lives as long as the inode.
type lockRec struct {
	shared    uint32
	pendingR  uint32
	overflow  uint32 // non-canonical spans, any type
	overflowW uint32 // non-canonical write spans
	reserved  lockKey
	pending   lockKey
	exclusive lockKey
	flockX    lockKey
	flockS    uint32
	owners    []lockOwner
	pool      [][]lockSpan
	wake      bool
}

func canonical(start, end uint64, typ uint32) bool {
	switch {
	case start == pendingByte && end == pendingByte:
		return typ == wire.LockRead || typ == wire.LockWrite
	case start == reservedByte && end == reservedByte:
		return typ == wire.LockWrite
	case start == sharedStart && end == sharedEnd:
		return typ == wire.LockRead || typ == wire.LockWrite
	}
	return false
}

func overlap(aStart, aEnd, bStart, bEnd uint64) bool {
	return aStart <= bEnd && bStart <= aEnd
}

func (r *lockRec) find(key lockKey) *lockOwner {
	for i := range r.owners {
		if r.owners[i].key == key {
			return &r.owners[i]
		}
	}
	return nil
}

func (r *lockRec) owner(key lockKey, fh uint64) *lockOwner {
	if o := r.find(key); o != nil {
		o.fh = fh
		return o
	}
	var spans []lockSpan
	if n := len(r.pool); n > 0 {
		spans = r.pool[n-1][:0]
		r.pool = r.pool[:n-1]
	}
	r.owners = append(r.owners, lockOwner{key: key, fh: fh, spans: spans})
	return &r.owners[len(r.owners)-1]
}

func (r *lockRec) remove(key lockKey) {
	for i := range r.owners {
		if r.owners[i].key != key {
			continue
		}
		last := len(r.owners) - 1
		r.pool = append(r.pool, r.owners[i].spans[:0])
		r.owners[i] = r.owners[last]
		r.owners[last] = lockOwner{}
		r.owners = r.owners[:last]
		return
	}
}

func (o *lockOwner) empty() bool {
	return len(o.spans) == 0 && o.flock == 0
}

// trim removes [start, end] from this owner, keeping any bytes on either side.
func (o *lockOwner) trim(start, end uint64) {
	out := o.spans[:0]
	for _, s := range o.spans {
		if s.end < start || s.start > end {
			out = append(out, s)
			continue
		}
		if s.start < start {
			out = append(out, lockSpan{start: s.start, end: start - 1, typ: s.typ, pid: s.pid})
		}
		if s.end > end {
			out = append(out, lockSpan{start: end + 1, end: s.end, typ: s.typ, pid: s.pid})
		}
	}
	o.spans = out
}

// exact returns the type of a span covering exactly [start, end].
func (o *lockOwner) exact(start, end uint64) (uint32, bool) {
	for _, s := range o.spans {
		if s.start == start && s.end == end {
			return s.typ, true
		}
	}
	return 0, false
}

func (o *lockOwner) canonicalOnly() bool {
	for _, s := range o.spans {
		if !canonical(s.start, s.end, s.typ) {
			return false
		}
	}
	return true
}

// conflicts reports whether another owner blocks this request.
func (r *lockRec) conflicts(self lockKey, start, end uint64, typ uint32) bool {
	if r.overflow == 0 {
		switch {
		case typ == wire.LockRead && start == pendingByte && end == pendingByte:
			return r.pending != (lockKey{}) && r.pending != self
		case typ == wire.LockRead && start == sharedStart && end == sharedEnd:
			return r.exclusive != (lockKey{}) && r.exclusive != self
		case typ == wire.LockWrite && start == reservedByte && end == reservedByte:
			return r.reserved != (lockKey{}) && r.reserved != self
		case typ == wire.LockWrite && start == pendingByte && end == pendingByte:
			if r.pending != (lockKey{}) && r.pending != self {
				return true
			}
			if r.pendingR == 0 {
				return false
			}
		}
	}
	for i := range r.owners {
		if r.owners[i].key == self {
			continue
		}
		for _, s := range r.owners[i].spans {
			if !overlap(s.start, s.end, start, end) {
				continue
			}
			if s.typ == wire.LockWrite || typ == wire.LockWrite {
				return true
			}
		}
	}
	return false
}

// put takes [start, end] for one owner, replacing that owner's own overlap.
func (r *lockRec) put(key lockKey, fh uint64, start, end uint64, typ uint32, pid uint32) {
	o := r.owner(key, fh)
	if canonical(start, end, typ) && o.canonicalOnly() {
		r.apply(o, start, end, typ, pid)
		return
	}
	o.trim(start, end)
	o.spans = append(o.spans, lockSpan{start: start, end: end, typ: typ, pid: pid})
	r.recount()
}

func (r *lockRec) apply(o *lockOwner, start, end uint64, typ uint32, pid uint32) {
	switch {
	case start == pendingByte && typ == wire.LockRead:
		t, ok := o.exact(start, end)
		if ok && t == wire.LockWrite {
			o.trim(start, end)
			if r.pending == o.key {
				r.pending = lockKey{}
			}
			o.spans = append(o.spans, lockSpan{start: start, end: end, typ: typ, pid: pid})
			r.pendingR++
			return
		}
		if !ok {
			o.spans = append(o.spans, lockSpan{start: start, end: end, typ: typ, pid: pid})
			r.pendingR++
		}
	case start == pendingByte && typ == wire.LockWrite:
		o.trim(start, end)
		o.spans = append(o.spans, lockSpan{start: start, end: end, typ: typ, pid: pid})
		r.pending = o.key
	case start == reservedByte:
		o.trim(start, end)
		o.spans = append(o.spans, lockSpan{start: start, end: end, typ: typ, pid: pid})
		r.reserved = o.key
	case start == sharedStart && typ == wire.LockRead:
		if r.exclusive == o.key {
			o.trim(start, end)
			r.exclusive = lockKey{}
		}
		if _, ok := o.exact(start, end); !ok {
			o.spans = append(o.spans, lockSpan{start: start, end: end, typ: typ, pid: pid})
			r.shared++
		}
	case start == sharedStart && typ == wire.LockWrite:
		if t, ok := o.exact(start, end); ok {
			o.trim(start, end)
			if t == wire.LockRead {
				r.shared--
			}
		}
		o.spans = append(o.spans, lockSpan{start: start, end: end, typ: typ, pid: pid})
		r.exclusive = o.key
	}
}

// unlock drops the range for one owner. It is a no-op for a range not held.
func (r *lockRec) unlock(key lockKey, start, end uint64) {
	o := r.find(key)
	if o == nil {
		return
	}
	o.trim(start, end)
	if o.empty() {
		r.remove(key)
	}
	r.recount()
	r.wake = true
}

func (r *lockRec) blocker(self lockKey, start, end uint64, typ uint32) (lockSpan, bool) {
	for i := range r.owners {
		if r.owners[i].key == self {
			continue
		}
		for _, s := range r.owners[i].spans {
			if !overlap(s.start, s.end, start, end) {
				continue
			}
			if s.typ == wire.LockWrite || typ == wire.LockWrite {
				return s, true
			}
		}
	}
	return lockSpan{}, false
}

// recount rebuilds the canonical counters from the owners. It runs on a
// partial range change, not on the hot path.
func (r *lockRec) recount() {
	r.shared, r.pendingR = 0, 0
	r.overflow = 0
	r.reserved, r.pending, r.exclusive = lockKey{}, lockKey{}, lockKey{}
	for i := range r.owners {
		for _, s := range r.owners[i].spans {
			switch {
			case s.start == pendingByte && s.end == pendingByte && s.typ == wire.LockRead:
				r.pendingR++
			case s.start == pendingByte && s.end == pendingByte && s.typ == wire.LockWrite:
				r.pending = r.owners[i].key
			case s.start == reservedByte && s.end == reservedByte && s.typ == wire.LockWrite:
				r.reserved = r.owners[i].key
			case s.start == sharedStart && s.end == sharedEnd && s.typ == wire.LockRead:
				r.shared++
			case s.start == sharedStart && s.end == sharedEnd && s.typ == wire.LockWrite:
				r.exclusive = r.owners[i].key
			default:
				r.overflow++
			}
		}
	}
}

func (r *lockRec) flockSet(key lockKey, fh uint64, typ uint32) error {
	o := r.owner(key, fh)
	switch typ {
	case wire.LockUnlock:
		o.flock = 0
		r.wake = true
	case wire.LockRead:
		for i := range r.owners {
			if r.owners[i].key != key && r.owners[i].flock == wire.LockWrite {
				return syscall.EAGAIN
			}
		}
		o.flock = wire.LockRead
	case wire.LockWrite:
		for i := range r.owners {
			if r.owners[i].key != key && r.owners[i].flock != 0 {
				return syscall.EAGAIN
			}
		}
		o.flock = wire.LockWrite
	default:
		return syscall.EINVAL
	}
	if o.empty() {
		r.remove(key)
	}
	return nil
}

// dropFH drops every owner that last named this file handle.
func (r *lockRec) dropFH(fh uint64) bool {
	changed := false
	for i := len(r.owners) - 1; i >= 0; i-- {
		if r.owners[i].fh != fh {
			continue
		}
		r.remove(r.owners[i].key)
		changed = true
	}
	if changed {
		r.recount()
		r.wake = true
	}
	return changed
}

// dropClient drops every lock one mount holds on this inode.
func (r *lockRec) dropClient(client string) bool {
	changed := false
	for i := len(r.owners) - 1; i >= 0; i-- {
		if r.owners[i].key.client != client {
			continue
		}
		r.remove(r.owners[i].key)
		changed = true
	}
	if changed {
		r.recount()
		r.wake = true
	}
	return changed
}

// entry returns an inode's entry. The caller holds t.mu.
func (t *Tree) entry(ns string, ino uint64) (*entry, bool) {
	st := t.ns[ns]
	if st == nil {
		return nil, false
	}
	e, ok := st.byIno[ino]
	return e, ok
}

// record returns an inode's lock record, allocating on first use.
// The caller holds t.mu.
func (t *Tree) record(e *entry) *lockRec {
	if e.locks == nil {
		e.locks = &lockRec{}
	}
	return e.locks
}

func lockTarget(t *Tree, p wire.Params) (uint64, lockKey, error) {
	ino := p.Inode
	if ino == 0 && p.Fh != 0 {
		if id, ok := t.inoOf(p.Fh); ok {
			ino = id
		}
	}
	if ino == 0 || p.Client == "" {
		return 0, lockKey{}, syscall.EINVAL
	}
	rel, err := t.relOf(p.Namespace, ino)
	if err != nil {
		return 0, lockKey{}, err
	}
	abs, err := t.abs(p.Namespace, rel)
	if err != nil {
		return 0, lockKey{}, err
	}
	var st unix.Stat_t
	if err := unix.Stat(abs, &st); err != nil {
		return 0, lockKey{}, err
	}
	if st.Mode&syscall.S_IFMT != syscall.S_IFREG {
		return 0, lockKey{}, syscall.EINVAL
	}
	return ino, lockKey{client: p.Client, owner: p.Owner}, nil
}

func lockRange(p wire.Params) (uint64, uint64, error) {
	if p.Offset < 0 {
		return 0, 0, syscall.EINVAL
	}
	start := uint64(p.Offset)
	if p.Size == 0 || p.Size == endOfFile {
		return start, endOfFile, nil
	}
	end := start + p.Size - 1
	if end < start {
		return 0, 0, syscall.EINVAL
	}
	return start, end, nil
}

func getlk(ctx context.Context, t *Tree, p wire.Params) (wire.Result, error) {
	if err := ctx.Err(); err != nil {
		return wire.Result{}, err
	}
	ino, key, err := lockTarget(t, p)
	if err != nil {
		return wire.Result{}, wrap("getlk", err)
	}
	if p.Lock != wire.LockRead && p.Lock != wire.LockWrite {
		return wire.Result{}, wrap("getlk", syscall.EINVAL)
	}
	start, end, err := lockRange(p)
	if err != nil {
		return wire.Result{}, wrap("getlk", err)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	e, ok := t.entry(p.Namespace, ino)
	if !ok {
		return wire.Result{}, wrap("getlk", syscall.ENOENT)
	}
	rec := t.record(e)
	out := wire.Lock{Type: wire.LockUnlock}
	if s, found := rec.blocker(key, start, end, p.Lock); found {
		out = wire.Lock{Type: s.typ, Start: s.start, End: s.end, Pid: s.pid}
	}
	return wire.Result{Lock: &out}, nil
}

func setlk(ctx context.Context, t *Tree, p wire.Params) (wire.Result, error) {
	if err := ctx.Err(); err != nil {
		return wire.Result{}, err
	}
	ino, key, err := lockTarget(t, p)
	if err != nil {
		return wire.Result{}, wrap("setlk", err)
	}
	if p.Flags&^wire.FlagFlock != 0 {
		return wire.Result{}, wrap("setlk", syscall.EINVAL)
	}
	flock := p.Flags&wire.FlagFlock != 0
	start, end, err := lockRange(p)
	if err != nil {
		return wire.Result{}, wrap("setlk", err)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	e, ok := t.entry(p.Namespace, ino)
	if !ok {
		return wire.Result{}, wrap("setlk", syscall.ENOENT)
	}
	rec := t.record(e)
	if flock {
		if err := rec.flockSet(key, p.Fh, p.Lock); err != nil {
			return wire.Result{}, wrap("setlk", err)
		}
		return wire.Result{}, nil
	}
	switch p.Lock {
	case wire.LockUnlock:
		rec.unlock(key, start, end)
	case wire.LockRead, wire.LockWrite:
		if rec.conflicts(key, start, end, p.Lock) {
			return wire.Result{}, wrap("setlk", syscall.EAGAIN)
		}
		rec.put(key, p.Fh, start, end, p.Lock, p.Pid)
	default:
		return wire.Result{}, wrap("setlk", syscall.EINVAL)
	}
	return wire.Result{}, nil
}

// DropFH drops the locks an open held on one inode.
func (t *Tree) DropFH(ns string, ino, fh uint64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	e, ok := t.entry(ns, ino)
	if !ok || e.locks == nil {
		return false
	}
	return e.locks.dropFH(fh)
}

// DropClient drops every lock one mount holds on one inode.
func (t *Tree) DropOwner(ns string, ino uint64, client string, owner uint64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	e, ok := t.entry(ns, ino)
	if !ok || e.locks == nil {
		return false
	}
	key := lockKey{client: client, owner: owner}
	if e.locks.find(key) == nil {
		return false
	}
	e.locks.remove(key)
	e.locks.recount()
	e.locks.wake = true
	return true
}

// ReleaseClient drops every lock one mount holds in a namespace.
func (t *Tree) ReleaseClient(ns, client string) bool {
	t.mu.Lock()
	recs := make([]*lockRec, 0, 8)
	if st := t.ns[ns]; st != nil {
		for _, e := range st.byIno {
			if e.locks != nil {
				recs = append(recs, e.locks)
			}
		}
	}
	changed := false
	for _, r := range recs {
		if r.dropClient(client) {
			changed = true
		}
	}
	t.mu.Unlock()
	return changed
}

// InodeOf resolves a file handle to its protocol inode.
func (t *Tree) InodeOf(fh uint64) (uint64, bool) {
	return t.inoOf(fh)
}

// TookWake consumes a pending release wake for the inode a call named.
func (t *Tree) TookWake(ns string, p wire.Params) bool {
	ino := p.Inode
	if ino == 0 && p.Fh != 0 {
		ino, _ = t.inoOf(p.Fh)
	}
	if ino == 0 {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	e, ok := t.entry(ns, ino)
	if !ok || e.locks == nil || !e.locks.wake {
		return false
	}
	e.locks.wake = false
	return true
}
