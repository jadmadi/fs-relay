package main

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"syscall"

	"fs-relay/internal/local"
	"fs-relay/internal/wire"
)

func newHandler(tree *local.Tree, token string) http.Handler {
	h, _ := handler(tree, newHub(token))
	return h
}

func handler(tree *local.Tree, h *hub) (http.Handler, *rawSet) {
	h.tree = tree
	set := newRawSet()
	min, max := rawLimits(tree)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", serveRoot)
	mux.HandleFunc("POST /fs/{ns}", func(w http.ResponseWriter, r *http.Request) {
		serve(h, tree, w, r)
	})
	mux.HandleFunc("GET /ws/{ns}", func(w http.ResponseWriter, r *http.Request) {
		serveWS(h, tree, w, r)
	})
	mux.HandleFunc("GET /raw/{ns}", func(w http.ResponseWriter, r *http.Request) {
		serveRaw(h, tree, set, min, max, w, r)
	})
	return withToken(h.token, mux), set
}

func serve(h *hub, tree *local.Tree, w http.ResponseWriter, r *http.Request) {
	ns := r.PathValue("ns")
	if err := wire.CheckNS(ns); err != nil {
		http.Error(w, "bad namespace", http.StatusBadRequest)
		return
	}
	if !flatbufferType(r.Header.Get("Content-Type")) {
		http.Error(w, "content type", http.StatusBadRequest)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, wire.MaxBody)
	req, err := wire.DecodeRequest(r.Body)
	if err != nil || req.Params.Namespace != ns {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	if req.Method == wire.Join {
		if err := tree.CheckNamespace(ns); err != nil {
			writeNamespaceErr(w, ns, err)
			return
		}
		serveJoin(h, ns, w, r, req.Params.Client)
		return
	}
	needsClient := isLock(req.Method) || req.Method == wire.Release && req.Params.Fh == 0
	if needsClient && !clientSigned(h.token, req.Params.Client) {
		http.Error(w, "bad client", http.StatusBadRequest)
		return
	}
	res, err := tree.Call(r.Context(), req.Method, req.Params)
	if req.Method == wire.Setlkw && errors.Is(err, syscall.EAGAIN) {
		h.markWaiting(ns, lockInode(tree, req.Params), req.Params.Client)
	}
	publishCall(h, tree, ns, req.Method, req.Params, err)
	writeCall(w, res, err)
}

func writeNamespaceErr(w http.ResponseWriter, ns string, err error) {
	switch {
	case errors.Is(err, os.ErrNotExist):
		http.Error(w, "namespace "+ns+" does not exist", http.StatusNotFound)
	case errors.Is(err, syscall.ENOTDIR):
		http.Error(w, "namespace "+ns+" is not a directory", http.StatusNotFound)
	case errors.Is(err, os.ErrPermission):
		http.Error(w, "namespace "+ns+": permission denied", http.StatusForbidden)
	default:
		http.Error(w, "namespace "+ns+": "+err.Error(), http.StatusInternalServerError)
	}
}

func publishCall(h *hub, tree *local.Tree, ns, method string, p wire.Params, err error) {
	if err != nil {
		return
	}
	h.evict(ns, tree.Changed(method, p))
	switch method {
	case wire.Setlk, wire.Setlkw, wire.Release:
		if ino, ok := tree.TookWake(ns, p); ok && h.clearWaiting(ns, ino) {
			h.releaseAll(ns)
		}
	}
}

func flatbufferType(ct string) bool {
	return ct == wire.ContentType || strings.HasPrefix(ct, wire.ContentType+";")
}

func writeCall(w http.ResponseWriter, res wire.Result, err error) {
	if err == nil {
		writeResult(w, res)
		return
	}
	if errors.Is(err, wire.ErrParams) || errors.Is(err, wire.ErrUnknown) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var n syscall.Errno
	if errors.As(err, &n) && n != 0 {
		b, merr := wire.MarshalCallError(wire.Error{Code: uint32(n), Message: n.Error()})
		if merr != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		writeBody(w, http.StatusOK, b)
		return
	}
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func writeResult(w http.ResponseWriter, res wire.Result) {
	b, err := wire.MarshalResult(res)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	writeBody(w, http.StatusOK, b)
}

func writeBody(w http.ResponseWriter, status int, b []byte) {
	w.Header().Set("Content-Type", wire.ContentType)
	w.WriteHeader(status)
	_, _ = w.Write(b)
}
