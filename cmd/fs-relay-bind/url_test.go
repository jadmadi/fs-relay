package main

import "testing"

func TestNormalizeURL(t *testing.T) {
	tests := []struct{ in, want string }{
		{"127.0.0.1:8080", "http://127.0.0.1:8080"},
		{"localhost:8080", "http://localhost:8080"},
		{"http://127.0.0.1:8080", "http://127.0.0.1:8080"},
		{"https://example.test", "https://example.test"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := normalizeURL(tt.in); got != tt.want {
			t.Fatalf("%q -> %q, want %q", tt.in, got, tt.want)
		}
	}
}
