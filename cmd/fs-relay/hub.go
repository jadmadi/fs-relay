package main

import (
	"sync"

	"fs-relay/internal/wire"
)

const pubQueue = 64

// hub fans one namespace's events out to its join streams and websockets.
type hub struct {
	mu    sync.Mutex
	rooms map[string]*room
}

type room struct {
	live map[string]*member
	owed map[string][]wire.Pub
}

type member struct {
	id     string
	ch     chan wire.Pub
	ws     bool
	cancel func()
}

func newHub() *hub {
	return &hub{rooms: map[string]*room{}}
}

func (h *hub) attach(ns, id string, ws bool, cancel func()) (*member, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if id == "" {
		var err error
		id, err = newID()
		if err != nil {
			return nil, err
		}
	}
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
		return
	}
	h.keepOwed(r, m)
}

func (h *hub) keepOwed(r *room, m *member) {
	for {
		select {
		case p := <-m.ch:
			if p.Release != "" {
				r.owed[m.id] = append(r.owed[m.id], p)
			}
		default:
			return
		}
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
		h.fan(r, wire.Pub{Evict: &c}, false)
	}
}

// oweRelease remembers a lock release for one client. A live stream gets it
// now. A later join of the same uuid receives it if this one did not.
func (h *hub) oweRelease(ns, client, id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r := h.room(ns)
	p := wire.Pub{Release: id}
	m := r.live[client]
	if m == nil {
		r.owed[client] = append(r.owed[client], p)
		return
	}
	h.give(r, m, p, true)
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
	if owe || p.Release != "" {
		r.owed[m.id] = append(r.owed[m.id], p)
	}
	m.cancel()
}

func (h *hub) room(ns string) *room {
	r := h.rooms[ns]
	if r == nil {
		r = &room{live: map[string]*member{}, owed: map[string][]wire.Pub{}}
		h.rooms[ns] = r
	}
	return r
}
