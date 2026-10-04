// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"net"
	"testing"
)

func TestBundleLaunchURL(t *testing.T) {
	const bundled = "/Applications/Trellis.app/Contents/MacOS/trellisd"
	loopback := func(port int) net.Addr { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port} }

	tests := []struct {
		name   string
		exe    string
		scheme string
		addr   net.Addr
		want   string
		opens  bool
	}{
		{"bundle on the canonical port", bundled, "http", loopback(8446), "http://127.0.0.1:8446/", true},
		{"bundle after a port fallback", bundled, "http", loopback(8447), "http://127.0.0.1:8447/", true},
		{"bundle with a credential configured", bundled, "https", loopback(8446), "https://127.0.0.1:8446/", true},
		{
			"credentialed bundle bound to every interface", bundled, "https",
			&net.TCPAddr{IP: net.IPv4zero, Port: 8448}, "https://127.0.0.1:8448/", true,
		},
		{
			"bundle bound to IPv6 loopback", bundled, "http",
			&net.TCPAddr{IP: net.IPv6loopback, Port: 8446}, "http://[::1]:8446/", true,
		},
		{"bundle copied outside Applications", "/Users/s/Desktop/Trellis.app/Contents/MacOS/trellisd", "http", loopback(8446), "http://127.0.0.1:8446/", true},
		{"CLI install", "/usr/local/bin/trellisd", "http", loopback(8446), "", false},
		{"systemd install", "/usr/bin/trellisd", "https", loopback(8446), "", false},
		{"go run build cache", "/tmp/go-build123/b001/exe/trellisd", "http", loopback(8446), "", false},
		{"MacOS directory outside a bundle", "/opt/Contents/MacOS/trellisd", "http", loopback(8446), "", false},
		{"bundle resources, not the executable directory", "/Applications/Trellis.app/Contents/Resources/trellisd", "http", loopback(8446), "", false},
		{"non-TCP listener", bundled, "http", &net.UnixAddr{Name: "/tmp/trellisd.sock", Net: "unix"}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, opens := bundleLaunchURL(tt.exe, tt.scheme, tt.addr)
			if opens != tt.opens || got != tt.want {
				t.Fatalf("bundleLaunchURL(%q, %q, %v) = %q, %v; want %q, %v",
					tt.exe, tt.scheme, tt.addr, got, opens, tt.want, tt.opens)
			}
		})
	}
}
