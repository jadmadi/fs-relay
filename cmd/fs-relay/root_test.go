package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRootNegotiates(t *testing.T) {
	tree := newTestTree(t)
	t.Cleanup(func() { tree.Close() })
	srv := httptest.NewServer(newHandler(tree, testToken))
	t.Cleanup(srv.Close)

	cases := []struct {
		name   string
		accept string
		json   bool
	}{
		{name: "missing", json: true},
		{name: "json", accept: "application/json", json: true},
		{name: "json wins", accept: "application/json, text/html;q=0.8", json: true},
		{name: "html", accept: "text/html", json: false},
		{name: "browser", accept: "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", json: false},
		{name: "star", accept: "*", json: true},
		{name: "curl", accept: "*/*", json: true},
		{name: "html and star", accept: "text/html, */*", json: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, srv.URL+"/", nil)
			if err != nil {
				t.Fatal(err)
			}
			if tc.accept != "" {
				req.Header.Set("Accept", tc.accept)
			}
			res, err := srv.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatal(err)
			}
			if res.StatusCode != http.StatusOK {
				t.Fatalf("status %d", res.StatusCode)
			}
			if tc.json {
				if !strings.Contains(res.Header.Get("Content-Type"), "application/json") {
					t.Fatalf("json %s", res.Header.Get("Content-Type"))
				}
				assertHealth(t, body)
				return
			}
			if !strings.Contains(res.Header.Get("Content-Type"), "text/html") || !strings.Contains(string(body), "Welcome to fs-relay server") {
				t.Fatalf("html %s", res.Header.Get("Content-Type"))
			}
		})
	}
}

func assertHealth(t *testing.T, body []byte) {
	t.Helper()
	var got health
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "ok" {
		t.Fatalf("status %q", got.Status)
	}
	if got.Version == "" {
		t.Fatal("health has no version")
	}
	parsed, err := time.Parse(time.RFC3339, got.Time)
	if err != nil || !strings.HasSuffix(got.Time, "Z") {
		t.Fatalf("time %q", got.Time)
	}
	if got.TimeMS/1000 != parsed.Unix() {
		t.Fatalf("time_ms %d time %s", got.TimeMS, got.Time)
	}
	if d := time.Since(time.UnixMilli(got.TimeMS)); d < -time.Second || d > 5*time.Second {
		t.Fatalf("skew %s", d)
	}
}
