package main

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"syscall"
	"time"

	"fs-relay/internal/local"
	"fs-relay/internal/wire"

	"github.com/coder/websocket"
)

func serveWS(h *hub, tree *local.Tree, w http.ResponseWriter, r *http.Request) {
	ns := r.PathValue("ns")
	if err := wire.CheckNS(ns); err != nil {
		http.Error(w, "bad namespace", http.StatusBadRequest)
		return
	}
	if err := tree.CheckNamespace(ns); err != nil {
		writeNamespaceErr(w, ns, err)
		return
	}
	client := r.URL.Query().Get("client")
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
		CompressionMode:    websocket.CompressionDisabled,
	})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	conn.SetReadLimit(wire.MaxBody)
	runWS(h, r.Context(), tree, conn, ns, client)
}

func runWS(h *hub, ctx context.Context, tree *local.Tree, conn *websocket.Conn, ns, client string) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	m, err := h.attach(ns, client, true, cancel)
	if err != nil {
		return
	}
	defer h.detach(ns, m)
	wr := &wsWriter{conn: conn}
	hello, err := wire.MarshalPub(wire.Pub{Client: m.id})
	if err != nil || wr.send(ctx, hello) != nil {
		return
	}
	var wg sync.WaitGroup
	defer wg.Wait()
	go pump(ctx, h, ns, m, func(p wire.Pub) error {
		b, err := pushBytes(p)
		if err != nil || b == nil {
			return nil
		}
		return wr.send(ctx, b)
	})
	go ping(ctx, conn)
	readCalls(ctx, h, tree, conn, ns, m.id, wr, &wg)
}

func ping(ctx context.Context, conn *websocket.Conn) {
	tick := time.NewTicker(wire.Heartbeat)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			pctx, cancel := context.WithTimeout(ctx, wire.Heartbeat)
			err := conn.Ping(pctx)
			cancel()
			if err != nil {
				_ = conn.Close(websocket.StatusGoingAway, "")
				return
			}
		}
	}
}

func readCalls(ctx context.Context, h *hub, tree *local.Tree, conn *websocket.Conn, ns, client string, wr *wsWriter, wg *sync.WaitGroup) {
	var mu sync.Mutex
	lanes := map[uint64]struct{}{}
	for {
		req, ok := readWS(ctx, conn)
		if !ok || req.Params.Namespace != ns || !claim(lanes, &mu, req.ID) {
			return
		}
		req.Params.Client = client
		wg.Add(1)
		go func(req wire.Request) {
			defer wg.Done()
			defer release(lanes, &mu, req.ID)
			res, callErr := tree.Call(ctx, req.Method, req.Params)
			publishCall(h, tree, ns, req.Method, req.Params, callErr)
			b, err := wsBody(req.ID, res, callErr)
			if err != nil {
				return
			}
			_ = wr.send(ctx, b)
		}(req)
	}
}

func readWS(ctx context.Context, conn *websocket.Conn) (wire.Request, bool) {
	_, data, err := conn.Read(ctx)
	if err != nil {
		return wire.Request{}, false
	}
	req, err := wire.DecodeRequestBuffer(data)
	if err != nil || req.ID == 0 {
		return wire.Request{}, false
	}
	return req, true
}

func claim(lanes map[uint64]struct{}, mu *sync.Mutex, id uint64) bool {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := lanes[id]; ok {
		return false
	}
	lanes[id] = struct{}{}
	return true
}

func release(lanes map[uint64]struct{}, mu *sync.Mutex, id uint64) {
	mu.Lock()
	delete(lanes, id)
	mu.Unlock()
}

func wsBody(id uint64, res wire.Result, err error) ([]byte, error) {
	if err == nil {
		return wire.MarshalReply(id, res)
	}
	if errors.Is(err, wire.ErrParams) || errors.Is(err, wire.ErrUnknown) {
		return wire.MarshalFault(id, err.Error())
	}
	var n syscall.Errno
	if errors.As(err, &n) && n != 0 {
		return wire.MarshalReplyError(id, wire.Error{Code: uint32(n), Message: n.Error()})
	}
	return wire.MarshalFault(id, http.StatusText(http.StatusInternalServerError))
}

type wsWriter struct {
	mu   sync.Mutex
	conn *websocket.Conn
}

func pushBytes(p wire.Pub) ([]byte, error) {
	if p.Evict != nil {
		return wire.MarshalPush(*p.Evict)
	}
	if p.Release != "" {
		return wire.MarshalRelease(p.Release)
	}
	return wire.MarshalPub(p)
}

func (w *wsWriter) send(ctx context.Context, b []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.Write(ctx, websocket.MessageBinary, b)
}
