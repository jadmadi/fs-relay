package main

import (
	"sync"
	"time"

	"fs-relay/internal/local"
	"fs-relay/internal/wire"
)

const (
	pubQueue = 64
	holdFor  = 8 * time.Second
)

// hub fans one namespace's events out to its join streams and websockets.
type hub struct {
	mu      sync.Mutex
	rooms   map[string]*room
	holdFor time.Duration
	token   string
	tree    *local.Tree

	wmu     sync.Mutex
	waiters map[string]map[*waiter]struct{}
}

// waiter is one parked setlkw on a namespace. Its channel is a one-slot
// wake-up, so a release that lands before the wait still leaves a token.
type waiter struct {
	ch chan struct{}
}

type room struct {
	live map[string]*member
	owed map[string][]wire.Pub
	hold map[string]time.Time
}

type member struct {
	id     string
	ch     chan wire.Pub
	ws     bool
	cancel func()
}

func newHub(token string) *hub {
	return &hub{
		rooms:   map[string]*room{},
		holdFor: holdFor,
		token:   token,
		waiters: map[string]map[*waiter]struct{}{},
	}
}

// park registers a waiter before the first grant attempt. A release after
// this point always leaves a wake-up, so the wait cannot be lost.
func (h *hub) park(ns string) *waiter {
	w := &waiter{ch: make(chan struct{}, 1)}
	h.wmu.Lock()
	if h.waiters[ns] == nil {
		h.waiters[ns] = map[*waiter]struct{}{}
	}
	h.waiters[ns][w] = struct{}{}
	h.wmu.Unlock()
	return w
}

func (h *hub) unpark(ns string, w *waiter) {
	h.wmu.Lock()
	if set := h.waiters[ns]; set != nil {
		delete(set, w)
		if len(set) == 0 {
			delete(h.waiters, ns)
		}
	}
	h.wmu.Unlock()
}

// wake releases every parked call in the namespace. It is a no-op when
// nobody waits, which keeps an uncontended commit free of events.
func (h *hub) wake(ns string) {
	h.wmu.Lock()
	set := h.waiters[ns]
	out := make([]*waiter, 0, len(set))
	for w := range set {
		out = append(out, w)
	}
	h.wmu.Unlock()
	for _, w := range out {
		select {
		case w.ch <- struct{}{}:
		default:
		}
	}
}

func (h *hub) waiting(ns string) bool {
	h.wmu.Lock()
	defer h.wmu.Unlock()
	return len(h.waiters[ns]) > 0
}

func (h *hub) attach(ns, id string, ws bool, cancel func()) (*member, error) {
	if id == "" {
		raw, err := newID()
		if err != nil {
			return nil, err
		}
		id = signClient(h.token, raw)
	} else if !clientSigned(h.token, id) {
		return nil, errClient
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	r := h.room(ns)
	if old := r.live[id]; old != nil {
		old.cancel()
		delete(r.live, id)
	}
	m := &member{id: id, ch: make(chan wire.Pub, pubQueue), ws: ws, cancel: cancel}
	h.replay(r, m)
	r.live[id] = m
	return m, nil
}

func (h *hub) replay(r *room, m *member) {
	if h.holding(r, m.id) {
		delete(r.hold, m.id)
	}
	owed := r.owed[m.id]
	var rest []wire.Pub
	for _, p := range owed {
		select {
		case m.ch <- p:
		default:
			rest = append(rest, p)
		}
	}
	if len(rest) == 0 {
		delete(r.owed, m.id)
		return
	}
	r.owed[m.id] = rest
}

func (h *hub) detach(ns string, m *member) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r := h.rooms[ns]
	if r == nil || r.live[m.id] != m {
		return
	}
	delete(r.live, m.id)
	if m.ws {
		if r.hold == nil {
			r.hold = map[string]time.Time{}
		}
		r.hold[m.id] = time.Now().Add(h.holdFor)
		h.arm(ns, m.id)
		return
	}
	h.keepOwed(r, m)
	h.arm(ns, m.id)
}

