package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"fs-relay/internal/local"
	"fs-relay/internal/wire"
)

const testToken = "testtoken"

func TestSignClient(t *testing.T) {
	const id = "01234567-89ab-7def-8123-456789abcdef"
	got := signClient(testToken, id)
	const want = id + ".C7xMBPt9OEokZOE_IXQdv4dqM-tSfYIqgbeTRUj-BsI"
	if got != want || !clientSigned(testToken, got) {
		t.Fatalf("signed %s", got)
	}
	if clientSigned(testToken, id) || clientSigned("other-key", got) {
		t.Fatal("accepted an unsigned or foreign id")
	}
	flipped := got[:len(got)-1] + "A"
	if clientSigned(testToken, flipped) {
		t.Fatal("accepted a flipped signature")
	}
}

func TestNewToken(t *testing.T) {
	token, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := wire.CheckToken(token); err != nil {
		t.Fatal(err)
	}
	if len(token) != 64 {
		t.Fatalf("len %d", len(token))
	}
}

func TestRejectToken(t *testing.T) {
	if _, err := chooseToken("short"); !errors.Is(err, wire.ErrTokenShort) {
		t.Fatal(err)
	}
	if _, err := chooseToken("bad+token"); !errors.Is(err, wire.ErrTokenChars) {
		t.Fatal(err)
	}
	got, err := chooseToken("long-enough")
	if err != nil || got != "long-enough" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestTokenGate(t *testing.T) {
	tree := local.New(t.TempDir())
	t.Cleanup(func() { _ = tree.Close() })
	srv := httptest.NewServer(newHandler(tree, testToken))
	t.Cleanup(srv.Close)
	body, err := wire.MarshalRequest(wire.Mkdir, wire.Params{Namespace: "demo", Inode: 1, Name: "sub", Mode: 0o755})
	if err != nil {
		t.Fatal(err)
	}
	missing := postAs(t, srv.URL, body, "")
	if missing.code != http.StatusUnauthorized || missing.why != "missing token" {
		t.Fatalf("missing %+v", missing)
	}
	wrong := postAs(t, srv.URL, body, "not-the-one")
	if wrong.code != http.StatusUnauthorized || wrong.why != "wrong token" {
		t.Fatalf("wrong %+v", wrong)
	}
	folded := postAs(t, srv.URL, body, "TestToken")
	if folded.code != http.StatusUnauthorized || folded.why != "wrong token" {
		t.Fatalf("case %+v", folded)
	}
	ok := postAs(t, srv.URL, body, testToken)
	if ok.code != http.StatusOK {
		t.Fatalf("ok %d %s", ok.code, ok.why)
	}
}

func TestForgedClient(t *testing.T) {
	tree := local.New(t.TempDir())
	t.Cleanup(func() { _ = tree.Close() })
	srv := httptest.NewServer(newHandler(tree, testToken))
	t.Cleanup(srv.Close)
	bare := "01234567-89ab-7def-8123-456789abcdef"
	if code := joinStatus(t, srv.URL, bare); code != http.StatusBadRequest {
		t.Fatalf("bare %d", code)
	}
	signed := signClient(testToken, bare)
	flipped := signed[:len(signed)-1] + "A"
	if code := joinStatus(t, srv.URL, flipped); code != http.StatusBadRequest {
		t.Fatalf("flipped %d", code)
	}
	if code := joinStatus(t, srv.URL, signed); code != http.StatusOK {
		t.Fatalf("signed %d", code)
	}
}

func freshID(t *testing.T) string {
	t.Helper()
	raw, err := newID()
	if err != nil {
		t.Fatal(err)
	}
	return signClient(testToken, raw)
}

type gateReply struct {
	code int
	why  string
}

func postAs(t *testing.T, base string, body []byte, token string) gateReply {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/fs/demo", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", wire.ContentType)
	if token != "" {
		req.Header.Set(wire.TokenHeader, wire.TokenScheme+" "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var msg struct {
		Error wire.Error `json:"error"`
	}
	_ = json.Unmarshal(raw, &msg)
	if resp.StatusCode == http.StatusUnauthorized && msg.Error.Code != http.StatusUnauthorized {
		t.Fatalf("error code %d", msg.Error.Code)
	}
	return gateReply{code: resp.StatusCode, why: msg.Error.Message}
}

func joinStatus(t *testing.T, base, id string) int {
	t.Helper()
	body, err := wire.MarshalRequest(wire.Join, wire.Params{Namespace: "demo", Client: id})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, base+"/fs/demo", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", wire.ContentType)
	req.Header.Set(wire.TokenHeader, wire.TokenScheme+" "+testToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}
