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
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	updateCheckTTL       = 12 * time.Hour
	defaultGitHubAPIBase = "https://api.github.com"
	releaseRepo          = "muayyad-alsadi/fs-relay"
	maxReleaseAssetBytes = 64 << 20
)

// subcommand extracts a natural-language command (update, version, help).
func subcommand(args []string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	switch args[0] {
	case "update", "version", "help":
		if len(args) > 1 {
			return "", fmt.Errorf("unknown argument %q", args[1])
		}
		return args[0], nil
	default:
		return "", nil
	}
}

func dispatchCLI(args []string) (bool, error) {
	if len(args) == 1 {
		switch args[0] {
		case "-update", "--update":
			args = []string{"update"}
		case "-version", "--version":
			args = []string{"version"}
		}
	}
	sub, err := subcommand(args)
	if err != nil {
		fs := flag.NewFlagSet("fs-relay", flag.ContinueOnError)
		fmt.Fprintf(os.Stderr, "%v\n\n%s", err, usageText(fs))
		return true, errUsage
	}
	if sub == "" {
		return false, nil
	}
	if sub == "help" {
		os.Args = []string{os.Args[0], "-h"}
		return false, nil
	}
	if sub == "version" {
		fmt.Println(versionLine("fs-relay"))
		runCachedCheck()
		return true, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return true, runSelfUpdate(ctx, defaultUpdateConfig())
}

func startBackgroundCheck(args []string) <-chan struct{} {
	done := make(chan struct{})
	for _, a := range args {
		if a == "-h" || a == "-help" || a == "--help" {
			close(done)
			return done
		}
	}
	go func() {
		defer close(done)
		runCachedCheck()
	}()
	return done
}

func runCachedCheck() {
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	_ = checkUpdateCached(ctx, defaultUpdateConfig())
}

var (
	reGoalAttr   = regexp.MustCompile(`(?:\(goals/[^)]+\)|\[goal:[^\]]+\])`)
	reCommitHash = regexp.MustCompile(`\b[0-9a-f]{7,40}\b`)
	reHandle     = regexp.MustCompile(`@[A-Za-z0-9-]+`)
)

type updateConfig struct {
	CurrentVersion string
	APIBase        string
	CacheFile      string
	ExePath        string
	GOOS           string
	GOARCH         string
	Client         *http.Client
	Stderr         io.Writer
	Stdout         io.Writer
}

type updateCache struct {
	CheckedAt    time.Time `json:"checked_at"`
	LatestTag    string    `json:"latest_tag"`
	ReleaseURL   string    `json:"release_url"`
	Summary      string    `json:"summary"`
	Contributors []string  `json:"contributors,omitempty"`
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Body    string `json:"body"`
	Author  struct {
		Login string `json:"login"`
	} `json:"author"`
	Assets []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

// defaultUpdateConfig builds the runtime configuration for version checks and self-update.
func defaultUpdateConfig() updateConfig {
	var cacheFile string
	if cacheRoot, err := os.UserCacheDir(); err == nil && cacheRoot != "" {
		cacheFile = filepath.Join(cacheRoot, "fs-relay", "update_check.json")
	}
	exe, _ := os.Executable()
	return updateConfig{
		CurrentVersion: buildVersion(),
		APIBase:        defaultGitHubAPIBase,
		CacheFile:      cacheFile,
		ExePath:        exe,
		GOOS:           runtime.GOOS,
		GOARCH:         runtime.GOARCH,
		Client:         &http.Client{Timeout: 60 * time.Second},
		Stderr:         os.Stderr,
		Stdout:         os.Stdout,
	}
}

func (c updateConfig) withDefaults() updateConfig {
	if c.APIBase == "" {
		c.APIBase = defaultGitHubAPIBase
	}
	if c.GOOS == "" {
		c.GOOS = runtime.GOOS
	}
	if c.GOARCH == "" {
		c.GOARCH = runtime.GOARCH
	}
	if c.Client == nil {
		c.Client = &http.Client{Timeout: 60 * time.Second}
	}
	if c.Stderr == nil {
		c.Stderr = io.Discard
	}
	if c.Stdout == nil {
		c.Stdout = io.Discard
	}
	return c
}

func isDevVersion(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" || v == "devel" || strings.HasPrefix(v, "(devel)") {
		return true
	}
	_, _, ok := parseSemver(v)
	return !ok
}

// compareSemver returns -1 when cur < latest, 0 when equal, and 1 when cur > latest.
// Unparseable versions return -2 to distinguish from equal.
func compareSemver(cur, latest string) int {
	a, preA, okA := parseSemver(cur)
	b, preB, okB := parseSemver(latest)
	if !okA || !okB {
		return -2
	}
	for i := 0; i < 3; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	if preA != "" && preB == "" {
		return -1
	}
	if preA == "" && preB != "" {
		return 1
	}
	if preA < preB {
		return -1
	}
	if preA > preB {
		return 1
	}
	return 0
}

func parseSemver(v string) ([3]int, string, bool) {
	v = strings.TrimSpace(strings.TrimPrefix(v, "v"))
	var prerelease string
	if idx := strings.Index(v, "-"); idx >= 0 {
		prerelease = v[idx+1:]
		v = v[:idx]
	}
	if idx := strings.Index(v, "+"); idx >= 0 {
		v = v[:idx]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return [3]int{}, "", false
	}
	var out [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return [3]int{}, "", false
		}
		out[i] = n
	}
	return out, prerelease, true
}

