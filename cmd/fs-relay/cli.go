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
