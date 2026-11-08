package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestUpdateCheckCacheTTLAndFormat verifies natural-language subcommands,
// the 12-hour update check cache TTL, value summary formatting, and handle credit.
func TestUpdateCheckCacheTTLAndFormat(t *testing.T) {
	t.Setenv("FS_RELAY_NO_UPDATE_CHECK", "")
	for _, word := range []string{"update", "version", "help"} {
		if got, err := subcommand([]string{word}); err != nil || got != word {
			t.Fatalf("subcommand(%q) = (%q, %v)", word, got, err)
		}
	}
	if _, err := subcommand([]string{"update", "extra"}); err == nil {
		t.Fatal("expected error for extra argument after subcommand")
	}
	if done, err := dispatchCLI([]string{"update", "extra"}); !done || !errors.Is(err, errUsage) {
		t.Fatalf("dispatchCLI(update extra) = (%v, %v), want (true, errUsage)", done, err)
	}
	if updateCheckTTL != 12*time.Hour || compareSemver("v0.1.2", "v0.2.0") >= 0 {
		t.Fatal("unexpected TTL or semver ordering")
	}
	if compareSemver("v1.2.3-rc1", "v1.2.3") >= 0 || compareSemver("v1.2.3", "v1.2.3-rc1") <= 0 {
		t.Fatal("expected prerelease to be strictly older than final release")
	}
	if compareSemver("v1.2.3-rc1", "v1.2.3-rc2") >= 0 || compareSemver("v1.2.3", "v1.2.3") != 0 {
		t.Fatal("unexpected prerelease ordering")
	}
	if compareSemver("untagged-hash", "v1.2.3") != -2 {
		t.Fatal("expected -2 for unparseable semver comparison")
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

	failHits := 0
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		failHits++
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer failSrv.Close()
	failCache := filepath.Join(t.TempDir(), "fs-relay", "fail_check.json")
	failCfg := updateConfig{CurrentVersion: "v0.1.2", APIBase: failSrv.URL, CacheFile: failCache, Client: failSrv.Client()}
	_ = checkUpdateCached(context.Background(), failCfg)
	_ = checkUpdateCached(context.Background(), failCfg)
	if failHits != 1 {
		t.Fatalf("expected failed check to populate cache and not retry immediately, got %d hits", failHits)
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
	_ = os.WriteFile(relayBin, []byte("old-relay"), 0o700)
	_ = os.WriteFile(bindBin, []byte("old-bind"), 0o700)

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
	if fi, err := os.Stat(relayBin); err != nil || fi.Mode().Perm() != 0o700 {
		t.Fatalf("expected 0700 mode preserved on relayBin, got %v (%v)", fi.Mode().Perm(), err)
	}
	if !strings.Contains(out.String(), "Updated fs-relay v0.1.2 -> v0.2.0") {
		t.Fatalf("missing upgrade confirmation in %q", out.String())
	}

	_ = os.WriteFile(bindBin, []byte("stale-bind"), 0o700)
	compCfg := updateConfig{CurrentVersion: "v0.2.0", APIBase: okSrv.URL, ExePath: relayBin, Client: okSrv.Client(), Stdout: &out}
	if err := runSelfUpdate(context.Background(), compCfg); err != nil {
		t.Fatalf("runSelfUpdate for stale companion: %v", err)
	}
	if got, _ := os.ReadFile(bindBin); string(got) != "new-fs-relay-bind-binary" {
		t.Fatalf("stale companion was not upgraded: %q", got)
	}

	devCfg := updateConfig{CurrentVersion: "1af47ad", APIBase: okSrv.URL, ExePath: relayBin, Client: okSrv.Client(), Stdout: &out}
	if err := runSelfUpdate(context.Background(), devCfg); err != nil {
		t.Fatalf("runSelfUpdate for untagged build: %v", err)
	}

	t.Run("rollback", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("directory permission bits do not apply to root")
		}
		_ = os.WriteFile(relayBin, []byte("rollback-original-relay"), 0o700)
		roDir := t.TempDir()
		roBind := filepath.Join(roDir, "fs-relay-bind")
		_ = os.WriteFile(roBind, []byte("old-bind"), 0o700)
		_ = os.Chmod(roDir, 0o500)
		t.Cleanup(func() { _ = os.Chmod(roDir, 0o700) })
		err := applyBinaryUpdates(relayBin, roBind, map[string][]byte{
			"fs-relay":      []byte("partially-updated-relay"),
			"fs-relay-bind": []byte("new-bind"),
		})
		if err == nil {
			t.Fatal("expected companion update in read-only directory to fail")
		}
		if got, _ := os.ReadFile(relayBin); string(got) != "rollback-original-relay" {
			t.Fatalf("primary binary was not rolled back after companion failure: got %q", got)
		}
	})
}
