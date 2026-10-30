package main

import "testing"

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

func TestBlobStopsAtTheCap(t *testing.T) {
	k := newKernel()
	k.held = maxHeld
	k.rememberBlob(1, "a", 7, []byte("hi"))
	if _, ok := k.blob(7, 0, 1); ok {
		t.Fatal("stored past the cap")
	}
}
