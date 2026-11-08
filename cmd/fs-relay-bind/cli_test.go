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

func TestMountArg(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantMnt string
		wantBad string
		wantErr error
	}{
		{name: "single-mountpoint", args: []string{"/tmp/mnt"}, wantMnt: "/tmp/mnt"},
		{name: "flags-before-mountpoint", args: []string{"-ns", "demo", "/tmp/mnt"}, wantMnt: "/tmp/mnt"},
		{name: "no-positional", args: []string{}, wantErr: errUsage},
		{name: "empty-positional", args: []string{""}, wantErr: errUsage},
		{name: "flag-after-positional", args: []string{"/tmp/mnt", "-post"}, wantBad: "-post", wantErr: errUsage},
		{name: "extra-positional", args: []string{"/tmp/a", "/tmp/b"}, wantErr: errUsage},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fs := flag.NewFlagSet("fs-relay-bind", flag.ContinueOnError)
			_ = fs.String("ns", "default", "")
			_ = fs.Bool("post", false, "")
			if err := fs.Parse(tc.args); err != nil {
				t.Fatalf("Parse(%v): %v", tc.args, err)
			}
			mnt, bad, err := mountArg(fs)
			if mnt != tc.wantMnt || bad != tc.wantBad || err != tc.wantErr {
				t.Fatalf("mountArg(%v) = (%q, %q, %v), want (%q, %q, %v)",
					tc.args, mnt, bad, err, tc.wantMnt, tc.wantBad, tc.wantErr)
			}
		})
	}
}
