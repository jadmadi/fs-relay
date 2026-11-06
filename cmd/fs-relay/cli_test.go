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

func TestCheckNoArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "empty", args: nil, want: ""},
		{name: "help-word", args: []string{"help"}, want: `unknown argument "help"`},
		{name: "version-word", args: []string{"version"}, want: `unknown argument "version"`},
		{name: "multiple", args: []string{"extra", "more"}, want: `unknown argument "extra"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkNoArgs(tc.args)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("checkNoArgs(%v) = %v, want nil", tc.args, err)
				}
				return
			}
			if err == nil || err.Error() != tc.want {
				t.Fatalf("checkNoArgs(%v) = %v, want %q", tc.args, err, tc.want)
			}
		})
	}
}
