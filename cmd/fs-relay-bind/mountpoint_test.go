package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckMountpoint(t *testing.T) {
	dir := t.TempDir()
	if err := checkMountpoint(dir, false); err != nil {
		t.Fatalf("empty: %v", err)
	}
	missing := filepath.Join(dir, "missing")
	if err := checkMountpoint(missing, false); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("missing: %v", err)
	}
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkMountpoint(file, false); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("file: %v", err)
	}
	nonEmpty := filepath.Join(dir, "full")
	if err := os.Mkdir(nonEmpty, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nonEmpty, "x"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkMountpoint(nonEmpty, false); err == nil || !strings.Contains(err.Error(), "-force-existing-mnt") {
		t.Fatalf("non-empty: %v", err)
	}
	if err := checkMountpoint(nonEmpty, true); err != nil {
		t.Fatalf("forced: %v", err)
	}
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o111); err != nil {
		t.Fatal(err)
	}
	if err := checkMountpoint(locked, true); err != nil {
		t.Fatalf("forced unreadable: %v", err)
	}
	if err := checkMountpoint(locked, false); err == nil {
		t.Fatal("unreadable directory accepted without force")
	}
}
