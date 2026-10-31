package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"syscall"
	"testing"
	"time"

	"fs-relay/internal/client"
	"fs-relay/internal/local"
	"fs-relay/internal/wire"

	"golang.org/x/sys/unix"
)

func TestRawRoundTrip(t *testing.T) {
	tree := local.New(t.TempDir())
	t.Cleanup(func() { _ = tree.Close() })
	hnd, _ := handler(tree, newHub())
	srv := httptest.NewServer(hnd)
	t.Cleanup(srv.Close)
	ctx := context.Background()
	raw, err := client.DialRaw(ctx, srv.URL, "demo")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	if raw.Min() <= 0 || raw.Max() < raw.Min() {
		t.Fatalf("limits %d %d", raw.Min(), raw.Max())
	}
	file, err := tree.Call(ctx, wire.Create, wire.Params{Namespace: "demo", Inode: 1, Name: "a", Mode: 0o644, Flags: syscall.O_RDWR})
	if err != nil {
		t.Fatal(err)
	}
	body := bytes.Repeat([]byte("xyz"), 30)
	n, err := raw.Write(ctx, file.Fh, 0, body)
	if err != nil || n != len(body) {
		t.Fatalf("write %d %v", n, err)
	}
	got := make([]byte, len(body))
	m, err := raw.ReadInto(ctx, file.Attr.Ino, 0, 0, got)
	if err != nil || m != len(body) || !bytes.Equal(got, body) {
		t.Fatalf("read %d %q %v", m, got, err)
	}
	_, err = raw.Write(ctx, 99, 0, []byte("nope"))
	var werr *wire.Error
	if !errors.As(err, &werr) || werr.Code != uint32(syscall.EBADF) {
		t.Fatalf("errno %v", err)
	}
	n, err = raw.Write(ctx, file.Fh, int64(len(body)), []byte("!"))
	if err != nil || n != 1 {
		t.Fatalf("after errno %d %v", n, err)
	}
}

func TestRawLargeWrite(t *testing.T) {
	tree := local.New(t.TempDir())
	t.Cleanup(func() { _ = tree.Close() })
	hnd, _ := handler(tree, newHub())
	srv := httptest.NewServer(hnd)
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	raw, err := client.DialRaw(ctx, srv.URL, "demo")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	file, err := tree.Call(ctx, wire.Create, wire.Params{Namespace: "demo", Inode: 1, Name: "a", Mode: 0o644, Flags: syscall.O_RDWR})
	if err != nil {
		t.Fatal(err)
	}
	body := bytes.Repeat([]byte("abcdefgh"), raw.Max()/8)
	n, err := raw.Write(ctx, file.Fh, 0, body)
	if err != nil || n != len(body) {
		t.Fatalf("write %d %v", n, err)
	}
	got := make([]byte, len(body))
	m, err := raw.ReadInto(ctx, file.Attr.Ino, 0, 0, got)
	if err != nil || m != len(body) || !bytes.Equal(got, body) {
		t.Fatalf("read %d %v equal=%v", m, err, bytes.Equal(got, body))
	}
	pipe, err := local.NewPipe(len(body) + wire.RawSpare)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pipe.Close() })
	m, err = raw.ReadToPipe(ctx, file.Attr.Ino, 0, 0, len(body), pipe.WriteFd())
	if err != nil || m != len(body) {
		t.Fatalf("pipe read %d %v", m, err)
	}
	got = make([]byte, len(body))
	off := 0
	for off < len(got) {
		c, err := unix.Read(pipe.ReadFd(), got[off:])
		if err != nil || c == 0 {
			t.Fatalf("pipe body at %d %v", off, err)
		}
		off += c
	}
	if !bytes.Equal(got, body) {
		t.Fatal("pipe mismatch")
	}
}

func TestRawEvict(t *testing.T) {
	tree := local.New(t.TempDir())
	t.Cleanup(func() { _ = tree.Close() })
	hnd, _ := handler(tree, newHub())
	srv := httptest.NewServer(hnd)
	t.Cleanup(srv.Close)
	ctx := context.Background()
	joined := dialJoin(t, srv.Client(), srv.URL, "")
	defer joined.cancel()
	file, err := tree.Call(ctx, wire.Create, wire.Params{Namespace: "demo", Inode: 1, Name: "a", Mode: 0o644, Flags: syscall.O_RDWR})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := client.DialRaw(ctx, srv.URL, "demo")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	if _, err := raw.Write(ctx, file.Fh, 0, []byte("hi")); err != nil {
		t.Fatal(err)
	}
	if got := awaitEvict(t, joined.lines); got.Name != "a" {
		t.Fatalf("write %+v", got)
	}
}

func TestRawNoUpgrade(t *testing.T) {
	tree := local.New(t.TempDir())
	t.Cleanup(func() { _ = tree.Close() })
	hnd, _ := handler(tree, newHub())
	srv := httptest.NewServer(hnd)
	t.Cleanup(srv.Close)
	resp, err := http.Get(srv.URL + "/raw/demo")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestRawPoolAndCancel(t *testing.T) {
	tree := local.New(t.TempDir())
	t.Cleanup(func() { _ = tree.Close() })
	hnd, _ := handler(tree, newHub())
	srv := httptest.NewServer(hnd)
	t.Cleanup(srv.Close)
	ctx := context.Background()
	raw, err := client.DialRaw(ctx, srv.URL, "demo")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	file, err := tree.Call(ctx, wire.Create, wire.Params{Namespace: "demo", Inode: 1, Name: "a", Mode: 0o644, Flags: syscall.O_RDWR})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			buf := []byte{byte(i)}
			if _, err := raw.Write(ctx, file.Fh, int64(i), buf); err != nil {
				t.Errorf("write %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	dead, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := raw.ReadInto(dead, file.Attr.Ino, 0, 0, []byte{0}); err == nil {
		t.Fatal("cancelled read succeeded")
	}
	got := make([]byte, 1)
	if _, err := raw.ReadInto(ctx, file.Attr.Ino, 0, 0, got); err != nil {
		t.Fatal(err)
	}
}
