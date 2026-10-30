package local

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"

	"fs-relay/internal/wire"
	"golang.org/x/sys/unix"
)

func newTree(t *testing.T) (*Tree, string) {
	t.Helper()
	dir := t.TempDir()
	tr := New(dir)
	t.Cleanup(func() { tr.Close() })
	return tr, dir
}

func call(t *testing.T, tr *Tree, method string, p wire.Params) wire.Result {
	t.Helper()
	p.Namespace = "demo"
	res, err := tr.Call(context.Background(), method, p)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	return res
}

func callErr(t *testing.T, tr *Tree, method string, p wire.Params) error {
	t.Helper()
	if p.Namespace == "" {
		p.Namespace = "demo"
	}
	_, err := tr.Call(context.Background(), method, p)
	if err == nil {
		t.Fatalf("%s: want error", method)
	}
	return err
}

func errnoOf(t *testing.T, err error) syscall.Errno {
	t.Helper()
	var n syscall.Errno
	if !errors.As(err, &n) {
		t.Fatalf("not errno: %v", err)
	}
	return n
}

func TestReadsSkipsWhatItCannotTake(t *testing.T) {
	tr, _ := newTree(t)
	small := call(t, tr, wire.Create, wire.Params{Inode: 1, Name: "a", Mode: 0o644, Flags: syscall.O_RDWR})
	call(t, tr, wire.Write, wire.Params{Fh: small.Fh, Data: []byte("hi")})
	call(t, tr, wire.Release, wire.Params{Fh: small.Fh})
	big := call(t, tr, wire.Create, wire.Params{Inode: 1, Name: "b", Mode: 0o644, Flags: syscall.O_RDWR})
	call(t, tr, wire.Write, wire.Params{Fh: big.Fh, Data: make([]byte, wire.MaxPrefetch+1)})
	call(t, tr, wire.Release, wire.Params{Fh: big.Fh})
	got := call(t, tr, wire.Reads, wire.Params{Inodes: []uint64{small.Attr.Ino, big.Attr.Ino, 99}})
	if len(got.Blobs) != 1 || got.Blobs[0].Inode != small.Attr.Ino || string(got.Blobs[0].Data) != "hi" {
		t.Fatalf("blobs %+v", got.Blobs)
	}
	err := callErr(t, tr, wire.Reads, wire.Params{Inodes: make([]uint64, wire.MaxReads+1)})
	if !errors.Is(err, wire.ErrParams) {
		t.Fatalf("reads: %v", err)
	}
}

func TestReadByInode(t *testing.T) {
	tr, _ := newTree(t)
	file := call(t, tr, wire.Create, wire.Params{Inode: 1, Name: "a", Mode: 0o644, Flags: syscall.O_RDWR})
	call(t, tr, wire.Write, wire.Params{Fh: file.Fh, Data: []byte("hi")})
	call(t, tr, wire.Release, wire.Params{Fh: file.Fh})
	rd := call(t, tr, wire.Read, wire.Params{Inode: file.Attr.Ino, Size: 8})
	if string(rd.Data) != "hi" {
		t.Fatalf("read %q", rd.Data)
	}
}

