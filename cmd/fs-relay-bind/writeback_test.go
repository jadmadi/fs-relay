package main

import (
	"context"
	"syscall"
	"testing"

	"fs-relay/internal/wire"

	"github.com/hanwen/go-fuse/v2/fuse"
)

type countCaller struct {
	writes []wire.Params
	fail   error
}

func (c *countCaller) Releases() <-chan string { return nil }

func (c *countCaller) Call(_ context.Context, method string, p wire.Params) (wire.Result, error) {
	if method == wire.Write || method == wire.Release {
		if c.fail != nil && method == wire.Write {
			err := c.fail
			c.fail = nil
			return wire.Result{}, err
		}
		if method == wire.Write || len(p.Data) > 0 {
			c.writes = append(c.writes, wire.Params{Offset: p.Offset, Data: append([]byte(nil), p.Data...)})
		}
		return wire.Result{Written: uint32(len(p.Data))}, nil
	}
	return wire.Result{}, nil
}

func TestPlanWrite(t *testing.T) {
	const min = 128
	cases := []struct {
		held, n, min int
		end, off     int64
		want         wbAction
	}{
		{0, 10, min, 0, 0, wbStart},
		{0, min, min, 0, 0, wbPut},
		{10, 5, min, 10, 10, wbAppend},
		{10, 5, min, 10, 20, wbSendThenBuf},
		{10, min, min, 10, 20, wbSendThenPut},
		{wire.MaxWrite - 1, 2, min, int64(wire.MaxWrite - 1), int64(wire.MaxWrite - 1), wbSendThenBuf},
	}
	for _, tc := range cases {
		got := planWrite(tc.held, tc.end, tc.off, tc.n, tc.min)
		if got != tc.want {
			t.Fatalf("held %d off %d n %d: got %d want %d", tc.held, tc.off, tc.n, got, tc.want)
		}
	}
}

func TestWriteBehindCoalesces(t *testing.T) {
	c := &countCaller{}
	n := &node{}
	h := &handle{c: c, n: n, fh: 1, ino: 2}
	ctx := context.Background()
	chunk := make([]byte, 10<<10)
	for i := 0; i < 100; i++ {
		if _, errno := h.Write(ctx, chunk, int64(i*len(chunk))); errno != 0 {
			t.Fatalf("write %d: %v", i, errno)
		}
	}
	if len(c.writes) != 0 {
		t.Fatalf("sent %d before flush", len(c.writes))
	}
	if errno := h.Flush(ctx); errno != 0 {
		t.Fatal(errno)
	}
	if len(c.writes) != 0 {
		t.Fatalf("flush sent %d", len(c.writes))
	}
	if errno := h.Release(ctx); errno != 0 {
		t.Fatal(errno)
	}
	if len(c.writes) != 1 || len(c.writes[0].Data) != 100*(10<<10) || c.writes[0].Offset != 0 {
		t.Fatalf("writes %d bytes %d off %d", len(c.writes), len(c.writes[0].Data), c.writes[0].Offset)
	}
}

func TestWriteBehindGapSendsTwice(t *testing.T) {
	c := &countCaller{}
	h := &handle{c: c, n: &node{}, fh: 1}
	ctx := context.Background()
	if _, errno := h.Write(ctx, []byte("aaaa"), 0); errno != 0 {
		t.Fatal(errno)
	}
	if _, errno := h.Write(ctx, []byte("bbbb"), 100); errno != 0 {
		t.Fatal(errno)
	}
	if len(c.writes) != 1 || c.writes[0].Offset != 0 {
		t.Fatalf("gap send %+v", c.writes)
	}
	if errno := h.Flush(ctx); errno != 0 {
		t.Fatal(errno)
	}
	if len(c.writes) != 1 {
		t.Fatalf("flush sent %+v", c.writes)
	}
	if errno := h.Release(ctx); errno != 0 {
		t.Fatal(errno)
	}
	if len(c.writes) != 2 || c.writes[1].Offset != 100 {
		t.Fatalf("writes %+v", c.writes)
	}
}

func TestWriteBehindErrorOnce(t *testing.T) {
	c := &countCaller{fail: &wire.Error{Code: uint32(syscall.EIO), Message: "i/o"}}
	h := &handle{c: c, n: &node{}, fh: 1}
	ctx := context.Background()
	buf := make([]byte, writeBehindMin)
	if _, errno := h.Write(ctx, buf, 0); errno != syscall.EIO {
		t.Fatalf("first %v", errno)
	}
	if _, errno := h.Write(ctx, []byte{1}, 0); errno != syscall.EIO {
		t.Fatalf("next %v", errno)
	}
	if errno := h.Flush(ctx); errno != syscall.EIO {
		t.Fatalf("flush %v", errno)
	}
	if errno := h.Flush(ctx); errno != 0 {
		t.Fatalf("second flush %v", errno)
	}
}

func TestFillSizeFloor(t *testing.T) {
	n := &node{dirty: map[*handle]int64{{}: 15}}
	var a fuse.Attr
	n.fill(&a, wire.Attr{Size: 3})
	if a.Size != 15 {
		t.Fatalf("size %d", a.Size)
	}
}
