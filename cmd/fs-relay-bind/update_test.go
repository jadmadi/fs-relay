package main

import (
	"flag"
	"testing"
)

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