func TestSession(t *testing.T) {
	tr, _ := newTree(t)
	sub := call(t, tr, wire.Mkdir, wire.Params{Inode: 1, Name: "sub", Mode: 0o755})
	file := call(t, tr, wire.Create, wire.Params{Inode: sub.Attr.Ino, Name: "hello", Mode: 0o644, Flags: syscall.O_RDWR})
	body := []byte("hi")
	wr := call(t, tr, wire.Write, wire.Params{Fh: file.Fh, Data: body})
	if wr.Written != uint32(len(body)) {
		t.Fatalf("written %d", wr.Written)
	}
	rd := call(t, tr, wire.Read, wire.Params{Fh: file.Fh, Size: 8})
	if string(rd.Data) != "hi" {
		t.Fatalf("read %q", rd.Data)
	}
	call(t, tr, wire.Flush, wire.Params{Fh: file.Fh})
	call(t, tr, wire.Fsync, wire.Params{Fh: file.Fh})
	dirIno := sub.Attr.Ino
	fileIno := file.Attr.Ino
	call(t, tr, wire.Rename, wire.Params{Inode: 1, Name: "sub", NewInode: 1, NewName: "dir"})
	got := call(t, tr, wire.Lookup, wire.Params{Inode: 1, Name: "dir"})
	if got.Attr.Ino != dirIno {
		t.Fatalf("dir ino %d want %d", got.Attr.Ino, dirIno)
	}
	child := call(t, tr, wire.Lookup, wire.Params{Inode: dirIno, Name: "hello"})
	if child.Attr.Ino != fileIno {
		t.Fatalf("file ino %d want %d", child.Attr.Ino, fileIno)
	}
	ents := call(t, tr, wire.Readdir, wire.Params{Inode: dirIno})
	if len(ents.Entries) != 1 || ents.Entries[0].Ino != fileIno || ents.Entries[0].Size != 2 {
		t.Fatalf("readdir %+v", ents.Entries)
	}
	raw, err := wire.MarshalResult(ents)
	if err != nil {
		t.Fatal(err)
	}
	back, err := wire.DecodeResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if back.Entries[0].Ino != fileIno || back.Entries[0].Size != 2 {
		t.Fatalf("dirent wire %+v", back.Entries[0])
	}
	again := call(t, tr, wire.Readdir, wire.Params{Inode: dirIno})
	if again.Entries[0].Ino != fileIno {
		t.Fatalf("ino changed")
	}
	sz := call(t, tr, wire.Setattr, wire.Params{Inode: fileIno, Valid: wire.SetSize, Size: 1})
	if sz.Attr.Size != 1 {
		t.Fatalf("size %d", sz.Attr.Size)
	}
	link := call(t, tr, wire.Symlink, wire.Params{Inode: 1, Name: "link", Target: "dir/hello"})
	target := call(t, tr, wire.Readlink, wire.Params{Inode: link.Attr.Ino})
	if target.Target != "dir/hello" {
		t.Fatalf("target %q", target.Target)
	}
	dh := call(t, tr, wire.Opendir, wire.Params{Inode: 1})
	call(t, tr, wire.Releasedir, wire.Params{Fh: dh.Fh})
	call(t, tr, wire.Release, wire.Params{Fh: file.Fh})
	call(t, tr, wire.Unlink, wire.Params{Inode: dirIno, Name: "hello"})
	call(t, tr, wire.Rmdir, wire.Params{Inode: 1, Name: "dir"})
	st := call(t, tr, wire.Statfs, wire.Params{Inode: 1})
	if st.Stat == nil || st.Stat.Bsize == 0 {
		t.Fatalf("statfs %+v", st.Stat)
	}
	call(t, tr, wire.Access, wire.Params{Inode: 1, Mask: unix.R_OK})
}

func TestRejects(t *testing.T) {
	tr, _ := newTree(t)
	cases := []struct {
		name string
		p    wire.Params
		want syscall.Errno
		is   error
	}{
		{"dotdot", wire.Params{Namespace: "demo", Inode: 1, Name: ".."}, syscall.EINVAL, nil},
		{"slash", wire.Params{Namespace: "demo", Inode: 1, Name: "a/b"}, syscall.EINVAL, nil},
		{"missing", wire.Params{Namespace: "demo", Inode: 1, Name: "nope"}, syscall.ENOENT, nil},
		{"badns", wire.Params{Namespace: "../x", Inode: 1, Name: "a"}, 0, wire.ErrParams},
		{"noname", wire.Params{Namespace: "demo", Inode: 1}, 0, wire.ErrParams},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := callErr(t, tr, wire.Lookup, tc.p)
			if tc.is != nil {
				if !errors.Is(err, tc.is) {
					t.Fatalf("%v", err)
				}
				return
			}
			if got := errnoOf(t, err); got != tc.want {
				t.Fatalf("%v", got)
			}
		})
	}
}

