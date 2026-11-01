package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"fs-relay/internal/wire"
)

func serveJoin(h *hub, ns string, w http.ResponseWriter, r *http.Request, client string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	m, err := h.attach(ns, client, false, cancel)
	if err != nil {
		writeJoinErr(w, r, err)
		return
	}
	defer h.detach(ns, m)
	w.Header().Set("Content-Type", wire.ContentType)
	w.WriteHeader(http.StatusOK)
	if writePub(w, flusher, wire.Pub{Client: m.id}) != nil {
		return
	}
	pump(ctx, h, ns, m, func(p wire.Pub) error {
		return writePub(w, flusher, p)
	})
}

func writeJoinErr(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errClient) {
		log.Printf("fs-relay: bad client from %s", clientIP(r))
		http.Error(w, "bad client", http.StatusBadRequest)
		return
	}
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// pump writes events from the member. A release stays owed until a later
// event is written, so a stream that dies on this write is replayed on rejoin.
func pump(ctx context.Context, h *hub, ns string, m *member, send func(wire.Pub) error) {
	var sent string
	tick := time.NewTicker(wire.Heartbeat)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if send(wire.Pub{}) != nil {
				return
			}
		case p := <-m.ch:
			if ctx.Err() != nil || send(p) != nil {
				putBack(m, p)
				return
			}
			h.ackRelease(ns, m.id, sent)
			sent = p.Release
		}
	}
}

func putBack(m *member, p wire.Pub) {
	select {
	case m.ch <- p:
	default:
	}
}

func writePub(w http.ResponseWriter, flusher http.Flusher, p wire.Pub) error {
	b, err := wire.MarshalPub(p)
	if err != nil {
		return err
	}
	if err := wire.WriteFrame(w, b); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