// formatValueSummary converts raw release notes into a plain-English benefit summary and contributor list.
func formatValueSummary(body, author string) (string, []string) {
	contributors := collectContributors(body, author)
	var benefits []string
	for _, raw := range strings.Split(body, "\n") {
		line := cleanReleaseLine(raw)
		if line == "" {
			continue
		}
		benefits = append(benefits, line)
		if len(benefits) == 3 {
			break
		}
	}
	if len(benefits) == 0 {
		return "Faster startup, stronger lock safety, and reliability improvements", contributors
	}
	return strings.Join(benefits, "; "), contributors
}

func cleanReleaseLine(raw string) string {
	line := strings.TrimSpace(raw)
	line = strings.TrimLeft(line, "#*-• ")
	if line == "" || strings.HasPrefix(line, "Full Changelog:") || strings.HasPrefix(line, "What's Changed") {
		return ""
	}
	line = reGoalAttr.ReplaceAllString(line, "")
	line = reCommitHash.ReplaceAllString(line, "")
	if idx := strings.Index(line, " by @"); idx > 0 {
		line = line[:idx]
	}
	return strings.Join(strings.Fields(line), " ")
}

func collectContributors(body, author string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(h string) {
		h = strings.TrimSpace(h)
		if h == "" || strings.Contains(strings.ToLower(h), "bot") {
			return
		}
		if !strings.HasPrefix(h, "@") {
			h = "@" + h
		}
		if !seen[h] {
			seen[h] = true
			out = append(out, h)
		}
	}
	for _, m := range reHandle.FindAllString(body, -1) {
		add(m)
	}
	add(author)
	if len(out) == 0 {
		add("@muayyad-alsadi")
	}
	return out
}

// checkUpdateCached checks for a newer release at most once every 12 hours and
// writes a value-focused upgrade notice to cfg.Stderr when a newer tag exists.
func checkUpdateCached(ctx context.Context, cfg updateConfig) error {
	cfg = cfg.withDefaults()
	if os.Getenv("FS_RELAY_NO_UPDATE_CHECK") != "" || isDevVersion(cfg.CurrentVersion) {
		return nil
	}
	if cached, err := readUpdateCache(cfg.CacheFile); err == nil && time.Since(cached.CheckedAt) < updateCheckTTL {
		printUpdateNotice(cfg.Stderr, cfg.CurrentVersion, cached)
		return nil
	}
	rel, err := fetchLatestRelease(ctx, cfg)
	if err != nil {
		_ = writeUpdateCache(cfg.CacheFile, updateCache{CheckedAt: time.Now().UTC()})
		return err
	}
	entry := cacheFromRelease(rel)
	_ = writeUpdateCache(cfg.CacheFile, entry)
	printUpdateNotice(cfg.Stderr, cfg.CurrentVersion, entry)
	return nil
}

func cacheFromRelease(rel githubRelease) updateCache {
	summary, contributors := formatValueSummary(rel.Body, rel.Author.Login)
	return updateCache{
		CheckedAt:    time.Now().UTC(),
		LatestTag:    rel.TagName,
		ReleaseURL:   rel.HTMLURL,
		Summary:      summary,
		Contributors: contributors,
	}
}

func printUpdateNotice(w io.Writer, current string, c updateCache) {
	if c.LatestTag == "" || compareSemver(current, c.LatestTag) >= 0 {
		return
	}
	credits := strings.Join(c.Contributors, ", ")
	fmt.Fprintf(w, "fs-relay: update available %s -> %s (%s)\n  Why upgrade: %s\n  Contributors: %s\n  Upgrade: fs-relay update\n",
		current, c.LatestTag, c.ReleaseURL, c.Summary, credits)
}

func readUpdateCache(path string) (updateCache, error) {
	if path == "" {
		return updateCache{}, errors.New("empty cache path")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return updateCache{}, err
	}
	var c updateCache
	if err := json.Unmarshal(b, &c); err != nil {
		return updateCache{}, err
	}
	return c, nil
}

