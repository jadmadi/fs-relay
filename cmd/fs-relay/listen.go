package main

import (
	"fmt"
	"net"
)

func listenURL(addr net.Addr, route func(net.IP) (net.IP, error)) string {
	return endpointURL(addr.String(), route)
}

func endpointURL(hostport string, route func(net.IP) (net.IP, error)) string {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		return "http://" + hostport
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsUnspecified() {
		return "http://" + net.JoinHostPort(host, port)
	}
	got, err := route(ip)
	if err != nil || got == nil || got.IsUnspecified() {
		return "http://" + net.JoinHostPort(host, port)
	}
	return "http://" + net.JoinHostPort(got.String(), port)
}

func defaultRouteIP(unspec net.IP) (net.IP, error) {
	if unspec.To4() == nil {
		if ip, err := routeIP("udp6", "[2001:db8::1]:80"); err == nil {
			return ip, nil
		}
	}
	return routeIP("udp4", "192.0.2.1:80")
}

func routeIP(network, dest string) (net.IP, error) {
	conn, err := net.Dial(network, dest)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr.IP == nil {
		return nil, fmt.Errorf("route %s: no address", network)
	}
	return addr.IP, nil
}