func TestBadNamespaceWritesNothing(t *testing.T) {
	tr, dir := newTree(t)
	err := callErr(t, tr, wire.Lookup, wire.Params{Namespace: "../x", Inode: 1, Name: "a"})
	if !errors.Is(err, wire.ErrParams) {
		t.Fatal(err)
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 0 {
		t.Fatalf("namespace escaped: %v", ents)
	}
}

func TestRenameRules(t *testing.T) {
	tr, _ := newTree(t)
	call(t, tr, wire.Mkdir, wire.Params{Inode: 1, Name: "d", Mode: 0o755})
	call(t, tr, wire.Create, wire.Params{Inode: 1, Name: "f", Mode: 0o644})
	call(t, tr, wire.Mkdir, wire.Params{Inode: 1, Name: "full", Mode: 0o755})
	full := call(t, tr, wire.Lookup, wire.Params{Inode: 1, Name: "full"})
	call(t, tr, wire.Create, wire.Params{Inode: full.Attr.Ino, Name: "c", Mode: 0o644})
	err := callErr(t, tr, wire.Rmdir, wire.Params{Inode: 1, Name: "full"})
	if errnoOf(t, err) != syscall.ENOTEMPTY {
		t.Fatal(err)
	}
	err = callErr(t, tr, wire.Rename, wire.Params{Inode: 1, Name: "f", NewInode: 1, NewName: "d"})
	if errnoOf(t, err) != syscall.EISDIR {
		t.Fatal(err)
	}
	err = callErr(t, tr, wire.Rename, wire.Params{Inode: 1, Name: "d", NewInode: 1, NewName: "f"})
	if errnoOf(t, err) != syscall.ENOTDIR {
		t.Fatal(err)
	}
	call(t, tr, wire.Create, wire.Params{Inode: 1, Name: "g", Mode: 0o644})
	err = callErr(t, tr, wire.Rename, wire.Params{
		Inode: 1, Name: "f", NewInode: 1, NewName: "g", Flags: uint32(unix.RENAME_NOREPLACE),
	})
	if errnoOf(t, err) != syscall.EEXIST {
		t.Fatal(err)
	}
	err = callErr(t, tr, wire.Rename, wire.Params{
		Inode: 1, Name: "f", NewInode: 1, NewName: "g", Flags: uint32(unix.RENAME_EXCHANGE),
	})
	if errnoOf(t, err) != syscall.EINVAL {
		t.Fatal(err)
	}
	call(t, tr, wire.Lookup, wire.Params{Inode: 1, Name: "f"})
	f := call(t, tr, wire.Open, wire.Params{Inode: call(t, tr, wire.Lookup, wire.Params{Inode: 1, Name: "f"}).Attr.Ino, Flags: syscall.O_RDONLY})
	call(t, tr, wire.Release, wire.Params{Fh: f.Fh})
	err = callErr(t, tr, wire.Read, wire.Params{Fh: f.Fh, Size: 1})
	if errnoOf(t, err) != syscall.EBADF {
		t.Fatal(err)
	}
}

func TestNamespacesDoNotShare(t *testing.T) {
	tr, _ := newTree(t)
	a, err := tr.Call(context.Background(), wire.Create, wire.Params{Namespace: "a", Inode: 1, Name: "f", Mode: 0o644})
	if err != nil {
		t.Fatal(err)
	}
	b, err := tr.Call(context.Background(), wire.Create, wire.Params{Namespace: "b", Inode: 1, Name: "f", Mode: 0o644})
	if err != nil {
		t.Fatal(err)
	}
	if a.Attr.Ino != 2 || b.Attr.Ino != 2 {
		t.Fatalf("inos %d %d", a.Attr.Ino, b.Attr.Ino)
	}
	_, err = tr.Call(context.Background(), wire.Lookup, wire.Params{Namespace: "a", Inode: 1, Name: "onlyb"})
	if errnoOf(t, err) != syscall.ENOENT {
		t.Fatal(err)
	}
}
