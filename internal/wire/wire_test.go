package wire

import (
	"bytes"
	"errors"
	"testing"
)

func TestGoldenReaddir(t *testing.T) {
	b, err := MarshalRequest(Readdir, Params{Namespace: "mydir", Inode: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte("FSR1")) {
		t.Fatal("missing file identifier")
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

func TestCheckToken(t *testing.T) {
	ok := []string{"testtoken", "long-enough", "Ab_12-cd", "AAAAAAAA"}
	short := []string{"", "short", "1234567"}
	bad := []string{"bad token", "bad+token", "bad/token", "has=equal", "has.dot.."}
	for _, token := range ok {
		if err := CheckToken(token); err != nil {
			t.Fatalf("%q: %v", token, err)
		}
	}
	for _, token := range short {
		if !errors.Is(CheckToken(token), ErrTokenShort) {
			t.Fatalf("%q: %v", token, CheckToken(token))
		}
	}
	for _, token := range bad {
		if !errors.Is(CheckToken(token), ErrTokenChars) {
			t.Fatalf("%q: %v", token, CheckToken(token))
		}
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

func TestReaddirsRoundTrip(t *testing.T) {
	res := Result{Listings: []Listing{{
		Inode:   3,
		Entries: []Dirent{{Name: "a", Attr: Attr{Ino: 4, Mode: 0100644, Size: 2}}},
	}}}
	b, err := MarshalResult(res)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeResponse(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Listings) != 1 || got.Listings[0].Inode != 3 || got.Listings[0].Entries[0].Name != "a" || got.Listings[0].Entries[0].Ino != 4 {
		t.Fatalf("listings %+v", got.Listings)
	}
}

func TestCopyFileRangeRoundTrip(t *testing.T) {
	p := Params{Namespace: "demo", Inode: 4, Fh: 0, Offset: 0, Size: 0, FhOut: 9, OffsetOut: 0}
	b, err := MarshalRequest(CopyFileRange, p)
	if err != nil {
		t.Fatal(err)
	}
	req, err := DecodeRequest(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	got := req.Params
	if req.Method != CopyFileRange || got.Inode != 4 || got.Fh != 0 || got.Offset != 0 || got.Size != 0 || got.FhOut != 9 || got.OffsetOut != 0 {
		t.Fatalf("decoded %+v", got)
	}
}

func TestClientAndReleaseRoundTrip(t *testing.T) {
	b, err := MarshalRequest(Join, Params{Namespace: "demo", Client: "01a18f33-1f73-7000-b869-6772b604713a"})
	if err != nil {
		t.Fatal(err)
	}
	req, err := DecodeRequest(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if req.Params.Client != "01a18f33-1f73-7000-b869-6772b604713a" {
		t.Fatalf("client %q", req.Params.Client)
	}
	p, err := MarshalPub(Pub{Release: "lock-1"})
	if err != nil {
		t.Fatal(err)
	}
	pub, err := DecodePub(p)
	if err != nil {
		t.Fatal(err)
	}
	if pub.Release != "lock-1" {
		t.Fatalf("release %q", pub.Release)
	}
	hello, err := MarshalPub(Pub{Client: "01a18f33-1f73-7000-b869-6772b604713a"})
	if err != nil {
		t.Fatal(err)
	}
	rp, err := DecodeReply(hello)
	if err != nil {
		t.Fatal(err)
	}
	if rp.Client != "01a18f33-1f73-7000-b869-6772b604713a" || rp.ID != 0 {
		t.Fatalf("hello %+v", rp)
	}
}
