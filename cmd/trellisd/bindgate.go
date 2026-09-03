// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"errors"
	"fmt"
	"net"
)

// errNotLoopback is returned for a TRELLIS_ADDR that would put the API on a
// network. The daemon has no authentication, TLS or CSRF, so off loopback the
// survey store is open to everyone on the same Wi-Fi — frequently a client's
// guest network. Serving to another device is a real want (#160) and arrives as
// a feature with those three in front of the bind, not as an address override.
var errNotLoopback = errors.New(
	"trellisd serves plain HTTP with no authentication and binds loopback only; " +
		"serving other devices is tracked in #160 and needs auth and TLS first")

// requireLoopback refuses any listen address that is not a loopback address.
//
// An empty host ("":8446") means every interface, and a hostname other than
// "localhost" could resolve anywhere, so both are refused rather than resolved:
// the check has to be decidable before anything is bound.
func requireLoopback(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("parse listen address %q: %w", addr, err)
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("%w: refusing %q", errNotLoopback, addr)
	}
	return nil
}
