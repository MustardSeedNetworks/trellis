// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
)

// portFallbackMaxOffset is the highest offset above the requested port that
// bindWithFallback probes: requested..requested+portFallbackMaxOffset.
const portFallbackMaxOffset = 9

// listen opens the daemon's listener and reports the address it actually
// bound.
//
// An operator who names a port with TRELLIS_ADDR gets that port or an error:
// something is pointed at it, and quietly serving a different one would break
// that. The built-in default walks instead. A Trellis.app launched from the
// Finder has no terminal, so a port it cannot have otherwise reads as an app
// that does nothing at all (#151).
func listen(ctx context.Context, addr string, explicit bool) (net.Listener, string, error) {
	if !explicit {
		return bindWithFallback(ctx, addr)
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return nil, "", fmt.Errorf("bind %s: %w", addr, err)
	}
	return ln, ln.Addr().String(), nil
}

// bindWithFallback opens a TCP listener on addr, walking
// port+1..+portFallbackMaxOffset for as long as the port it wants is held by
// something else, and returns the first listener that binds along with the
// address it bound. This is the fleet's port convention, shared with seed,
// stem and niac (#69).
//
// Any other bind error is returned immediately — the caller must treat it as
// fatal (permission denied, address not available).
func bindWithFallback(ctx context.Context, addr string) (net.Listener, string, error) {
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, "", fmt.Errorf("parse listen address %q: %w", addr, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return nil, "", fmt.Errorf("parse port %q from %q: %w", portText, addr, err)
	}

	var lc net.ListenConfig

	// Port 0 already means "any free port". Walking from it would probe ports
	// nobody asked for.
	if port == 0 {
		ln, err := lc.Listen(ctx, "tcp", addr)
		if err != nil {
			return nil, "", fmt.Errorf("bind %s: %w", addr, err)
		}
		return ln, ln.Addr().String(), nil
	}

	for offset := 0; offset <= portFallbackMaxOffset; offset++ {
		bound := port + offset
		candidate := net.JoinHostPort(host, strconv.Itoa(bound))
		ln, err := lc.Listen(ctx, "tcp", candidate)
		if err == nil {
			if offset > 0 {
				slog.WarnContext(ctx, "requested port is in use, bound fallback port instead",
					"requested", port,
					"bound", bound,
				)
			}
			return ln, candidate, nil
		}
		if !isAddrInUse(err) {
			return nil, "", fmt.Errorf("bind %s: %w", candidate, err)
		}
	}
	return nil, "", fmt.Errorf("bind %s and +1..+%d all in use", addr, portFallbackMaxOffset)
}
