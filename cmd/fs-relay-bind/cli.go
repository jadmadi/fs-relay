package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
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

// bindSubcommand returns "update", "version", or "help" when invoked as a bare
// single-word command with no flags, or "" when arguments represent a mountpoint.
func bindSubcommand(fs *flag.FlagSet) string {
	if fs.NFlag() != 0 || fs.NArg() != 1 {
		return ""
	}
	switch fs.Arg(0) {
	case "update", "version", "help":
		return fs.Arg(0)
	default:
		return ""
	}
}

func dispatchBindCLI(flags *flag.FlagSet, showVersion, doUpdate bool) (bool, error) {
	sub := bindSubcommand(flags)
	if sub == "help" {
		fmt.Print(usageText(flags))
		return true, nil
	}
	if showVersion || sub == "version" {
		fmt.Println(versionLine("fs-relay-bind"))
		return true, nil
	}
	if doUpdate || sub == "update" {
		return true, runBindUpdate()
	}
	return false, nil
}

func runBindUpdate() error {
	if self, err := os.Executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(self), "fs-relay")
		if info, err := os.Stat(sibling); err == nil && info.Mode().IsRegular() {
			cmd := exec.Command(sibling, "update")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
	}
	if path, err := exec.LookPath("fs-relay"); err == nil {
		cmd := exec.Command(path, "update")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	return fmt.Errorf("run \"fs-relay update\" to upgrade both fs-relay and fs-relay-bind")
}

func usageText(fs *flag.FlagSet) string {
	var b strings.Builder
	b.WriteString("fs-relay-bind mounts an fs-relay namespace with FUSE.\n\n")
	b.WriteString("Usage:\n  fs-relay-bind [flags] <mountpoint>\n  fs-relay-bind update\n  fs-relay-bind version\n  fs-relay-bind help\n\n")
	b.WriteString("Example:\n  fs-relay-bind -url http://127.0.0.1:8080 -ns demo -token local-demo /mnt/fs-relay\n\n")
	b.WriteString("Version:\n  ")
	b.WriteString(versionLine("fs-relay-bind"))
	b.WriteString("\n\nFlags:\n")
	fs.SetOutput(&b)
	fs.PrintDefaults()
	fs.SetOutput(os.Stderr)
	return b.String()
}

type mountServer interface {
	Unmount() error
	Wait()
}

// lazyUnmount detaches a busy FUSE mountpoint so it clears when open files close.
func lazyUnmount(mnt string) error {
	if err := exec.Command("fusermount3", "-uz", mnt).Run(); err == nil {
		return nil
	}
	return syscall.Unmount(mnt, syscall.MNT_DETACH)
}

// waitMount waits for a FUSE server to exit or unmounts it on SIGINT/SIGTERM.
// When the mount is busy or a second signal arrives, it detaches lazily and
// returns an error so the process exits non-zero instead of hanging.
func waitMount(mnt string, srv mountServer, detach func(string) error, sigc <-chan os.Signal) error {
	done := make(chan struct{})
	go func() {
		srv.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-sigc:
	}
	umc := make(chan error, 1)
	go func() {
		umc <- srv.Unmount()
	}()
	select {
	case <-done:
		return nil
	case <-sigc:
		_ = detach(mnt)
		return fmt.Errorf("mount %s is busy; close open files or run fusermount3 -uz %s", mnt, mnt)
	case err := <-umc:
		if err != nil {
			_ = detach(mnt)
			return fmt.Errorf("mount %s is busy; close open files or run fusermount3 -uz %s", mnt, mnt)
		}
	}
	select {
	case <-done:
		return nil
	case <-sigc:
		_ = detach(mnt)
		return fmt.Errorf("mount %s is busy; close open files or run fusermount3 -uz %s", mnt, mnt)
	}
}
