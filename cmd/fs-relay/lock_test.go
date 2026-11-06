package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"
	"time"

	"fs-relay/internal/client"
	"fs-relay/internal/local"
	"fs-relay/internal/wire"

	"github.com/coder/websocket"
)

const (
	uuidA = "00000000-0000-7000-8000-00000000000a"
	uuidB = "00000000-0000-7000-8000-00000000000b"
	uuidC = "00000000-0000-7000-8000-00000000000c"
)

func lockServer(t *testing.T) (*httptest.Server, *local.Tree, *hub) {
	t.Helper()
	tree := newTestTree(t)
	t.Cleanup(func() { tree.Close() })
	h := newHub(testToken)
	srv := httptest.NewServer(mustHandler(tree, h))
	t.Cleanup(srv.Close)
	return srv, tree, h
}

func mustHandler(tree *local.Tree, h *hub) http.Handler {
	got, _ := handler(tree, h)
	return got
}

type lockClient struct {
	c   *client.Client
	ino uint64
	fh  uint64
}

func lockCaller(t *testing.T, url, token string) *lockClient {
	t.Helper()
	c := &client.Client{URL: url, NS: "demo", Token: token}
	res, err := c.Call(context.Background(), wire.Create, wire.Params{
		Inode: 1, Name: "db", Mode: 0o644, Flags: syscall.O_RDWR,
	})
	if err != nil {
		t.Fatal(err)
	}
	return &lockClient{c: c, ino: res.Attr.Ino, fh: res.Fh}
}

func TestPostSetlkwReturnsEAGAINAndReleaseWakesClient(t *testing.T) {
	srv, _, _ := lockServer(t)
	a := lockCaller(t, srv.URL, testToken)
	b := &client.Client{URL: srv.URL, NS: "demo", Token: testToken}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = b.Listen(ctx) }()
	time.Sleep(50 * time.Millisecond)

	pa := wire.Params{
		Inode: a.ino, Fh: a.fh, Client: signClient(testToken, uuidA),
		Owner: 1, Lock: wire.LockWrite, Offset: 100, Size: 1,
	}
	if _, err := a.c.Call(context.Background(), wire.Setlk, pa); err != nil {
		t.Fatal(err)
	}

	pb := wire.Params{
		Inode: a.ino, Client: signClient(testToken, uuidB), Owner: 2,
		Lock: wire.LockWrite, Offset: 100, Size: 1,
	}
	started := time.Now()
	_, err := b.Call(ctx, wire.Setlkw, pb)
	var werr *wire.Error
	if !errors.As(err, &werr) || werr.Code != uint32(syscall.EAGAIN) {
		t.Fatalf("setlkw: %v", err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("setlkw did not return at once")
	}

	released := b.Releases()
	pa.Lock = wire.LockUnlock
	if _, err := a.c.Call(context.Background(), wire.Setlk, pa); err != nil {
		t.Fatal(err)
	}
	select {
	case <-released:
	case <-time.After(2 * time.Second):
		t.Fatal("release event not received")
	}
}

