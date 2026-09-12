// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"errors"
	"fmt"
	"net"

	"github.com/MustardSeedNetworks/trellis/internal/auth"
)

// errNotLoopback is returned for a TRELLIS_ADDR that would put the API on a
// network without an operator credential configured. Unprotected, the survey
// API is reachable by anyone on the same Wi-Fi — frequently a client's guest
// network — and it has no authentication, no CSRF and no TLS.
//
// Configuring a credential is what lifts the gate: the daemon then authenticates
// every RPC, checks a CSRF token against the session, rate-limits the login
// surface, and serves over TLS rather than plain HTTP.
var errNotLoopback = errors.New(
	"trellisd binds loopback only until an operator credential is configured; " +
		"set " + auth.EnvUsername + " and " + auth.EnvPassword +
		" to serve another device (TLS is enabled automatically)")

// requireBindAllowed refuses a listen address the daemon must not serve.
//
// A loopback address is always allowed: that is the desktop app ADR-0007
// describes, one operator's browser on the same machine. Any other address is
// allowed only when protected — a credential is configured, so the gate in
// internal/auth is installed in front of every RPC.
//
// An empty host (":8446") means every interface, and a hostname other than
// "localhost" could resolve anywhere, so both are treated as non-loopback
// rather than resolved: the check has to be decidable before anything is bound.
func requireBindAllowed(addr string, protected bool) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("parse listen address %q: %w", addr, err)
	}
	if host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	if protected {
		return nil
	}
	return fmt.Errorf("%w: refusing %q", errNotLoopback, addr)
}
