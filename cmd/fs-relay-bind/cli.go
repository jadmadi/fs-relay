package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"syscall"
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
	b.WriteString("Usage:\n  fs-relay-bind [flags] <mountpoint>\n  fs-relay-bind update\n  fs-relay-bind version\n  fs-relay-bind help\n\n")
	b.WriteString("Example:\n  fs-relay-bind -url http://127.0.0.1:8080 -ns demo -token local-demo /mnt/fs-relay\n\n")
	b.WriteString("Version:\n  " + versionLine("fs-relay-bind") + "\n\n")
	b.WriteString("Flags:\n")
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, "fusermount3", "-uz", mnt).Run(); err == nil {
		return nil
	}
	return syscall.Unmount(mnt, syscall.MNT_DETACH)
}

func busyError(mnt string, detachErr error) error {
	if detachErr != nil {
		return fmt.Errorf("mount %s is busy and lazy unmount failed: %w", mnt, detachErr)
	}
	return fmt.Errorf("mount %s is busy; close open files or run fusermount3 -uz %s", mnt, mnt)
}

func isDone(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	case <-time.After(5 * time.Millisecond):
		return false
	}
}

func runDetach(mnt string, done <-chan struct{}, detach func(string) error, sigc <-chan os.Signal) error {
	if isDone(done) {
		return nil
	}
	dc := make(chan error, 1)
	go func() {
		dc <- detach(mnt)
	}()
	select {
	case <-done:
		return nil
	case err := <-dc:
		if isDone(done) {
			return nil
		}
		return busyError(mnt, err)
	case <-sigc:
		if isDone(done) {
			return nil
		}
		return fmt.Errorf("mount %s is busy and unmount was interrupted; run fusermount3 -uz %s", mnt, mnt)
	}
}

// waitMount waits for a FUSE server to exit or unmounts it on SIGINT/SIGTERM.
// When the mount is busy, stalls past timeout, or receives a second signal, it
// detaches lazily and returns an error so the process exits non-zero.
func waitMount(mnt string, srv mountServer, detach func(string) error, sigc <-chan os.Signal, timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		srv.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-sigc:
		if isDone(done) {
			return nil
		}
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	umc := make(chan error, 1)
	go func() {
		umc <- srv.Unmount()
	}()
	select {
	case <-done:
		return nil
	case <-sigc:
		return runDetach(mnt, done, detach, sigc)
	case <-timer.C:
		return runDetach(mnt, done, detach, sigc)
	case err := <-umc:
		if err != nil {
			return runDetach(mnt, done, detach, sigc)
		}
	}
	select {
	case <-done:
		return nil
	case <-sigc:
		return runDetach(mnt, done, detach, sigc)
	case <-timer.C:
		return runDetach(mnt, done, detach, sigc)
	}
}
