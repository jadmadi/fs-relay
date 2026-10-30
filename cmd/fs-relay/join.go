package main

import (
	"context"
	"net/http"

	"fs-relay/internal/wire"
)

func serveJoin(h *hub, ns string, w http.ResponseWriter, r *http.Request, client string) {
	if client != "" && !validID(client) {
		http.Error(w, "bad client", http.StatusBadRequest)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	m, err := h.attach(ns, client, false, cancel)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer h.detach(ns, m)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if writePub(w, flusher, wire.Pub{Client: m.id}) != nil {
		return
	}
	readJoin(ctx, w, flusher, m)
}

func readJoin(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, m *member) {
	for {
		select {
		case <-ctx.Done():
			return
		case p := <-m.ch:
			if writePub(w, flusher, p) != nil {
				return
			}
		}
	}
}

func writePub(w http.ResponseWriter, flusher http.Flusher, p wire.Pub) error {
	b, err := wire.MarshalPub(p)
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