func writeUpdateCache(path string, c updateCache) error {
	if path == "" {
		return nil
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	fi, err := os.Lstat(dir)
	if err != nil || !fi.IsDir() || fi.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("unsafe cache directory %s", dir)
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".update-check-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func fetchLatestRelease(ctx context.Context, cfg updateConfig) (githubRelease, error) {
	url := strings.TrimRight(cfg.APIBase, "/") + "/repos/" + releaseRepo + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return githubRelease{}, fmt.Errorf("release check: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := cfg.Client.Do(req)
	if err != nil {
		return githubRelease{}, fmt.Errorf("release check: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return githubRelease{}, fmt.Errorf("release check: HTTP %d", resp.StatusCode)
	}
	var rel githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&rel); err != nil {
		return githubRelease{}, fmt.Errorf("release check decode: %w", err)
	}
	if rel.TagName == "" {
		return githubRelease{}, errors.New("release check: empty tag_name")
	}
	return rel, nil
}

// runSelfUpdate checks the latest release, verifies the tarball SHA-256 digest,
// and atomically replaces cfg.ExePath (and sibling fs-relay-bind if present).
func runSelfUpdate(ctx context.Context, cfg updateConfig) error {
	cfg = cfg.withDefaults()
	if cfg.ExePath == "" {
		return errors.New("update: executable path unknown")
	}
	rel, err := fetchLatestRelease(ctx, cfg)
	if err != nil {
		_ = writeUpdateCache(cfg.CacheFile, updateCache{CheckedAt: time.Now().UTC()})
		return err
	}
	entry := cacheFromRelease(rel)
	_ = writeUpdateCache(cfg.CacheFile, entry)
	if !isDevVersion(cfg.CurrentVersion) && compareSemver(cfg.CurrentVersion, rel.TagName) >= 0 && !companionNeedsUpdate(ctx, cfg.ExePath, rel.TagName) {
		fmt.Fprintf(cfg.Stdout, "fs-relay %s is already up to date (%s)\n", cfg.CurrentVersion, rel.TagName)
		return nil
	}
	assetName := fmt.Sprintf("fs-relay-%s-%s.tar.gz", cfg.GOOS, cfg.GOARCH)
	assetURL, wantSum, err := resolveAssetAndChecksum(ctx, cfg, rel, assetName)
	if err != nil {
		return err
	}
	tarBytes, err := downloadVerifiedAsset(ctx, cfg, assetURL, wantSum)
	if err != nil {
		return err
	}
	bins, err := extractTarGzBinaries(tarBytes)
	if err != nil {
		return err
	}
	if err := applyBinaryUpdates(cfg.ExePath, bins); err != nil {
		return err
	}
	fmt.Fprintf(cfg.Stdout, "Updated fs-relay %s -> %s\n  Why upgrade: %s\n  Contributors: %s\n",
		cfg.CurrentVersion, rel.TagName, entry.Summary, strings.Join(entry.Contributors, ", "))
	return nil
}

func resolveAssetAndChecksum(ctx context.Context, cfg updateConfig, rel githubRelease, assetName string) (string, string, error) {
	var asset githubAsset
	var checksumURL string
	for _, a := range rel.Assets {
		if a.Name == assetName {
			asset = a
		}
		if a.Name == "checksums.txt" {
			checksumURL = a.BrowserDownloadURL
		}
	}
	if asset.BrowserDownloadURL == "" {
		return "", "", fmt.Errorf("update: release %s has no asset %s", rel.TagName, assetName)
	}
	if sum := strings.TrimPrefix(asset.Digest, "sha256:"); len(sum) == 64 {
		return asset.BrowserDownloadURL, strings.ToLower(sum), nil
	}
	if checksumURL == "" {
		return "", "", fmt.Errorf("update: release %s asset %s has no sha256 digest", rel.TagName, assetName)
	}
	sum, err := fetchChecksumFromFile(ctx, cfg, checksumURL, assetName)
	if err != nil {
		return "", "", err
	}
	return asset.BrowserDownloadURL, sum, nil
}

func fetchChecksumFromFile(ctx context.Context, cfg updateConfig, url, assetName string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("update checksums: %w", err)
	}
	resp, err := cfg.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("update checksums: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("update checksums: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return "", fmt.Errorf("update checksums: %w", err)
	}
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.TrimPrefix(fields[1], "*") == assetName && len(fields[0]) == 64 {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("update: %s not found in checksums.txt", assetName)
}

func downloadVerifiedAsset(ctx context.Context, cfg updateConfig, url, wantHex string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("update download: %w", err)
	}
	resp, err := cfg.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("update download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update download: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxReleaseAssetBytes))
	if err != nil {
		return nil, fmt.Errorf("update download: %w", err)
	}
	sum := sha256.Sum256(data)
	gotHex := hex.EncodeToString(sum[:])
	if gotHex != strings.ToLower(wantHex) {
		return nil, fmt.Errorf("update checksum mismatch: got %s, want %s", gotHex, wantHex)
	}
	return data, nil
}

func extractTarGzBinaries(tarGz []byte) (map[string][]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(tarGz))
	if err != nil {
		return nil, fmt.Errorf("update archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	bins := map[string][]byte{}
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("update archive: %w", err)
		}
		base := filepath.Base(hdr.Name)
		if base != "fs-relay" && base != "fs-relay-bind" {
			continue
		}
		b, err := io.ReadAll(io.LimitReader(tr, maxReleaseAssetBytes))
		if err != nil {
			return nil, fmt.Errorf("update archive %s: %w", base, err)
		}
		bins[base] = b
	}
	if len(bins["fs-relay"]) == 0 {
		return nil, errors.New("update archive: missing fs-relay binary")
	}
	return bins, nil
}

func companionPath(exePath string) string {
	if filepath.Base(exePath) == "fs-relay-bind" {
		return filepath.Join(filepath.Dir(exePath), "fs-relay")
	}
	return filepath.Join(filepath.Dir(exePath), "fs-relay-bind")
}

func companionNeedsUpdate(ctx context.Context, exePath, latestTag string) bool {
	sib := companionPath(exePath)
	info, err := os.Lstat(sib)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, sib, "-version").Output()
	if err != nil {
		return true
	}
	fields := strings.Fields(string(out))
	if len(fields) < 2 || isDevVersion(fields[1]) {
		return true
	}
	return compareSemver(fields[1], latestTag) < 0
}

