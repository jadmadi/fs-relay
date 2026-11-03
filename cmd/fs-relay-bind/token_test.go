package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fs-relay/internal/wire"

	"golang.org/x/sys/unix"
)

func TestResolveTokenFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token")
	if err := os.WriteFile(path, []byte("file-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := resolveToken("", path)
	if err != nil || got != "file-token" {
		t.Fatalf("file token %q %v", got, err)
	}
	if _, err := resolveToken("given-token", path); err == nil {
		t.Fatal("accepted both sources")
	}
	if _, err := resolveToken("", ""); err == nil || !strings.Contains(err.Error(), "pass -token or -token-file") {
		t.Fatalf("none: %v", err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveToken("", path); err == nil || !strings.Contains(err.Error(), "readable by other users") {
		t.Fatalf("mode: %v", err)
	}
	bad := filepath.Join(dir, "bad")
	if err := os.WriteFile(bad, []byte("bad+token"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveToken("", bad); !errors.Is(err, wire.ErrTokenChars) {
		t.Fatalf("chars: %v", err)
	}
	big := filepath.Join(dir, "big")
	if err := os.WriteFile(big, []byte(strings.Repeat("a", 4097)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveToken("", big); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("oversized: %v", err)
	}
}

func TestResolveTokenFileFIFO(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "fifo")
	if err := unix.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := resolveToken("", fifo)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("fifo: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("blocked on the FIFO")
	}
}
