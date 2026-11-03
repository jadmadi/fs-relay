package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"fs-relay/internal/wire"
)

func chooseToken(given, file string) (string, error) {
	if given != "" && file != "" {
		return "", fmt.Errorf("pass -token or -token-file, not both")
	}
	if file != "" {
		return readTokenFile(file)
	}
	if given == "" {
		token, err := newToken()
		if err != nil {
			return "", err
		}
		fmt.Fprintf(os.Stderr, "fs-relay: token %s\n", token)
		return token, nil
	}
	if err := wire.CheckToken(given); err != nil {
		return "", err
	}
	return given, nil
}

// readTokenFile reads a secret from a regular file only its user can read.
// The mode check is the point: a token in argv is visible in ps, while a
// mode-600 file is not.
func readTokenFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("token file %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("token file %s is not a regular file", path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("token file %s is readable by other users; chmod 600 %s", path, path)
	}
	if info.Size() > 4096 {
		return "", fmt.Errorf("token file %s is too large", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("token file %s: %w", path, err)
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

// newToken is HMAC-SHA256(key=UUIDv7, msg=pid), as hex.
func newToken() (string, error) {
	key, err := newID()
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(strconv.Itoa(os.Getpid())))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func withToken(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			next.ServeHTTP(w, r)
			return
		}
		got, ok := bearer(r.Header.Get(wire.TokenHeader))
		if !ok {
			rejectToken(w, r, "missing token")
			return
		}
		if !hmac.Equal([]byte(got), []byte(token)) {
			rejectToken(w, r, "wrong token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearer(v string) (string, bool) {
	rest, ok := strings.CutPrefix(v, wire.TokenScheme+" ")
	if !ok || rest == "" {
		return "", false
	}
	return rest, true
}

func rejectToken(w http.ResponseWriter, r *http.Request, why string) {
	log.Printf("fs-relay: %s from %s", why, clientIP(r))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("WWW-Authenticate", wire.TokenScheme)
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(struct {
		Error wire.Error `json:"error"`
	}{Error: wire.Error{Code: http.StatusUnauthorized, Message: why}})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
