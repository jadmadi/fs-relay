package main

import (
	"flag"
	"strings"
	"testing"
)

func TestVersionLine(t *testing.T) {
	version = "v1.2.3"
	defer func() { version = "" }()
	if got := versionLine("fs-relay-bind"); got != "fs-relay-bind v1.2.3" {
		t.Fatalf("version %q", got)
	}
	version = ""
	if got := versionLine("fs-relay-bind"); !strings.HasPrefix(got, "fs-relay-bind ") {
		t.Fatalf("fallback version %q", got)
	}
}

func TestUsageTextVersion(t *testing.T) {
	version = "v1.2.3"
	defer func() { version = "" }()
	fs := flag.NewFlagSet("fs-relay-bind", flag.ContinueOnError)
	if got := usageText(fs); !strings.Contains(got, "Version:\n  fs-relay-bind v1.2.3") {
		t.Fatalf("usage missing version:\n%s", got)
	}
}

func TestMisplacedFlag(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{}, ""},
		{[]string{"mnt"}, ""},
		{[]string{"mnt", "-post"}, "-post"},
		{[]string{"mnt", "extra"}, ""},
		{[]string{"mnt", "extra", "-post"}, "-post"},
	}
	for _, tt := range tests {
		if got := misplacedFlag(tt.args); got != tt.want {
			t.Fatalf("%v -> %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestBindSubcommands(t *testing.T) {
	check := func(args []string, want string) {
		t.Helper()
		fs := flag.NewFlagSet("fs-relay-bind", flag.ContinueOnError)
		_ = fs.String("ns", "default", "")
		if err := fs.Parse(args); err != nil {
			t.Fatalf("Parse(%v): %v", args, err)
		}
		if got := bindSubcommand(fs); got != want {
			t.Fatalf("bindSubcommand(%v) = %q, want %q", args, got, want)
		}
	}
	check([]string{"update"}, "update")
	check([]string{"version"}, "version")
	check([]string{"help"}, "help")
	check([]string{"/tmp/mnt"}, "")
	check([]string{"-ns", "demo", "version"}, "")
}
