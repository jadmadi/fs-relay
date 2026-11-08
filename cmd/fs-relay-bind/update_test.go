package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBindSubcommands(t *testing.T) {
	check := func(args []string, want string) {
		t.Helper()
		fs := flag.NewFlagSet("fs-relay-bind", flag.ContinueOnError)
		_ = fs.String("ns", "default", "")
		if err := fs.Parse(args); err != nil {
			t.Fatalf("Parse(%v): %v", args, err)
		}
		if got := bindSubcommand(fs); got != want {
			t.Fatalf("bindSubcommand(%v) = %q, want %q", args, got, want)
		}
	}
	check([]string{"update"}, "update")
	check([]string{"version"}, "version")
	check([]string{"help"}, "help")
	check([]string{"/tmp/mnt"}, "")
	check([]string{"-ns", "demo", "version"}, "")
}

func TestRunBindSelfUpdateClientOnly(t *testing.T) {
	var tarBuf bytes.Buffer
	gz := gzip.NewWriter(&tarBuf)
	tw := tar.NewWriter(gz)
	wantBody := []byte("standalone-updated-bind-binary")
	_ = tw.WriteHeader(&tar.Header{Name: "fs-relay-bind", Mode: 0o755, Size: int64(len(wantBody))})
	_, _ = tw.Write(wantBody)
	_ = tw.Close()
	_ = gz.Close()
	tarBytes := tarBuf.Bytes()
	sum := sha256.Sum256(tarBytes)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	tarName := "fs-relay-" + runtime.GOOS + "-" + runtime.GOARCH + ".tar.gz"

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/muayyad-alsadi/fs-relay/releases/latest":
			_ = json.NewEncoder(w).Encode(bindRelease{
				TagName: "v0.2.0",
				Assets: []bindAsset{
					{Name: tarName, BrowserDownloadURL: srv.URL + "/" + tarName, Digest: digest},
				},
			})
		case "/" + tarName:
			_, _ = w.Write(tarBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	bindBin := filepath.Join(dir, "fs-relay-bind")
	_ = os.WriteFile(bindBin, []byte("old-bind"), 0o700)

	var out bytes.Buffer
	err := runBindSelfUpdate(context.Background(), bindUpdateConfig{
		CurrentVersion: "v0.1.2",
		APIBase:        srv.URL,
		ExePath:        bindBin,
		Stdout:         &out,
		Client:         srv.Client(),
	})
	if err != nil {
		t.Fatalf("runBindSelfUpdate: %v", err)
	}
	got, _ := os.ReadFile(bindBin)
	if string(got) != string(wantBody) {
		t.Fatalf("bindBin = %q, want %q", got, wantBody)
	}
	if fi, err := os.Stat(bindBin); err != nil || fi.Mode().Perm() != 0o700 {
		t.Fatalf("expected 0700 mode preserved, got %v (%v)", fi.Mode().Perm(), err)
	}
	if !strings.Contains(out.String(), "Updated fs-relay-bind v0.1.2 -> v0.2.0") {
		t.Fatalf("missing upgrade output in %q", out.String())
	}
}
