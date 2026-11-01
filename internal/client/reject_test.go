package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestListenStopsWhenTokenIsRefused(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":401,"message":"wrong token"}}`))
	}))
	t.Cleanup(srv.Close)
	c := &Client{URL: srv.URL, NS: "demo", Token: "testtoken"}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := c.Listen(ctx)
	if !Refused(err) || !strings.Contains(err.Error(), "wrong token") || !strings.Contains(err.Error(), "Stopping") {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Fatalf("tries %d", hits.Load())
	}
}

func TestDialStopsWhenTokenIsRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":401,"message":"wrong token"}}`))
	}))
	t.Cleanup(srv.Close)
	_, err := Dial(context.Background(), srv.URL, "demo", "testtoken")
	if !Refused(err) || !strings.Contains(err.Error(), "wrong token") {
		t.Fatal(err)
	}
}
