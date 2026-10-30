package wire

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"
)

func TestGoldenReaddir(t *testing.T) {
	b, err := MarshalRequest(Readdir, Params{Namespace: "mydir", Inode: 1})
	if err != nil {
		t.Fatal(err)
	}
	got := hex.EncodeToString(b)
	const want = "82a66d6574686f64a772656164646972a6706172616d7382a96e616d657370616365a56d79646972a5696e6f6465cf0000000000000001"
	if got != want {
		t.Fatalf("golden %s", got)
	}
	req, err := DecodeRequest(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != Readdir || req.Params.Namespace != "mydir" || req.Params.Inode != 1 {
		t.Fatalf("decoded %+v", req)
	}
}

func TestResultAndError(t *testing.T) {
	ok, err := MarshalResult(Result{Data: []byte{0x00, 0xff}})
	if err != nil {
		t.Fatal(err)
	}
	res, err := DecodeResponse(ok)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(res.Data, []byte{0x00, 0xff}) {
		t.Fatalf("data %x", res.Data)
	}
	fail, err := MarshalCallError(Error{Code: 2, Message: "no such file or directory"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = DecodeResponse(fail)
	var got *Error
	if !errors.As(err, &got) || got.Code != 2 {
		t.Fatalf("err %#v", err)
	}
}

func TestCallID(t *testing.T) {
	if _, err := MarshalCall(0, Mkdir, Params{Namespace: "demo"}); err == nil {
		t.Fatal("id 0 accepted")
	}
	b, err := MarshalCall(7, Mkdir, Params{Namespace: "demo", Name: "a"})
	if err != nil {
		t.Fatal(err)
	}
	req, err := DecodeRequest(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if req.ID != 7 || req.Method != Mkdir || req.Params.Name != "a" {
		t.Fatalf("decoded %+v", req)
	}
	pending, err := MarshalPending(7, "ticket")
	if err != nil {
		t.Fatal(err)
	}
	rp, err := DecodeReply(pending)
	if err != nil {
		t.Fatal(err)
	}
	if rp.ID != 7 || rp.Pending != "ticket" || rp.Result != nil || rp.Err != nil {
		t.Fatalf("pending %+v", rp)
	}
	ok, err := MarshalReply(7, Result{Written: 3})
	if err != nil {
		t.Fatal(err)
	}
	rp, err = DecodeReply(ok)
	if err != nil {
		t.Fatal(err)
	}
	if rp.ID != 7 || rp.Result == nil || rp.Result.Written != 3 || rp.Pending != "" {
		t.Fatalf("reply %+v", rp)
	}
}

func TestCheckNS(t *testing.T) {
	ok := []string{"a", "mydir", "A_b-1"}
	bad := []string{"", ".", "..", "a/b", "has space", string(make([]byte, MaxNamespace+1))}
	for _, ns := range ok {
		if err := CheckNS(ns); err != nil {
			t.Fatalf("%q: %v", ns, err)
		}
	}
	for _, ns := range bad {
		if err := CheckNS(ns); err == nil {
			t.Fatalf("%q accepted", ns)
		}
	}
}
