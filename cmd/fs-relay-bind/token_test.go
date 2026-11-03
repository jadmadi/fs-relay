package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fs-relay/internal/wire"
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
