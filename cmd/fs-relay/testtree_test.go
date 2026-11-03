package main

import (
	"os"
	"path/filepath"
	"testing"

	"fs-relay/internal/local"
)

func newTestTree(t *testing.T) *local.Tree {
	t.Helper()
	tree, _ := newTestTreeDir(t)
	return tree
}

func newTestTreeDir(t *testing.T) (*local.Tree, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	tree := local.New(dir)
	t.Cleanup(func() { _ = tree.Close() })
	return tree, dir
}
