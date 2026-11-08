package main

import (
	"os"
	"strings"
	"syscall"
	"testing"
)

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

	t.Run("detach-failure-reported", func(t *testing.T) {
		waitCh := make(chan struct{})
		defer close(waitCh)
		srv := &fakeMount{
			waitCh:    waitCh,
			unmountFn: func() error { return syscall.EBUSY },
		}
		sigc := make(chan os.Signal, 1)
		sigc <- syscall.SIGINT
		err := waitMount("/tmp/mnt", srv, func(m string) error { return syscall.EPERM }, sigc)
		if err == nil || !strings.Contains(err.Error(), "lazy unmount failed: operation not permitted") {
			t.Fatalf("waitMount detach failure error = %v, want operation not permitted", err)
		}
	})
}
