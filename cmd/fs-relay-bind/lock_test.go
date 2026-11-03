package main

import (
	"context"
	"reflect"
	"syscall"
	"testing"

	"fs-relay/internal/wire"

	"github.com/hanwen/go-fuse/v2/fuse"
)

func TestLockParams(t *testing.T) {
	n := &node{ino: 5}
	tests := []struct {
		name  string
		lk    fuse.FileLock
		flags uint32
		fh    uint64
		want  wire.Params
	}{
		{
			name: "write one byte",
			lk:   fuse.FileLock{Start: 0x40000000, End: 0x40000000, Typ: syscall.F_WRLCK, Pid: 7},
			want: wire.Params{Inode: 5, Owner: 42, Pid: 7, Lock: wire.LockWrite, Offset: 0x40000000, Size: 1},
		},
		{
			name: "read to end of file",
			lk:   fuse.FileLock{Start: 10, End: ^uint64(0), Typ: syscall.F_RDLCK, Pid: 8},
			want: wire.Params{Inode: 5, Owner: 42, Pid: 8, Lock: wire.LockRead, Offset: 10, Size: 0},
		},
		{
			name:  "flock carries the flag",
			lk:    fuse.FileLock{Start: 0, End: ^uint64(0), Typ: syscall.F_WRLCK},
			flags: fuse.LK_FLOCK,
			want:  wire.Params{Inode: 5, Owner: 42, Lock: wire.LockWrite, Size: 0, Flags: wire.FlagFlock},
		},
		{
			name: "open handle is named",
			lk:   fuse.FileLock{Start: 0, End: 0, Typ: syscall.F_UNLCK},
			fh:   9,
			want: wire.Params{Inode: 5, Owner: 42, Lock: wire.LockUnlock, Size: 1, Fh: 9},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var f *handle
			if tt.fh != 0 {
				f = &handle{fh: tt.fh}
			}
			got, errno := lockParams(n, f, 42, &tt.lk, tt.flags)
			if errno != 0 {
				t.Fatalf("errno %v", errno)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %+v want %+v", got, tt.want)
			}
		})
	}
}

func TestLockParamsRejectsHugeOffset(t *testing.T) {
	lk := fuse.FileLock{Start: ^uint64(0), End: ^uint64(0), Typ: syscall.F_RDLCK}
	if _, errno := lockParams(&node{}, nil, 1, &lk, 0); errno != syscall.EINVAL {
		t.Fatalf("errno %v", errno)
	}
}

func TestToErrnoCancelIsEINTR(t *testing.T) {
	if got := toErrno(context.Canceled); got != syscall.EINTR {
		t.Fatalf("cancel: %v", got)
	}
	if got := toErrno(context.DeadlineExceeded); got != syscall.EINTR {
		t.Fatalf("deadline: %v", got)
	}
}
