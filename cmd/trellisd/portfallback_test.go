// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"net"
	"strconv"
	"testing"
)

// freeRange returns the first of n consecutive free TCP ports on loopback,
// having closed them again. Nothing else in this binary competes for them, so
// the window between probing and binding is advisory but sufficient.
func freeRange(t *testing.T, n int) int {
	t.Helper()

	var lc net.ListenConfig
	for attempt := 0; attempt < 20; attempt++ {
		probe, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("probe for a free port: %v", err)
		}
		base := probe.Addr().(*net.TCPAddr).Port
		_ = probe.Close()

		held := make([]net.Listener, 0, n)
		for offset := range n {
			ln, err := lc.Listen(t.Context(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(base+offset)))
			if err != nil {
				break
			}
			held = append(held, ln)
		}
		for _, ln := range held {
			_ = ln.Close()
		}
		if len(held) == n {
			return base
		}
	}
	t.Fatalf("no run of %d consecutive free ports after 20 attempts", n)
	return 0
}

// hold binds addr and closes it when the test ends, standing in for whatever
// else on the machine is squatting the port.
func hold(t *testing.T, port int) {
	t.Helper()

	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("hold port %d: %v", port, err)
	}
	t.Cleanup(func() { _ = ln.Close() })
}

func TestBindWithFallbackTakesTheRequestedPortWhenFree(t *testing.T) {
	port := freeRange(t, 1)
	want := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))

	ln, bound, err := bindWithFallback(t.Context(), want)
	if err != nil {
		t.Fatalf("bindWithFallback: %v", err)
	}
	defer func() { _ = ln.Close() }()

	if bound != want {
		t.Errorf("bound %s, want %s", bound, want)
	}
}

func TestBindWithFallbackWalksPastAPortInUse(t *testing.T) {
	port := freeRange(t, 2)
	hold(t, port)

	ln, bound, err := bindWithFallback(t.Context(), net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("bindWithFallback: %v", err)
	}
	defer func() { _ = ln.Close() }()

	want := net.JoinHostPort("127.0.0.1", strconv.Itoa(port+1))
	if bound != want {
		t.Errorf("bound %s, want %s", bound, want)
	}
}

func TestBindWithFallbackFailsWhenTheWholeWalkIsInUse(t *testing.T) {
	port := freeRange(t, portFallbackMaxOffset+1)
	for offset := range portFallbackMaxOffset + 1 {
		hold(t, port+offset)
	}

	ln, _, err := bindWithFallback(t.Context(), net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err == nil {
		_ = ln.Close()
		t.Fatal("bindWithFallback succeeded with the whole walk held")
	}
}

// Port 0 already means "any free port": walking from it would probe ports the
// operator never asked for.
func TestBindWithFallbackDoesNotWalkFromPortZero(t *testing.T) {
	ln, bound, err := bindWithFallback(t.Context(), "127.0.0.1:0")
	if err != nil {
		t.Fatalf("bindWithFallback: %v", err)
	}
	defer func() { _ = ln.Close() }()

	if bound != ln.Addr().String() {
		t.Errorf("reported %s, listening on %s", bound, ln.Addr())
	}
}

func TestBindWithFallbackRejectsAnUnparseableAddress(t *testing.T) {
	if _, _, err := bindWithFallback(t.Context(), "127.0.0.1"); err == nil {
		t.Fatal("bindWithFallback accepted an address with no port")
	}
}

// An operator who names a port wants that port. Silently serving a different
// one would break whatever they pointed at it.
func TestListenDoesNotWalkForAnExplicitAddress(t *testing.T) {
	port := freeRange(t, 2)
	hold(t, port)

	ln, _, err := listen(t.Context(), net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), true)
	if err == nil {
		_ = ln.Close()
		t.Fatal("listen walked past an explicitly requested port that was in use")
	}
}

func TestListenWalksForTheDefaultAddress(t *testing.T) {
	port := freeRange(t, 2)
	hold(t, port)

	ln, bound, err := listen(t.Context(), net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), false)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	want := net.JoinHostPort("127.0.0.1", strconv.Itoa(port+1))
	if bound != want {
		t.Errorf("bound %s, want %s", bound, want)
	}
}
