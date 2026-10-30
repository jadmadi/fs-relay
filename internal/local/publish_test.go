package local

import (
	"context"
	"os"
	"reflect"
	"testing"

	"fs-relay/internal/wire"
)

func TestChangedAddresses(t *testing.T) {
	tree := New(t.TempDir())
	t.Cleanup(func() { _ = tree.Close() })
	ctx := context.Background()
	ns := "demo"
	root := wire.Params{Namespace: ns, Inode: 1}

	steps := []struct {
		name   string
		method string
		p      wire.Params
		want   []wire.Change
	}{
		{"mkdir", wire.Mkdir, with(root, "d", 0o755), []wire.Change{{Parent: 1, Name: "d"}}},
		{"create", wire.Create, with(root, "a b", 0o644), []wire.Change{{Parent: 1, Name: "a b"}}},
		{"symlink", wire.Symlink, sym(root, "l", "a b"), []wire.Change{{Parent: 1, Name: "l"}}},
	}
	var created wire.Result
	for _, tc := range steps {
		got := publish(t, tree, ctx, tc.method, tc.p)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s published %+v, want %+v", tc.name, got, tc.want)
		}
		if tc.method == wire.Create {
			created, _ = tree.Call(ctx, wire.Lookup, wire.Params{Namespace: ns, Inode: 1, Name: "a b"})
		}
	}
	fh := publishWrite(t, tree, ctx, ns, created.Attr.Ino)
	if got := publish(t, tree, ctx, wire.Write, wire.Params{Namespace: ns, Fh: fh, Data: []byte("x")}); !reflect.DeepEqual(got, []wire.Change{{Parent: 1, Name: "a b"}}) {
		t.Fatalf("write %+v", got)
	}
	if got := publish(t, tree, ctx, wire.Setattr, wire.Params{Namespace: ns, Inode: created.Attr.Ino, Valid: wire.SetMode, Mode: 0o600}); !reflect.DeepEqual(got, []wire.Change{{Parent: 1, Name: "a b"}}) {
		t.Fatalf("setattr %+v", got)
	}
	ren := wire.Params{Namespace: ns, Inode: 1, Name: "a b", NewInode: 1, NewName: "c"}
	if got := publish(t, tree, ctx, wire.Rename, ren); !reflect.DeepEqual(got, []wire.Change{{Parent: 1, Name: "a b"}, {Parent: 1, Name: "c"}}) {
		t.Fatalf("rename %+v", got)
	}
	if got := publish(t, tree, ctx, wire.Unlink, wire.Params{Namespace: ns, Inode: 1, Name: "c"}); !reflect.DeepEqual(got, []wire.Change{{Parent: 1, Name: "c"}}) {
		t.Fatalf("unlink %+v", got)
	}
	if got := publish(t, tree, ctx, wire.Unlink, wire.Params{Namespace: ns, Inode: 1, Name: "l"}); !reflect.DeepEqual(got, []wire.Change{{Parent: 1, Name: "l"}}) {
		t.Fatalf("unlink link %+v", got)
	}
	if got := publish(t, tree, ctx, wire.Rmdir, wire.Params{Namespace: ns, Inode: 1, Name: "d"}); !reflect.DeepEqual(got, []wire.Change{{Parent: 1, Name: "d"}}) {
		t.Fatalf("rmdir %+v", got)
	}
	again := with(root, "x", 0o644)
	again.Flags = uint32(os.O_EXCL)
	if _, err := tree.Call(ctx, wire.Create, again); err != nil {
		t.Fatal(err)
	}
	if got := published(tree, wire.Create, again); got != nil {
		t.Fatalf("refused create published %+v", got)
	}
}

func publish(t *testing.T, tree *Tree, ctx context.Context, method string, p wire.Params) []wire.Change {
	t.Helper()
	if _, err := tree.Call(ctx, method, p); err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	return tree.Changed(method, p)
}

func published(tree *Tree, method string, p wire.Params) []wire.Change {
	_, err := tree.Call(context.Background(), method, p)
	if err != nil {
		return nil
	}
	return tree.Changed(method, p)
}

func publishWrite(t *testing.T, tree *Tree, ctx context.Context, ns string, ino uint64) uint64 {
	t.Helper()
	res, err := tree.Call(ctx, wire.Open, wire.Params{Namespace: ns, Inode: ino, Flags: 1})
	if err != nil {
		t.Fatal(err)
	}
	return res.Fh
}

func with(root wire.Params, name string, mode uint32) wire.Params {
	root.Name = name
	root.Mode = mode
	return root
}

func sym(root wire.Params, name, target string) wire.Params {
	root.Name = name
	root.Target = target
	return root
}
