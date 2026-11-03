package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fs-relay/internal/client"
	"fs-relay/internal/local"
	"fs-relay/internal/wire"
)

func TestJoinEvictAndRejoin(t *testing.T) {
	tree := local.New(t.TempDir())
	t.Cleanup(func() { _ = tree.Close() })
	h := newHub(testToken)
	hnd, _ := handler(tree, h)
	srv := httptest.NewServer(hnd)
	t.Cleanup(srv.Close)
	httpClient := srv.Client()

	a := dialJoin(t, httpClient, srv.URL, "")
	defer a.cancel()
	b := dialJoin(t, httpClient, srv.URL, "")
	defer b.cancel()

	post := &client.Client{URL: srv.URL, NS: "demo", Token: testToken, HTTP: httpClient}
	if _, err := post.Call(context.Background(), wire.Mkdir, wire.Params{Inode: 1, Name: "sub", Mode: 0o755}); err != nil {
		t.Fatal(err)
	}
	if got := awaitEvict(t, a.lines); got.Name != "sub" || got.Parent != 1 {
		t.Fatalf("a evict %+v", got)
	}
	if got := awaitEvict(t, b.lines); got.Name != "sub" || got.Parent != 1 {
		t.Fatalf("b evict %+v", got)
	}

	again := wire.Params{Inode: 1, Name: "sub", Mode: 0o755}
	if _, err := post.Call(context.Background(), wire.Mkdir, again); err == nil {
		t.Fatal("duplicate mkdir succeeded")
	}
	if _, err := post.Call(context.Background(), wire.Mkdir, wire.Params{Inode: 1, Name: "other", Mode: 0o755}); err != nil {
		t.Fatal(err)
	}
	if got := awaitEvict(t, a.lines); got.Name != "other" {
		t.Fatalf("refused mkdir published %+v", got)
	}
	if got := awaitEvict(t, b.lines); got.Name != "other" {
		t.Fatalf("b other %+v", got)
	}

	a.cancel()
	for range a.lines {
	}
	h.oweRelease("demo", a.id, "lock-1")
	back := dialJoin(t, httpClient, srv.URL, a.id)
	defer back.cancel()
	select {
	case p := <-back.lines:
		if p.Release != "lock-1" {
			t.Fatalf("rejoin %+v", p)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("rejoin missed the owed release")
	}
}

func TestHeldEvictReplayed(t *testing.T) {
	h := newHub(testToken)
	m, err := h.attach("demo", freshID(t), false, func() {})
	if err != nil {
		t.Fatal(err)
	}
	h.detach("demo", m)
	h.evict("demo", []wire.Change{{Parent: 1, Name: "gone"}})
	back, err := h.attach("demo", m.id, false, func() {})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case p := <-back.ch:
		if p.Evict == nil || p.Evict.Name != "gone" {
			t.Fatalf("replay %+v", p)
		}
	default:
		t.Fatal("evict was not held")
	}
}

func TestHoldExpires(t *testing.T) {
	h := newHub(testToken)
	h.holdFor = -time.Second
	m, err := h.attach("demo", freshID(t), false, func() {})
	if err != nil {
		t.Fatal(err)
	}
	h.evict("demo", []wire.Change{{Parent: 1, Name: "gone"}})
	h.detach("demo", m)
	back, err := h.attach("demo", m.id, false, func() {})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case p := <-back.ch:
		t.Fatalf("expired event %+v", p)
	default:
	}
}

func TestWSEvict(t *testing.T) {
	tree := local.New(t.TempDir())
	t.Cleanup(func() { _ = tree.Close() })
	hnd, _ := handler(tree, newHub(testToken))
	srv := httptest.NewServer(hnd)
	t.Cleanup(srv.Close)
	ctx := context.Background()
	one, err := client.Dial(ctx, srv.URL, "demo", testToken)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = one.Close() })
	two, err := client.Dial(ctx, srv.URL, "demo", testToken)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = two.Close() })
	if _, err := one.Call(ctx, wire.Mkdir, wire.Params{Inode: 1, Name: "sub", Mode: 0o755}); err != nil {
		t.Fatal(err)
	}
	if got := awaitEvict(t, recv(one.Events())); got.Name != "sub" {
		t.Fatalf("one %+v", got)
	}
	if got := awaitEvict(t, recv(two.Events())); got.Name != "sub" {
		t.Fatalf("two %+v", got)
	}
}

type joined struct {
	id     string
	lines  chan wire.Pub
	cancel context.CancelFunc
}

func dialJoin(t *testing.T, httpClient *http.Client, base, id string) *joined {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	body, err := wire.MarshalRequest(wire.Join, wire.Params{Namespace: "demo", Client: id})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/fs/demo", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", wire.ContentType)
	req.Header.Set(wire.TokenHeader, wire.TokenScheme+" "+testToken)
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("join http %d", resp.StatusCode)
	}
	frame, err := wire.ReadFrame(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	first, err := wire.DecodePub(frame)
	if err != nil || first.Client == "" {
		t.Fatalf("client frame %+v %v", first, err)
	}
	lines := make(chan wire.Pub, 8)
	go func() {
		defer close(lines)
		defer resp.Body.Close()
		for {
			frame, err := wire.ReadFrame(resp.Body)
			if err != nil {
				return
			}
			p, err := wire.DecodePub(frame)
			if err != nil {
				return
			}
			if p.Evict == nil && p.Release == "" && p.Client == "" {
				continue
			}
			lines <- p
		}
	}()
	return &joined{id: first.Client, lines: lines, cancel: cancel}
}

func awaitEvict(t *testing.T, lines <-chan wire.Pub) wire.Change {
	t.Helper()
	select {
	case p := <-lines:
		if p.Evict == nil {
			t.Fatalf("line %+v", p)
		}
		return *p.Evict
	case <-time.After(2 * time.Second):
		t.Fatal("no eviction")
	}
	return wire.Change{}
}

func recv(events <-chan wire.Change) <-chan wire.Pub {
	out := make(chan wire.Pub, 1)
	go func() {
		ch := <-events
		out <- wire.Pub{Evict: &ch}
	}()
	return out
}

func TestReleaseAllReplayedOnRejoin(t *testing.T) {
	tree := local.New(t.TempDir())
	t.Cleanup(func() { _ = tree.Close() })
	h := newHub(testToken)
	hnd, _ := handler(tree, h)
	srv := httptest.NewServer(hnd)
	t.Cleanup(srv.Close)
	httpClient := srv.Client()

	a := dialJoin(t, httpClient, srv.URL, "")
	defer a.cancel()
	h.releaseAll("demo")
	select {
	case p := <-a.lines:
		if p.Release == "" {
			t.Fatalf("first line %+v", p)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no release")
	}

	a.cancel()
	for range a.lines {
	}
	back := dialJoin(t, httpClient, srv.URL, a.id)
	defer back.cancel()
	select {
	case p := <-back.lines:
		if p.Release == "" {
			t.Fatalf("rejoin line %+v", p)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("rejoin missed the release")
	}
}
