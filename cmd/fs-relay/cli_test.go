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
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"fs-relay/internal/local"
)

func TestVersionLine(t *testing.T) {
	version = "v1.2.3"
	defer func() { version = "" }()
	if got := versionLine("fs-relay"); got != "fs-relay v1.2.3" {
		t.Fatalf("version %q", got)
	}
	version = ""
	if got := versionLine("fs-relay"); !strings.HasPrefix(got, "fs-relay ") || len(got) < len("fs-relay x") {
		t.Fatalf("fallback version %q", got)
	}
}

func TestUsageTextVersion(t *testing.T) {
	version = "v1.2.3"
	defer func() { version = "" }()
	fs := flag.NewFlagSet("fs-relay", flag.ContinueOnError)
	if got := usageText(fs); !strings.Contains(got, "Version:\n  fs-relay v1.2.3") {
		t.Fatalf("usage missing version:\n%s", got)
	}
}

func TestCheckNoArgs(t *testing.T) {
	t.Setenv("FS_RELAY_NO_UPDATE_CHECK", "1")
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "empty", args: nil, want: ""},
		{name: "help-word", args: []string{"help"}, want: `unknown argument "help"`},
		{name: "version-word", args: []string{"version"}, want: `unknown argument "version"`},
		{name: "multiple", args: []string{"extra", "more"}, want: `unknown argument "extra"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkNoArgs(tc.args)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("checkNoArgs(%v) = %v, want nil", tc.args, err)
				}
				return
			}
			if err == nil || err.Error() != tc.want {
				t.Fatalf("checkNoArgs(%v) = %v, want %q", tc.args, err, tc.want)
			}
		})
	}
}

// TestForceMultiServeFlag verifies that run refuses an already-locked data
// directory by default and starts cleanly when -force-multi-serve is set.
func TestForceMultiServeFlag(t *testing.T) {
	t.Setenv("FS_RELAY_NO_UPDATE_CHECK", "1")
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })

	root := filepath.Join(t.TempDir(), "data")
	first := local.New(root)
	t.Cleanup(func() { _ = first.Close() })
	if err := first.Prepare(); err != nil {
		t.Fatal(err)
	}
	if err := first.Lock(); err != nil {
		t.Fatal(err)
	}

	os.Args = []string{"fs-relay", "-data", root, "-token", "local-demo", "-listen", "127.0.0.1:0"}
	if err := run(); err == nil || !strings.Contains(err.Error(), "is already served by pid ") {
		t.Fatalf("run without -force-multi-serve = %v, want already served by pid error", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	os.Args = []string{"fs-relay", "-data", root, "-token", "local-demo", "-listen", addr, "-force-multi-serve"}
	errc := make(chan error, 1)
	go func() { errc <- run() }()
	for {
		resp, err := http.Get("http://" + addr + "/")
		if err == nil {
			_ = resp.Body.Close()
			break
		}
		select {
		case runErr := <-errc:
			t.Fatalf("run exited early: %v", runErr)
		default:
			runtime.Gosched()
		}
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if err := <-errc; err != nil {
		t.Fatalf("run with -force-multi-serve: %v", err)
	}
}

// TestUpdateCheckCacheTTLAndFormat verifies natural-language subcommands,
// the 12-hour update check cache TTL, value summary formatting, and handle credit.
func TestUpdateCheckCacheTTLAndFormat(t *testing.T) {
	for _, word := range []string{"update", "version", "help"} {
		if got, err := subcommand([]string{word}); err != nil || got != word {
			t.Fatalf("subcommand(%q) = (%q, %v)", word, got, err)
		}
	}
	if _, err := subcommand([]string{"update", "extra"}); err == nil {
		t.Fatal("expected error for extra argument after subcommand")
	}
	if updateCheckTTL != 12*time.Hour || compareSemver("v0.1.2", "v0.2.0") >= 0 {
		t.Fatal("unexpected TTL or semver ordering")
	}

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		var rel githubRelease
		rel.TagName = "v0.2.0"
		rel.Body = "* 640e881 Prevent concurrent server corruption by @jadmadi [goal: single-serve-data-lock]\n* Faster raw socket reads by @muayyad-alsadi\n"
		rel.Author.Login = "muayyad-alsadi"
		_ = json.NewEncoder(w).Encode(rel)
	}))
	defer srv.Close()

	cacheFile := filepath.Join(t.TempDir(), "fs-relay", "update_check.json")
	var out bytes.Buffer
	cfg := updateConfig{CurrentVersion: "v0.1.2", APIBase: srv.URL, CacheFile: cacheFile, Client: srv.Client(), Stderr: &out}

	if err := checkUpdateCached(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "Upgrade: fs-relay update") || !strings.Contains(text, "@jadmadi") || strings.Contains(text, "640e881") {
		t.Fatalf("unexpected update notice: %q", text)
	}
	if err := checkUpdateCached(context.Background(), cfg); err != nil || hits.Load() != 1 {
		t.Fatalf("expected cached hit within 12h, hits=%d err=%v", hits.Load(), err)
	}
	cached, _ := readUpdateCache(cacheFile)
	cached.CheckedAt = time.Now().UTC().Add(-13 * time.Hour)
	_ = writeUpdateCache(cacheFile, cached)
	if err := checkUpdateCached(context.Background(), cfg); err != nil || hits.Load() != 2 {
		t.Fatalf("expected refresh after 12h, hits=%d err=%v", hits.Load(), err)
	}
}

func makeReleaseTarGz(t *testing.T, files map[string][]byte) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, data := range files {
		_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(data))})
		_, _ = tw.Write(data)
	}
	_ = tw.Close()
	_ = gz.Close()
	raw := buf.Bytes()
	sum := sha256.Sum256(raw)
	return raw, hex.EncodeToString(sum[:])
}

func newMockReleaseServer(t *testing.T, tarName string, tarBytes []byte, digest, checksumBody string) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/muayyad-alsadi/fs-relay/releases/latest":
			var rel githubRelease
			rel.TagName = "v0.2.0"
			rel.Body = "- Atomic self-update with SHA-256 verification by @jadmadi\n"
			rel.Author.Login = "muayyad-alsadi"
			rel.Assets = []githubAsset{
				{Name: tarName, BrowserDownloadURL: srv.URL + "/" + tarName, Digest: digest},
				{Name: "checksums.txt", BrowserDownloadURL: srv.URL + "/checksums.txt"},
			}
			_ = json.NewEncoder(w).Encode(rel)
		case "/" + tarName:
			_, _ = w.Write(tarBytes)
		case "/checksums.txt":
			_, _ = w.Write([]byte(checksumBody))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestSelfUpdateAtomicSwapAndChecksum verifies atomic binary replacement,
// SHA-256 verification via asset digest and checksums.txt, and mismatch rejection.
func TestSelfUpdateAtomicSwapAndChecksum(t *testing.T) {
	tarName := "fs-relay-" + runtime.GOOS + "-" + runtime.GOARCH + ".tar.gz"
	tarBytes, tarSum := makeReleaseTarGz(t, map[string][]byte{
		"fs-relay":      []byte("new-fs-relay-binary"),
		"fs-relay-bind": []byte("new-fs-relay-bind-binary"),
	})

	dir := t.TempDir()
	relayBin := filepath.Join(dir, "fs-relay")
	bindBin := filepath.Join(dir, "fs-relay-bind")
	_ = os.WriteFile(relayBin, []byte("old-relay"), 0o755)
	_ = os.WriteFile(bindBin, []byte("old-bind"), 0o755)

	badSrv := newMockReleaseServer(t, tarName, tarBytes, "sha256:"+strings.Repeat("0", 64), "")
	badCfg := updateConfig{CurrentVersion: "v0.1.2", APIBase: badSrv.URL, ExePath: relayBin, Client: badSrv.Client()}
	if err := runSelfUpdate(context.Background(), badCfg); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch error, got %v", err)
	}
	if got, _ := os.ReadFile(relayBin); string(got) != "old-relay" {
		t.Fatalf("corrupted update modified binary: %q", got)
	}

	okSrv := newMockReleaseServer(t, tarName, tarBytes, "", tarSum+"  "+tarName+"\n")
	var out bytes.Buffer
	okCfg := updateConfig{CurrentVersion: "v0.1.2", APIBase: okSrv.URL, ExePath: relayBin, Client: okSrv.Client(), Stdout: &out}
	if err := runSelfUpdate(context.Background(), okCfg); err != nil {
		t.Fatalf("runSelfUpdate: %v", err)
	}
	gotRelay, _ := os.ReadFile(relayBin)
	gotBind, _ := os.ReadFile(bindBin)
	if string(gotRelay) != "new-fs-relay-binary" || string(gotBind) != "new-fs-relay-bind-binary" {
		t.Fatalf("binaries = (%q, %q)", gotRelay, gotBind)
	}
	if !strings.Contains(out.String(), "Updated fs-relay v0.1.2 -> v0.2.0") {
		t.Fatalf("missing upgrade confirmation in %q", out.String())
	}
}
