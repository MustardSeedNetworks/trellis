// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"net"
	"net/http"
	"strconv"
)

// loopbackHostGuard rejects any request whose Host header is not the daemon's
// own loopback address on the port it bound.
//
// The unprotected daemon has no credential, so the only thing standing between
// it and a web page is that a browser reaches loopback only for names that
// resolve there. DNS rebinding defeats that: a page on attacker.example
// re-resolves its name to 127.0.0.1 and then reads the API with the browser's
// same-origin rights. The request still carries the attacker's name in Host,
// which is what this refuses. The credentialed daemon serves TLS to other
// devices under names this cannot know, and authenticates instead, so it is
// not wrapped.
func loopbackHostGuard(next http.Handler, bound net.IP, port int) http.Handler {
	allowed := map[string]bool{
		"localhost":               true,
		"127.0.0.1":               true,
		net.IPv6loopback.String(): true,
		bound.String():            true,
	}
	wantPort := strconv.Itoa(port)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, hostPort, err := net.SplitHostPort(r.Host)
		if err != nil || hostPort != wantPort || !allowed[host] {
			http.Error(w, "invalid Host header", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
