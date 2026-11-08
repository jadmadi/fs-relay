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
	"runtime"
	"strings"
	"time"
)

const (
	bindUpdateRepo       = "muayyad-alsadi/fs-relay"
	bindUpdateMaxBytes   = 128 << 20
	bindDefaultGitHubAPI = "https://api.github.com"
)

type bindUpdateConfig struct {
	CurrentVersion string
	APIBase        string
	ExePath        string
	Stdout         io.Writer
	Client         *http.Client
}

type bindRelease struct {
	TagName string      `json:"tag_name"`
	Assets  []bindAsset `json:"assets"`
}

type bindAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

// bindSubcommand returns "update", "version", or "help" when invoked as a bare
// single-word command with no flags, or "" when arguments represent a mountpoint.
func bindSubcommand(fs *flag.FlagSet) string {
	if fs.NFlag() != 0 || fs.NArg() != 1 {
		return ""
	}
	switch fs.Arg(0) {
	case "update", "version", "help":
		return fs.Arg(0)
	default:
		return ""
	}
}

func dispatchBindCLI(flags *flag.FlagSet, showVersion, doUpdate bool) (bool, error) {
	sub := bindSubcommand(flags)
	if sub == "help" {
		fmt.Print(usageText(flags))
		return true, nil
	}
	if showVersion || sub == "version" {
		fmt.Println(versionLine("fs-relay-bind"))
		return true, nil
	}
	if doUpdate || sub == "update" {
		return true, runBindUpdate()
	}
	return false, nil
}

func runBindUpdate() error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("update executable path: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}
	sibling := filepath.Join(filepath.Dir(self), "fs-relay")
	if info, err := os.Lstat(sibling); err == nil && info.Mode().IsRegular() {
		cmd := exec.Command(sibling, "update")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	return runBindSelfUpdate(ctx, bindUpdateConfig{
		CurrentVersion: version,
		APIBase:        bindDefaultGitHubAPI,
		ExePath:        self,
		Stdout:         os.Stdout,
		Client:         &http.Client{Timeout: 60 * time.Second},
	})
}

func runBindSelfUpdate(ctx context.Context, cfg bindUpdateConfig) error {
	if cfg.APIBase == "" {
		cfg.APIBase = bindDefaultGitHubAPI
	}
	if cfg.Stdout == nil {
		cfg.Stdout = os.Stdout
	}
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 60 * time.Second}
	}
	rel, err := fetchBindLatestRelease(ctx, cfg)
	if err != nil {
		return err
	}
	assetName := fmt.Sprintf("fs-relay-%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	assetURL, wantSum, err := resolveBindAssetChecksum(ctx, cfg, rel, assetName)
	if err != nil {
		return err
	}
	tarBytes, err := fetchVerifiedBindArchive(ctx, cfg, assetURL, wantSum)
	if err != nil {
		return err
	}
	binBytes, err := extractBindBinary(tarBytes)
	if err != nil {
		return err
	}
	if err := swapBindBinary(cfg.ExePath, binBytes); err != nil {
		return err
	}
	fmt.Fprintf(cfg.Stdout, "Updated fs-relay-bind %s -> %s\n", cfg.CurrentVersion, rel.TagName)
	return nil
}

func fetchBindLatestRelease(ctx context.Context, cfg bindUpdateConfig) (bindRelease, error) {
	url := strings.TrimRight(cfg.APIBase, "/") + "/repos/" + bindUpdateRepo + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return bindRelease{}, fmt.Errorf("update request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := cfg.Client.Do(req)
	if err != nil {
		return bindRelease{}, fmt.Errorf("update check failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return bindRelease{}, fmt.Errorf("update check failed: HTTP %d", resp.StatusCode)
	}
	var rel bindRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return bindRelease{}, fmt.Errorf("update decode: %w", err)
	}
	return rel, nil
}

func resolveBindAssetChecksum(ctx context.Context, cfg bindUpdateConfig, rel bindRelease, assetName string) (string, string, error) {
	var asset bindAsset
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
	sum, err := fetchBindChecksum(ctx, cfg, checksumURL, assetName)
	if err != nil {
		return "", "", err
	}
	return asset.BrowserDownloadURL, sum, nil
}

func fetchBindChecksum(ctx context.Context, cfg bindUpdateConfig, url, assetName string) (string, error) {
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

func fetchVerifiedBindArchive(ctx context.Context, cfg bindUpdateConfig, url, wantHex string) ([]byte, error) {
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
	data, err := io.ReadAll(io.LimitReader(resp.Body, bindUpdateMaxBytes))
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

func extractBindBinary(tarGz []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(tarGz))
	if err != nil {
		return nil, fmt.Errorf("update archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("update archive: %w", err)
		}
		if filepath.Base(hdr.Name) != "fs-relay-bind" {
			continue
		}
		b, err := io.ReadAll(io.LimitReader(tr, bindUpdateMaxBytes))
		if err != nil {
			return nil, fmt.Errorf("update archive fs-relay-bind: %w", err)
		}
		return b, nil
	}
	return nil, errors.New("update archive: missing fs-relay-bind binary")
}

func swapBindBinary(target string, content []byte) error {
	mode := os.FileMode(0o755)
	if info, err := os.Lstat(target); err == nil && info.Mode().IsRegular() {
		mode = info.Mode().Perm()
	}
	dir := filepath.Dir(target)
	base := "." + filepath.Base(target)
	newFile, err := os.CreateTemp(dir, base+".new-*")
	if err != nil {
		return fmt.Errorf("update write %s: %w", target, err)
	}
	tmp := newFile.Name()
	if _, err := newFile.Write(content); err != nil {
		_ = newFile.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("update write %s: %w", target, err)
	}
	if err := newFile.Chmod(mode); err != nil {
		_ = newFile.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("update chmod %s: %w", target, err)
	}
	if err := newFile.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("update close %s: %w", target, err)
	}
	oldFile, err := os.CreateTemp(dir, base+".old-*")
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("update backup %s: %w", target, err)
	}
	old := oldFile.Name()
	_ = oldFile.Close()
	if err := os.Rename(target, old); err != nil {
		_ = os.Remove(tmp)
		_ = os.Remove(old)
		return fmt.Errorf("update backup %s: %w", target, err)
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Rename(old, target)
		_ = os.Remove(tmp)
		_ = os.Remove(old)
		return fmt.Errorf("update replace %s: %w", target, err)
	}
	_ = os.Remove(old)
	return nil
}
