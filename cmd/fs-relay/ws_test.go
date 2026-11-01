package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"

	"fs-relay/internal/client"
	"fs-relay/internal/local"
	"fs-relay/internal/wire"

	"net/http/httptest"
)

func TestWSMkdir(t *testing.T) {
	dir := t.TempDir()
	tree := local.New(dir)
	t.Cleanup(func() { _ = tree.Close() })
	srv := httptest.NewServer(newHandler(tree, testToken))
	t.Cleanup(srv.Close)
	sock, err := client.Dial(context.Background(), srv.URL, "demo", testToken)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sock.Close() })

	var wg sync.WaitGroup
	wg.Add(2)
	for _, name := range []string{"a", "b"} {
		go func(name string) {
			defer wg.Done()
			_, err := sock.Call(context.Background(), wire.Mkdir, wire.Params{Inode: 1, Name: name, Mode: 0o755})
			if err != nil {
				t.Error(err)
			}
		}(name)
	}
	wg.Wait()
	for _, name := range []string{"a", "b"} {
		if _, err := os.Stat(filepath.Join(dir, "demo", name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}

	_, err = sock.Call(context.Background(), wire.Lookup, wire.Params{Inode: 1, Name: "missing"})
	var werr *wire.Error
	if !errors.As(err, &werr) || werr.Code != uint32(syscall.ENOENT) {
		t.Fatalf("%v", err)
	}
	_, err = sock.Call(context.Background(), "nope", wire.Params{Inode: 1})
	if err == nil || errors.As(err, &werr) {
		t.Fatalf("unknown method: %v", err)
	}
}
