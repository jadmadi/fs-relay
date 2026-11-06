package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"time"
)

// version is set at build time with -ldflags "-X main.version=<tag>".
var version string

func buildVersion() string {
	if version != "" {
		return version
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "(devel)"
	}
	v := bi.Main.Version
	if v == "" {
		v = "(devel)"
	}
	rev := ""
	for _, s := range bi.Settings {
		if s.Key == "vcs.revision" {
			rev = s.Value
		}
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	if rev != "" {
		return v + " (" + rev + ")"
	}
	return v
}

func versionLine(name string) string {
	return name + " " + buildVersion()
}

// checkNoArgs rejects unexpected positional arguments passed to fs-relay.
func checkNoArgs(args []string) error {
	if len(args) == 0 {
		runCachedCheck()
		return nil
	}
	return fmt.Errorf("unknown argument %q", args[0])
}

// subcommand extracts a natural-language command (update, version, help) or
// returns an unknown-argument error when unexpected positionals are present.
func subcommand(args []string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	switch args[0] {
	case "update", "version", "help":
		if len(args) > 1 {
			return "", fmt.Errorf("unknown argument %q", args[1])
		}
		return args[0], nil
	default:
		return "", checkNoArgs(args)
	}
}

func dispatchCLI(args []string) (bool, error) {
	if len(args) == 1 && (args[0] == "-update" || args[0] == "--update") {
		args = []string{"update"}
	}
	sub, err := subcommand(args)
	if err != nil || sub == "" {
		return false, nil
	}
	if sub == "help" {
		os.Args = []string{os.Args[0], "-h"}
		return false, nil
	}
	if sub == "version" {
		runCachedCheck()
		os.Args = []string{os.Args[0], "-version"}
		return false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return true, runSelfUpdate(ctx, defaultUpdateConfig())
}

func runCachedCheck() {
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	_ = checkUpdateCached(ctx, defaultUpdateConfig())
}

func usageText(fs *flag.FlagSet) string {
	var b strings.Builder
	b.WriteString("fs-relay serves one directory over HTTP for remote FUSE mounts.\n\n")
	b.WriteString("Usage:\n  fs-relay [-listen addr] [-data dir] [-token secret]\n  fs-relay update | version | help\n\n")
	b.WriteString("Example:\n  fs-relay -listen 127.0.0.1:8080 -data data -token local-demo\n\n")
	b.WriteString("Version:\n  ")
	b.WriteString(versionLine("fs-relay"))
	b.WriteString("\n\nFlags:\n")
	fs.SetOutput(&b)
	fs.PrintDefaults()
	fs.SetOutput(os.Stderr)
	return b.String()
}
