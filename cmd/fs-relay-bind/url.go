package main

import "strings"

// normalizeURL gives a bare host:port the http scheme, as curl does. An
// explicit scheme is left alone, including one this tool will later refuse.
func normalizeURL(raw string) string {
	if raw == "" || strings.Contains(raw, "://") {
		return raw
	}
	return "http://" + raw
}
