package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"

	"fs-relay/internal/client"
	"fs-relay/internal/wire"
)

type countTrip struct {
	base http.RoundTripper
	n    int
}

func (c *countTrip) RoundTrip(r *http.Request) (*http.Response, error) {
	c.n++
	return c.base.RoundTrip(r)
}

func TestOnePost(t *testing.T) {
	tree := newTestTree(t)
	t.Cleanup(func() { tree.Close() })
	srv := httptest.NewServer(newHandler(tree, testToken))
	t.Cleanup(srv.Close)
	httpClient := srv.Client()
	counter := &countTrip{base: httpClient.Transport}
	httpClient.Transport = counter
	c := &client.Client{URL: srv.URL, NS: "demo", Token: testToken, HTTP: httpClient}

	if _, err := c.Call(context.Background(), wire.Mkdir, wire.Params{Inode: 1, Name: "sub", Mode: 0o755}); err != nil {
		t.Fatal(err)
	}
	if counter.n != 1 {
		t.Fatalf("posts %d", counter.n)
	}
	_, err := c.Call(context.Background(), wire.Lookup, wire.Params{Inode: 1, Name: "missing"})
	var werr *wire.Error
	if !errors.As(err, &werr) || werr.Code != uint32(syscall.ENOENT) {
		t.Fatalf("%v", err)
	}
	if counter.n != 2 {
		t.Fatalf("posts %d", counter.n)
	}
	_, err = c.Call(context.Background(), "nope", wire.Params{Inode: 1})
	if err == nil || errors.As(err, &werr) {
		t.Fatalf("unknown method: %v", err)
	}
}
