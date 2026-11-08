package main

import (
	"errors"
	"flag"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"fs-relay/internal/local"
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

// TestForceMultiServeFlag verifies that run refuses an already-locked data
// directory by default and starts cleanly when -force-multi-serve is set.
func TestForceMultiServeFlag(t *testing.T) {
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })

	root := filepath.Join(t.TempDir(), "data")
	first := local.New(root)
	t.Cleanup(func() { _ = first.Close() })
	if err := first.Prepare(); err != nil {
		t.Fatal(err)
	}
	if err := first.Lock(); err != nil {
		t.Fatal(err)
	}

	os.Args = []string{"fs-relay", "-data", root, "-token", "local-demo", "-listen", "127.0.0.1:0"}
	if err := run(); err == nil || !strings.Contains(err.Error(), "is already served by pid ") {
		t.Fatalf("run without -force-multi-serve = %v, want already served by pid error", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	os.Args = []string{"fs-relay", "-data", root, "-token", "local-demo", "-listen", addr, "-force-multi-serve"}
	errc := make(chan error, 1)
	go func() { errc <- run() }()
	for {
		resp, err := http.Get("http://" + addr + "/")
		if err == nil {
			_ = resp.Body.Close()
			break
		}
		select {
		case runErr := <-errc:
			t.Fatalf("run exited early: %v", runErr)
		default:
			runtime.Gosched()
		}
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if err := <-errc; err != nil {
		t.Fatalf("run with -force-multi-serve: %v", err)
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

func TestRunRejectsPositionalArgs(t *testing.T) {
	origArgs := os.Args
	origStderr := os.Stderr
	t.Cleanup(func() {
		os.Args = origArgs
		os.Stderr = origStderr
	})

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w

	root := filepath.Join(t.TempDir(), "data")
	os.Args = []string{"fs-relay", "-data", root, "unexpected-arg"}
	runErr := run()
	_ = w.Close()

	out, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(runErr, errUsage) {
		t.Fatalf("run() = %v, want errUsage", runErr)
	}
	stderr := string(out)
	if !strings.Contains(stderr, `unknown argument "unexpected-arg"`) {
		t.Fatalf("stderr = %q, want unknown argument message", stderr)
	}
	if !strings.Contains(stderr, "Usage:\n  fs-relay") {
		t.Fatalf("stderr = %q, want usage text", stderr)
	}
	if _, statErr := os.Stat(root); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("data dir %s should not exist when args are rejected: %v", root, statErr)
	}
}
