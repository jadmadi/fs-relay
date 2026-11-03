package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"fs-relay/internal/wire"
)

// resolveToken returns the shared secret from -token or -token-file. The file
// is the safe source: an argv value is visible in ps to every local user.
func resolveToken(given, file string) (string, error) {
	if given != "" && file != "" {
		return "", fmt.Errorf("pass -token or -token-file, not both")
	}
	if file != "" {
		return readTokenFile(file)
	}
	if given == "" {
		return "", fmt.Errorf("pass -token or -token-file")
	}
	if err := wire.CheckToken(given); err != nil {
		return "", err
	}
	return given, nil
}

// readTokenFile reads a secret from a regular file only its user can read.
func readTokenFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("token file %s: %w", path, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("token file %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("token file %s is not a regular file", path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("token file %s is readable by other users; chmod 600 %s", path, path)
	}
	b, err := io.ReadAll(io.LimitReader(f, 4097))
	if err != nil {
		return "", fmt.Errorf("token file %s: %w", path, err)
	}
	if len(b) > 4096 {
		return "", fmt.Errorf("token file %s is too large", path)
	}
	token := strings.TrimSpace(string(b))
	if token == "" {
		return "", fmt.Errorf("token file %s is empty", path)
	}
	if err := wire.CheckToken(token); err != nil {
		return "", fmt.Errorf("token file %s: %w", path, err)
	}
	return token, nil
}