func applyBinaryUpdates(exePath string, bins map[string][]byte) error {
	targetName := filepath.Base(exePath)
	primary := bins[targetName]
	if len(primary) == 0 {
		primary = bins["fs-relay"]
	}
	cleanupPrimary, rollbackPrimary, err := stageAndSwapBinary(exePath, primary)
	if err != nil {
		return err
	}
	sibling := companionPath(exePath)
	sibBytes := bins[filepath.Base(sibling)]
	if len(sibBytes) > 0 {
		if info, err := os.Lstat(sibling); err == nil && info.Mode().IsRegular() {
			cleanupSib, _, sibErr := stageAndSwapBinary(sibling, sibBytes)
			if sibErr != nil {
				rollbackPrimary()
				return sibErr
			}
			cleanupSib()
		}
	}
	cleanupPrimary()
	return nil
}

func stageAndSwapBinary(target string, content []byte) (func(), func(), error) {
	mode := os.FileMode(0o755)
	if info, err := os.Lstat(target); err == nil && info.Mode().IsRegular() {
		mode = info.Mode().Perm()
	}
	dir := filepath.Dir(target)
	base := "." + filepath.Base(target)
	newFile, err := os.CreateTemp(dir, base+".new-*")
	if err != nil {
		return nil, nil, fmt.Errorf("update write %s: %w", target, err)
	}
	tmp := newFile.Name()
	if _, err := newFile.Write(content); err != nil {
		_ = newFile.Close()
		_ = os.Remove(tmp)
		return nil, nil, fmt.Errorf("update write %s: %w", target, err)
	}
	if err := newFile.Chmod(mode); err != nil {
		_ = newFile.Close()
		_ = os.Remove(tmp)
		return nil, nil, fmt.Errorf("update chmod %s: %w", target, err)
	}
	if err := newFile.Close(); err != nil {
		_ = os.Remove(tmp)
		return nil, nil, fmt.Errorf("update close %s: %w", target, err)
	}
	oldFile, err := os.CreateTemp(dir, base+".old-*")
	if err != nil {
		_ = os.Remove(tmp)
		return nil, nil, fmt.Errorf("update backup %s: %w", target, err)
	}
	old := oldFile.Name()
	_ = oldFile.Close()
	if err := os.Rename(target, old); err != nil {
		_ = os.Remove(tmp)
		_ = os.Remove(old)
		return nil, nil, fmt.Errorf("update backup %s: %w", target, err)
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Rename(old, target)
		_ = os.Remove(tmp)
		_ = os.Remove(old)
		return nil, nil, fmt.Errorf("update replace %s: %w", target, err)
	}
	cleanup := func() { _ = os.Remove(old) }
	rollback := func() {
		_ = os.Rename(old, target)
		_ = os.Remove(old)
	}
	return cleanup, rollback, nil
}
