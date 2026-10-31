package main

import "testing"

func TestRoute(t *testing.T) {
	const (
		min  = 128 << 10
		max  = 1 << 20
		ceil = 1 << 20
	)
	cases := []struct {
		write      bool
		n, lo, hi  int
		ceil, want path
	}{
		{false, 100, min, max, ceil, viaEnvelope},
		{false, min, 0, max, ceil, viaEnvelope},
		{false, min, min, max, ceil, viaRawPipe},
		{false, min, min, max, 0, viaRawBuf},
		{false, 2 << 20, min, max, ceil, viaEnvelope},
		{true, min, min, max, ceil, viaRawWrite},
		{true, 10, min, max, ceil, viaEnvelope},
	}
	for _, tc := range cases {
		if got := route(tc.write, tc.n, tc.lo, tc.hi, int(tc.ceil)); got != tc.want {
			t.Fatalf("write %v n %d: got %d want %d", tc.write, tc.n, got, tc.want)
		}
	}
}
