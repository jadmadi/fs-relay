package main

import (
	"flag"
	"os"
	"strings"
	"syscall"
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

type fakeMount struct {
	waitCh    chan struct{}
	unmountFn func() error
}

func (f *fakeMount) Wait() {
	<-f.waitCh
}

func (f *fakeMount) Unmount() error {
	return f.unmountFn()
}

func TestWaitMount(t *testing.T) {
	t.Run("external-unmount", func(t *testing.T) {
		waitCh := make(chan struct{})
		close(waitCh)
		srv := &fakeMount{
			waitCh:    waitCh,
			unmountFn: func() error { return nil },
		}
		sigc := make(chan os.Signal, 1)
		if err := waitMount("/tmp/mnt", srv, func(string) error { return nil }, sigc); err != nil {
			t.Fatalf("waitMount: %v", err)
		}
	})

	t.Run("clean-signal-unmount", func(t *testing.T) {
		waitCh := make(chan struct{})
		srv := &fakeMount{
			waitCh: waitCh,
			unmountFn: func() error {
				close(waitCh)
				return nil
			},
		}
		sigc := make(chan os.Signal, 1)
		sigc <- syscall.SIGINT
		var detached bool
		if err := waitMount("/tmp/mnt", srv, func(string) error { detached = true; return nil }, sigc); err != nil {
			t.Fatalf("waitMount: %v", err)
		}
		if detached {
			t.Fatal("clean unmount should not lazy-detach")
		}
	})

	t.Run("busy-unmount-detaches-and-returns-error", func(t *testing.T) {
		waitCh := make(chan struct{})
		defer close(waitCh)
		srv := &fakeMount{
			waitCh:    waitCh,
			unmountFn: func() error { return syscall.EBUSY },
		}
		sigc := make(chan os.Signal, 1)
		sigc <- syscall.SIGINT
		var detached string
		err := waitMount("/tmp/mnt", srv, func(m string) error { detached = m; return nil }, sigc)
		if err == nil || !strings.Contains(err.Error(), "mount /tmp/mnt is busy; close open files or run fusermount3 -uz /tmp/mnt") {
			t.Fatalf("waitMount error = %v, want busy message", err)
		}
		if detached != "/tmp/mnt" {
			t.Fatalf("detached = %q, want /tmp/mnt", detached)
		}
	})

	t.Run("second-signal-forces-exit", func(t *testing.T) {
		waitCh := make(chan struct{})
		defer close(waitCh)
		entered := make(chan struct{})
		srv := &fakeMount{
			waitCh: waitCh,
			unmountFn: func() error {
				close(entered)
				<-waitCh
				return nil
			},
		}
		sigc := make(chan os.Signal, 2)
		sigc <- syscall.SIGINT
		go func() {
			<-entered
			sigc <- syscall.SIGINT
		}()
		var detached string
		err := waitMount("/tmp/mnt", srv, func(m string) error { detached = m; return nil }, sigc)
		if err == nil || !strings.Contains(err.Error(), "mount /tmp/mnt is busy") {
			t.Fatalf("waitMount second signal error = %v, want busy message", err)
		}
		if detached != "/tmp/mnt" {
			t.Fatalf("detached = %q, want /tmp/mnt", detached)
		}
	})
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
