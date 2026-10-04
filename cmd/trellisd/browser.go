// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

// bundleLaunchURL returns the address to open in the operator's browser when
// exe is the daemon inside a macOS application bundle, and false otherwise.
//
// A Finder launch of Trellis.app gives the operator nothing to look at: the
// bundle runs trellisd with no window and no terminal, and after a port
// fallback the habitual address reaches whatever holds 8446 (#157). So a
// bundled launch opens the address it actually bound, once. A CLI or systemd
// start runs a binary outside any bundle and never opens a browser.
//
// exe is taken as reported, not resolved: a shell symlink into the bundle is a
// CLI start, and resolving it would make it look like a Finder launch.
func bundleLaunchURL(exe, scheme string, addr net.Addr) (string, bool) {
	macOS := filepath.Dir(exe)
	contents := filepath.Dir(macOS)
	if filepath.Base(macOS) != "MacOS" || filepath.Base(contents) != "Contents" ||
		!strings.HasSuffix(filepath.Dir(contents), ".app") {
		return "", false
	}

	tcp, ok := addr.(*net.TCPAddr)
	if !ok {
		return "", false
	}
	// A credentialed daemon may bind every interface; the browser is on this
	// machine, and loopback is covered by the generated certificate.
	ip := tcp.IP
	if ip == nil || ip.IsUnspecified() {
		ip = net.IPv4(127, 0, 0, 1)
	}
	u := url.URL{Scheme: scheme, Host: net.JoinHostPort(ip.String(), strconv.Itoa(tcp.Port)), Path: "/"}
	return u.String(), true
}
