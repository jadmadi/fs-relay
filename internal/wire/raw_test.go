package wire

import (
	"errors"
	"testing"
)

func TestRawHeaderRoundTrip(t *testing.T) {
	var reqb [RawReqLen]byte
	req := RawRequest{Op: RawWrite, Inode: 4, Fh: 9, Offset: -1, Length: 128}
	if err := PutRawRequest(reqb[:], req); err != nil {
		t.Fatal(err)
	}
	got, err := ParseRawRequest(reqb[:])
	if err != nil {
		t.Fatal(err)
	}
	if got != req {
		t.Fatalf("request %+v", got)
	}
	var repb [RawRepLen]byte
	rep := RawReply{Errno: 0, Length: 7}
	if err := PutRawReply(repb[:], rep); err != nil {
		t.Fatal(err)
	}
	back, err := ParseRawReply(repb[:])
	if err != nil || back != rep {
		t.Fatalf("reply %+v %v", back, err)
	}
}

func TestRawHeaderRejects(t *testing.T) {
	var b [RawReqLen]byte
	if err := PutRawRequest(b[:], RawRequest{Op: RawRead, Length: 1}); err != nil {
		t.Fatal(err)
	}
	b[0] = 'X'
	if _, err := ParseRawRequest(b[:]); !errors.Is(err, ErrParams) {
		t.Fatalf("magic %v", err)
	}
	b[0] = 'F'
	b[4] = 9
	if _, err := ParseRawRequest(b[:]); !errors.Is(err, ErrParams) {
		t.Fatalf("op %v", err)
	}
	if err := PutRawRequest(b[:], RawRequest{Op: RawRead, Length: 1}); err != nil {
		t.Fatal(err)
	}
	b[6] = 1
	if _, err := ParseRawRequest(b[:]); !errors.Is(err, ErrParams) {
		t.Fatalf("reserved %v", err)
	}
	if err := PutRawRequest(b[:], RawRequest{Op: RawRead, Length: 1}); err != nil {
		t.Fatal(err)
	}
	binaryZeroLen(b[:])
	if _, err := ParseRawRequest(b[:]); !errors.Is(err, ErrParams) {
		t.Fatalf("length %v", err)
	}
}

func binaryZeroLen(b []byte) {
	b[32], b[33], b[34], b[35] = 0, 0, 0, 0
}

func TestRawLimits(t *testing.T) {
	cases := []struct {
		blk, pipe int
		min, max  int
	}{
		{4096, 1 << 20, 128 << 10, 1<<20 - RawSpare},
		{512, 1 << 20, 64 << 10, 1<<20 - RawSpare},
		{1 << 20, 1 << 20, 1<<20 - RawSpare, 1<<20 - RawSpare},
		{4096, 64 << 10, 64 << 10, 64 << 10},
	}
	for _, tc := range cases {
		min, max := RawLimits(tc.blk, tc.pipe)
		if min != tc.min || max != tc.max {
			t.Fatalf("blk %d pipe %d: got %d %d want %d %d", tc.blk, tc.pipe, min, max, tc.min, tc.max)
		}
	}
}
