package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"
)

// errClient is an id the server did not sign.
var errClient = errors.New("bad client")

// newID is a UUIDv7. The join stream uses it as the mount's client id.
func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	ms := uint64(time.Now().UnixMilli())
	binary.BigEndian.PutUint64(b[:8], ms<<16)
	b[6] = (b[6] & 0x0f) | 0x70
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// signClient is "<uuid>.<signature>". The signature is HMAC-SHA256 of the
// uuid under the token, as URL-safe base64 with the padding stripped.
func signClient(token, id string) string {
	return id + "." + clientSig(token, id)
}

func clientSig(token, id string) string {
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte(id))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func clientSigned(token, presented string) bool {
	id, sig, ok := strings.Cut(presented, ".")
	if !ok || !validID(id) || sig == "" {
		return false
	}
	return hmac.Equal([]byte(sig), []byte(clientSig(token, id)))
}

func validID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' {
			continue
		}
		return false
	}
	return true
}
