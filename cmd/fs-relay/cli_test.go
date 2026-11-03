package main

import (
	"flag"
	"strings"
	"testing"
)

func TestVersionLine(t *testing.T) {
	version = "v1.2.3"
	defer func() { version = "" }()
	if got := versionLine("fs-relay"); got != "fs-relay v1.2.3" {
		t.Fatalf("version %q", got)
	}
	version = ""
	if got := versionLine("fs-relay"); !strings.HasPrefix(got, "fs-relay ") || len(got) < len("fs-relay x") {
		t.Fatalf("fallback version %q", got)
	}
}

func TestUsageTextVersion(t *testing.T) {
	version = "v1.2.3"
	defer func() { version = "" }()
	fs := flag.NewFlagSet("fs-relay", flag.ContinueOnError)
	if got := usageText(fs); !strings.Contains(got, "Version:\n  fs-relay v1.2.3") {
		t.Fatalf("usage missing version:\n%s", got)
	}
}
