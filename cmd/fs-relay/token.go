package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"fs-relay/internal/wire"
)

func chooseToken(given string) (string, error) {
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
			rejectToken(w, "missing token")
			return
		}
		if !hmac.Equal([]byte(got), []byte(token)) {
			rejectToken(w, "wrong token")
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

func rejectToken(w http.ResponseWriter, why string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("WWW-Authenticate", wire.TokenScheme)
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(struct {
		Error wire.Error `json:"error"`
	}{Error: wire.Error{Code: http.StatusUnauthorized, Message: why}})
}
