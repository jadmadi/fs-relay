package client

import (
	"testing"
	"time"
)

func TestReleaseBroadcastWakesAll(t *testing.T) {
	c := &Client{}
	a := c.Releases()
	b := c.Releases()
	c.notifyRelease()
	for name, ch := range map[string]<-chan struct{}{"a": a, "b": b} {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("%s not woken", name)
		}
	}
	next := c.Releases()
	select {
	case <-next:
		t.Fatal("new broadcast already closed")
	default:
	}
}

func TestReleaseWithoutWaiterDoesNotBlock(t *testing.T) {
	c := &Client{}
	for i := 0; i < 100; i++ {
		c.notifyRelease()
	}
}
