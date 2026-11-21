// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoopbackHostGuard(t *testing.T) {
	t.Parallel()

	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	guard := loopbackHostGuard(ok, net.ParseIP("127.0.0.1"), 8446)

	tests := []struct {
		name string
		host string
		want int
	}{
		{name: "ipv4 loopback", host: "127.0.0.1:8446", want: http.StatusNoContent},
		{name: "localhost", host: "localhost:8446", want: http.StatusNoContent},
		{name: "ipv6 loopback", host: "[::1]:8446", want: http.StatusNoContent},
		{name: "rebound attacker name", host: "attacker.example:8446", want: http.StatusForbidden},
		{name: "attacker name that looks local", host: "localhost.attacker.example:8446", want: http.StatusForbidden},
		{name: "wrong port", host: "127.0.0.1:9000", want: http.StatusForbidden},
		{name: "no port", host: "127.0.0.1", want: http.StatusForbidden},
		{name: "empty host", host: "", want: http.StatusForbidden},
		{name: "routable address", host: "10.0.0.5:8446", want: http.StatusForbidden},
		{name: "wildcard", host: "0.0.0.0:8446", want: http.StatusForbidden},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", http.NoBody)
			req.Host = tc.host
			rec := httptest.NewRecorder()
			guard.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("Host %q = %d, want %d", tc.host, rec.Code, tc.want)
			}
		})
	}
}

// A daemon bound to a non-default loopback address is reachable under that
// address too, and only that one.
func TestLoopbackHostGuardNamesTheBoundAddress(t *testing.T) {
	t.Parallel()

	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	guard := loopbackHostGuard(ok, net.ParseIP("127.0.0.2"), 8447)

	for host, want := range map[string]int{
		"127.0.0.2:8447": http.StatusNoContent,
		"127.0.0.3:8447": http.StatusForbidden,
	} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
		req.Host = host
		rec := httptest.NewRecorder()
		guard.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("Host %q = %d, want %d", host, rec.Code, want)
		}
	}
}
