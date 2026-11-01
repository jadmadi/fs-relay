package client

import (
	"testing"
	"time"
)

func TestNextDrops(t *testing.T) {
	tests := []struct {
		drops       int
		established bool
		alive       time.Duration
		want        int
	}{
		{0, false, 0, 1},
		{0, true, 10 * time.Millisecond, 1},
		{0, true, time.Second, 0},
		{4, true, 2 * time.Second, 0},
		{4, true, 10 * time.Millisecond, 5},
	}
	for _, tt := range tests {
		got := nextDrops(tt.drops, tt.established, tt.alive)
		if got != tt.want {
			t.Errorf("drops %d established %v alive %s: got %d, want %d", tt.drops, tt.established, tt.alive, got, tt.want)
		}
	}
}
