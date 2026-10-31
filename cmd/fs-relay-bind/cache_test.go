package main

import (
	"testing"

	"fs-relay/internal/wire"
)

func TestBlobDroppedWithTheName(t *testing.T) {
	k := newKernel()
	k.rememberBlob(1, "a", 7, []byte("hi"))
	got, ok := k.blob(7, 0, 8)
	if !ok || string(got) != "hi" {
		t.Fatalf("blob %q %v", got, ok)
	}
	k.forgetName(1, "a")
	if _, ok := k.blob(7, 0, 8); ok {
		t.Fatal("blob survived the name")
	}
}

func TestOwnNameIsConsumedOnce(t *testing.T) {
	k := newKernel()
	k.own(1, "a")
	k.own(1, "a")
	if !k.take(1, "a") || !k.take(1, "a") || k.take(1, "a") {
		t.Fatal("own count")
	}
	k.own(1, "a")
	k.disown(1, "a")
	if k.take(1, "a") {
		t.Fatal("disown left a mark")
	}
	k.rememberDir(4, []wire.Dirent{{Name: "a"}})
	k.drop(wire.Change{Parent: 4, Name: "a"})
	if _, ok := k.dir(4); ok {
		t.Fatal("listing survived an eviction of its directory")
	}
}

func TestBlobStopsAtTheCap(t *testing.T) {
	k := newKernel()
	k.held = maxHeld
	k.rememberBlob(1, "a", 7, []byte("hi"))
	if _, ok := k.blob(7, 0, 1); ok {
		t.Fatal("stored past the cap")
	}
}
