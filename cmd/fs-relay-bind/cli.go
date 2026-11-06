package main

import (
	"flag"
	"os"
	"runtime/debug"
	"strings"
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

func misplacedFlag(args []string) string {
	if len(args) < 2 {
		return ""
	}
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-") {
			return a
		}
	}
	return ""
}

// mountArg extracts the single mountpoint positional argument from a parsed
// FlagSet, returning any misplaced trailing flag and errUsage when missing or extra.
func mountArg(fs *flag.FlagSet) (string, string, error) {
	if fs.NArg() != 1 || fs.Arg(0) == "" {
		return "", misplacedFlag(fs.Args()), errUsage
	}
	return fs.Arg(0), "", nil
}

func usageText(fs *flag.FlagSet) string {
	var b strings.Builder
	b.WriteString("fs-relay-bind mounts an fs-relay namespace with FUSE.\n\n")
	b.WriteString("Usage:\n  fs-relay-bind [flags] <mountpoint>\n\n")
	b.WriteString("Example:\n  fs-relay-bind -url http://127.0.0.1:8080 -ns demo -token local-demo /mnt/fs-relay\n\n")
	b.WriteString("Version:\n  ")
	b.WriteString(versionLine("fs-relay-bind"))
	b.WriteString("\n\nFlags:\n")
	fs.SetOutput(&b)
	fs.PrintDefaults()
	fs.SetOutput(os.Stderr)
	return b.String()
}