func TestWebsocketSetlkwReturnsEAGAINAndReleaseArrives(t *testing.T) {
	srv, _, _ := lockServer(t)
	a := lockCaller(t, srv.URL, testToken)
	pa := wire.Params{
		Inode: a.ino, Fh: a.fh, Client: signClient(testToken, uuidA),
		Owner: 1, Lock: wire.LockWrite, Offset: 300, Size: 1,
	}
	if _, err := a.c.Call(context.Background(), wire.Setlk, pa); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/demo"
	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		HTTPHeader: map[string][]string{
			wire.TokenHeader: {wire.TokenScheme + " " + testToken},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	if _, _, err := conn.Read(ctx); err != nil {
		t.Fatalf("hello: %v", err)
	}
	body, err := wire.MarshalCall(1, wire.Setlkw, wire.Params{
		Namespace: "demo", Inode: a.ino,
		Owner: 2, Lock: wire.LockWrite, Offset: 300, Size: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Write(ctx, websocket.MessageBinary, body); err != nil {
		t.Fatal(err)
	}
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("reply: %v", err)
	}
	reply, err := wire.DecodeReply(data)
	if err != nil || reply.Err == nil || reply.Err.Code != uint32(syscall.EAGAIN) {
		t.Fatalf("reply %+v err %v", reply, err)
	}

	pa.Lock = wire.LockUnlock
	if _, err := a.c.Call(context.Background(), wire.Setlk, pa); err != nil {
		t.Fatal(err)
	}
	_, data, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("release read: %v", err)
	}
	push, err := wire.DecodeReply(data)
	if err != nil || push.Release == "" {
		t.Fatalf("release push %+v err %v", push, err)
	}
}

func TestHoldExpiryReleasesClientLocks(t *testing.T) {
	tree := newTestTree(t)
	t.Cleanup(func() { tree.Close() })
	h := newHub(testToken)
	h.tree = tree
	id := signClient(testToken, uuidA)

	res, err := tree.Call(context.Background(), wire.Create, wire.Params{
		Namespace: "demo", Inode: 1, Name: "db", Mode: 0o644, Flags: syscall.O_RDWR,
	})
	if err != nil {
		t.Fatal(err)
	}
	lock := wire.Params{
		Namespace: "demo", Inode: res.Attr.Ino, Fh: res.Fh,
		Client: id, Owner: 1, Lock: wire.LockWrite, Offset: 400, Size: 1,
	}
	if _, err := tree.Call(context.Background(), wire.Setlk, lock); err != nil {
		t.Fatal(err)
	}

	if _, err := tree.Call(context.Background(), wire.Setlk, wire.Params{
		Namespace: "demo", Inode: res.Attr.Ino, Client: signClient(testToken, uuidB),
		Owner: 2, Lock: wire.LockWrite, Offset: 400, Size: 1,
	}); !errors.Is(err, syscall.EAGAIN) {
		t.Fatalf("before expiry: %v", err)
	}

	r := h.room("demo")
	r.hold[id] = time.Now().Add(-time.Second)
	h.dropHold("demo", id)
	h.mu.Lock()
	_, still := r.hold[id]
	h.mu.Unlock()
	if still {
		t.Fatal("hold not cleared")
	}

	if _, err := tree.Call(context.Background(), wire.Setlk, wire.Params{
		Namespace: "demo", Inode: res.Attr.Ino, Client: signClient(testToken, uuidB),
		Owner: 2, Lock: wire.LockWrite, Offset: 400, Size: 1,
	}); err != nil {
		t.Fatalf("after expiry: %v", err)
	}
}

func TestHoldKeepsLocksOnRejoin(t *testing.T) {
	tree := newTestTree(t)
	t.Cleanup(func() { tree.Close() })
	h := newHub(testToken)
	h.tree = tree
	id := signClient(testToken, uuidA)

	res, err := tree.Call(context.Background(), wire.Create, wire.Params{
		Namespace: "demo", Inode: 1, Name: "db", Mode: 0o644, Flags: syscall.O_RDWR,
	})
	if err != nil {
		t.Fatal(err)
	}
	lock := wire.Params{
		Namespace: "demo", Inode: res.Attr.Ino, Fh: res.Fh,
		Client: id, Owner: 1, Lock: wire.LockWrite, Offset: 500, Size: 1,
	}
	if _, err := tree.Call(context.Background(), wire.Setlk, lock); err != nil {
		t.Fatal(err)
	}

	r := h.room("demo")
	r.live[id] = &member{id: id}
	r.hold[id] = time.Now().Add(time.Minute)
	h.dropHold("demo", id)

	if _, err := tree.Call(context.Background(), wire.Setlk, wire.Params{
		Namespace: "demo", Inode: res.Attr.Ino, Client: signClient(testToken, uuidB),
		Owner: 2, Lock: wire.LockWrite, Offset: 500, Size: 1,
	}); !errors.Is(err, syscall.EAGAIN) {
		t.Fatalf("rejoin dropped locks: %v", err)
	}
}

func TestThreeWaitersWakeOnSuccessiveUnlocks(t *testing.T) {
	srv, _, _ := lockServer(t)
	a := lockCaller(t, srv.URL, testToken)
	b := &client.Client{URL: srv.URL, NS: "demo", Token: testToken}
	c := &client.Client{URL: srv.URL, NS: "demo", Token: testToken}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = b.Listen(ctx) }()
	go func() { _ = c.Listen(ctx) }()
	time.Sleep(50 * time.Millisecond)

	pa := wire.Params{
		Inode: a.ino, Fh: a.fh, Client: signClient(testToken, uuidA),
		Owner: 1, Lock: wire.LockWrite, Offset: 600, Size: 1,
	}
	if _, err := a.c.Call(ctx, wire.Setlk, pa); err != nil {
		t.Fatal(err)
	}

	pb := wire.Params{
		Inode: a.ino, Client: signClient(testToken, uuidB),
		Owner: 2, Lock: wire.LockWrite, Offset: 600, Size: 1,
	}
	pc := wire.Params{
		Inode: a.ino, Client: signClient(testToken, uuidC),
		Owner: 3, Lock: wire.LockWrite, Offset: 600, Size: 1,
	}
	for _, tc := range []struct {
		cl *client.Client
		p  wire.Params
	}{{b, pb}, {c, pc}} {
		_, err := tc.cl.Call(ctx, wire.Setlkw, tc.p)
		var werr *wire.Error
		if !errors.As(err, &werr) || werr.Code != uint32(syscall.EAGAIN) {
			t.Fatalf("initial setlkw: %v", err)
		}
	}

	relB := b.Releases()
	relC1 := c.Releases()
	pa.Lock = wire.LockUnlock
	if _, err := a.c.Call(ctx, wire.Setlk, pa); err != nil {
		t.Fatal(err)
	}
	for _, ch := range []<-chan struct{}{relB, relC1} {
		select {
		case <-ch:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("first release event not received")
		}
	}

	if _, err := b.Call(ctx, wire.Setlkw, pb); err != nil {
		t.Fatalf("client B acquire: %v", err)
	}

	relC2 := c.Releases()
	pb.Lock = wire.LockUnlock
	if _, err := b.Call(ctx, wire.Setlk, pb); err != nil {
		t.Fatalf("client B unlock: %v", err)
	}
	select {
	case <-relC2:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("second release event not sent to remaining waiter C")
	}

	if _, err := c.Call(ctx, wire.Setlkw, pc); err != nil {
		t.Fatalf("client C acquire: %v", err)
	}
}

