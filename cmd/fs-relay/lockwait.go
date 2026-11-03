package main

import (
	"context"
	"errors"
	"syscall"

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

// callWait runs one call. A setlkw that conflicts parks here, on a namespace
// wake-up, and retries. The waiter registers before the first try, so a
// release that lands between the try and the park still wakes it.
func callWait(ctx context.Context, h *hub, tree *local.Tree, ns, method string, p wire.Params, onWait func() error) (wire.Result, error) {
	if method != wire.Setlkw {
		return tree.Call(ctx, method, p)
	}
	w := h.park(ns)
	defer h.unpark(ns, w)
	waited := false
	for {
		if ctx.Err() != nil {
			return wire.Result{}, syscall.EINTR
		}
		res, err := tree.Call(ctx, method, p)
		if err == nil || !errors.Is(err, syscall.EAGAIN) {
			return res, err
		}
		if !waited {
			waited = true
			if onWait != nil {
				if err := onWait(); err != nil {
					return wire.Result{}, err
				}
			}
		}
		select {
		case <-w.ch:
		case <-ctx.Done():
			return wire.Result{}, syscall.EINTR
		}
	}
}