func (h *hub) keepOwed(r *room, m *member) {
	if r.hold == nil {
		r.hold = map[string]time.Time{}
	}
	r.hold[m.id] = time.Now().Add(h.holdFor)
	for {
		select {
		case p := <-m.ch:
			h.remember(r, m.id, p)
		default:
			return
		}
	}
}

func (h *hub) remember(r *room, id string, p wire.Pub) {
	if p.Evict == nil && p.Release == "" {
		return
	}
	if p.Release != "" && owedHas(r.owed[id], p.Release) {
		return
	}
	r.owed[id] = append(r.owed[id], p)
}

func (h *hub) arm(ns, id string) {
	if h.holdFor <= 0 {
		return
	}
	time.AfterFunc(h.holdFor, func() { h.dropHold(ns, id) })
}

func (h *hub) dropHold(ns, id string) {
	h.mu.Lock()
	r := h.rooms[ns]
	if r == nil || r.live[id] != nil {
		h.mu.Unlock()
		return
	}
	until, held := r.hold[id]
	if !held || time.Now().Before(until) {
		h.mu.Unlock()
		return
	}
	delete(r.hold, id)
	delete(r.owed, id)
	h.mu.Unlock()
	if h.tree != nil && h.tree.ReleaseClient(ns, id) {
		h.wake(ns)
	}
}

func (h *hub) evict(ns string, changes []wire.Change) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r := h.rooms[ns]
	if r == nil {
		return
	}
	for _, c := range changes {
		c := c
		p := wire.Pub{Evict: &c}
		h.fan(r, p, false)
		h.holdAbsent(r, p)
	}
}

// oweRelease remembers a lock release for one client. The owed copy is what a
// rejoin replays. A live stream is also notified, and that copy is dropped
// from the owed list only after a later event shows the stream is still up.
func (h *hub) oweRelease(ns, client, id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r := h.room(ns)
	m := r.live[client]
	if m == nil && !h.holding(r, client) {
		return
	}
	p := wire.Pub{Release: id}
	r.owed[client] = append(r.owed[client], p)
	if m == nil {
		return
	}
	select {
	case m.ch <- p:
	default:
		m.cancel()
	}
}

func (h *hub) ackRelease(ns, client, id string) {
	if id == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	r := h.rooms[ns]
	if r == nil {
		return
	}
	owed := r.owed[client]
	for i, p := range owed {
		if p.Release != id {
			continue
		}
		r.owed[client] = append(owed[:i], owed[i+1:]...)
		if len(r.owed[client]) == 0 {
			delete(r.owed, client)
		}
		return
	}
}

func owedHas(owed []wire.Pub, id string) bool {
	for _, p := range owed {
		if p.Release == id {
			return true
		}
	}
	return false
}

func (h *hub) fan(r *room, p wire.Pub, owe bool) {
	for _, m := range r.live {
		h.give(r, m, p, owe)
	}
}

func (h *hub) give(r *room, m *member, p wire.Pub, owe bool) {
	select {
	case m.ch <- p:
		return
	default:
	}
	if !m.ws {
		h.remember(r, m.id, p)
		m.cancel()
		return
	}
	if owe || p.Release != "" {
		r.owed[m.id] = append(r.owed[m.id], p)
	}
	m.cancel()
}

func (h *hub) holdAbsent(r *room, p wire.Pub) {
	for id := range r.hold {
		if r.live[id] != nil || !h.holding(r, id) {
			continue
		}
		h.remember(r, id, p)
	}
}

func (h *hub) holding(r *room, id string) bool {
	until, ok := r.hold[id]
	if !ok {
		return false
	}
	if time.Now().Before(until) {
		return true
	}
	delete(r.hold, id)
	delete(r.owed, id)
	return false
}

func (h *hub) room(ns string) *room {
	r := h.rooms[ns]
	if r == nil {
		r = &room{
			live: map[string]*member{},
			owed: map[string][]wire.Pub{},
			hold: map[string]time.Time{},
		}
		h.rooms[ns] = r
	}
	return r
}