func TestConcurrentSetlkwAndUnlockNeverMissesRelease(t *testing.T) {
	srv, _, _ := lockServer(t)
	a := lockCaller(t, srv.URL, testToken)
	b := &client.Client{URL: srv.URL, NS: "demo", Token: testToken}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = b.Listen(ctx) }()
	time.Sleep(50 * time.Millisecond)

	idA := signClient(testToken, uuidA)
	idB := signClient(testToken, uuidB)
	for i := 0; i < 50; i++ {
		pa := wire.Params{
			Inode: a.ino, Fh: a.fh, Client: idA,
			Owner: 1, Lock: wire.LockWrite, Offset: 700, Size: 1,
		}
		if _, err := a.c.Call(ctx, wire.Setlk, pa); err != nil {
			t.Fatalf("iter %d lock A: %v", i, err)
		}

		rel := b.Releases()
		unlockDone := make(chan error, 1)
		go func() {
			u := pa
			u.Lock = wire.LockUnlock
			_, err := a.c.Call(ctx, wire.Setlk, u)
			unlockDone <- err
		}()

		pb := wire.Params{
			Inode: a.ino, Client: idB,
			Owner: 2, Lock: wire.LockWrite, Offset: 700, Size: 1,
		}
		_, err := b.Call(ctx, wire.Setlkw, pb)
		if uerr := <-unlockDone; uerr != nil {
			t.Fatalf("iter %d unlock A: %v", i, uerr)
		}
		if err != nil {
			var werr *wire.Error
			if !errors.As(err, &werr) || werr.Code != uint32(syscall.EAGAIN) {
				t.Fatalf("iter %d setlkw B: %v", i, err)
			}
			select {
			case <-rel:
			case <-time.After(200 * time.Millisecond):
				t.Fatalf("iter %d missed release after EAGAIN", i)
			}
			if _, err := b.Call(ctx, wire.Setlkw, pb); err != nil {
				t.Fatalf("iter %d retry setlkw B: %v", i, err)
			}
		}
		pb.Lock = wire.LockUnlock
		if _, err := b.Call(ctx, wire.Setlk, pb); err != nil {
			t.Fatalf("iter %d unlock B: %v", i, err)
		}
	}
}
