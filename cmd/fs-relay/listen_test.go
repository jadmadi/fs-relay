package main

import (
	"errors"
	"net"
	"testing"
)

func TestListenURL(t *testing.T) {
	v4 := func(net.IP) (net.IP, error) { return net.ParseIP("192.168.15.29"), nil }
	v6 := func(net.IP) (net.IP, error) { return net.ParseIP("2001:db8::29"), nil }
	fail := func(net.IP) (net.IP, error) { return nil, errors.New("no route") }
	tests := []struct {
		addr  string
		want  string
		route func(net.IP) (net.IP, error)
	}{
		{"127.0.0.1:8080", "http://127.0.0.1:8080", v4},
		{"192.168.1.9:9090", "http://192.168.1.9:9090", fail},
		{"0.0.0.0:8080", "http://192.168.15.29:8080", v4},
		{"[::]:8080", "http://[2001:db8::29]:8080", v6},
		{"0.0.0.0:8080", "http://0.0.0.0:8080", fail},
	}
	for _, tt := range tests {
		got := endpointURL(tt.addr, tt.route)
		if got != tt.want {
			t.Errorf("%s: got %s, want %s", tt.addr, got, tt.want)
		}
	}
}
