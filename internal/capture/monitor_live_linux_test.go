// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package capture

import (
	"context"
	"net"
	"os"
	"testing"
	"time"
)

// TestLiveMonitorSweep drives a real radio in monitor mode.
//
// Opt-in like [TestLiveScan], and for the same reason: it needs an adapter
// whose driver supports monitor mode plus CAP_NET_ADMIN. Set
// TRELLIS_LIVE_MONITOR=1 to run it.
//
// It is the only test that can tell monitor-mode acquisition from a
// convincing imitation of it. Every parser here has a unit test against a
// captured frame, and all of them would still pass if the sweep never changed
// channel, never created an interface, or returned the same cached BSS list
// the OS scan returns.
func TestLiveMonitorSweep(t *testing.T) {
	if os.Getenv("TRELLIS_LIVE_MONITOR") == "" {
		t.Skip("set TRELLIS_LIVE_MONITOR=1 on a host with a monitor-capable adapter")
	}

	scanner, err := NewMonitor()
	if err != nil {
		t.Fatalf("NewMonitor: %v", err)
	}

	conn, family, err := dial()
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()
	_, managedName, _, err := managedInterface(conn, family)
	if err != nil {
		t.Fatalf("managedInterface: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	networks, err := scanner.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(networks) == 0 {
		t.Fatal("sweep returned no BSSs; expected at least one AP in range")
	}

	// Every reading must be attributable to a channel the AP claimed. A
	// channel of 0 means the DS Parameter Set was never read, which is what a
	// sweep that reported the dwell channel would look like.
	channels := make(map[int]int)
	for _, n := range networks {
		if n.Channel == 0 {
			t.Errorf("BSS %s has no channel", n.BSSID)
		}
		if n.Signal >= 0 {
			t.Errorf("BSS %s signal = %d dBm, want a negative reading", n.BSSID, n.Signal)
		}
		if _, err := net.ParseMAC(n.BSSID); err != nil {
			t.Errorf("BSS %q is not a MAC address", n.BSSID)
		}
		channels[n.Channel]++
	}
	t.Logf("%d BSSs across %d channels", len(networks), len(channels))

	// The adapter must be exactly as it was found: still a station, with no
	// monitor interface left behind.
	if _, err := net.InterfaceByName(monitorIfName); err == nil {
		t.Errorf("%s still exists after the sweep", monitorIfName)
	}
	assertManagedType(t, managedName)
}

// TestLiveMonitorRecoversStaleInterface proves the crash path.
//
// A killed sweep cannot run its own cleanup, so it leaves a monitor interface
// behind and the managed link down. The next sweep has to undo both: deleting
// the interface but leaving the link down would leave the operator's Wi-Fi
// switched off with nothing to say why.
func TestLiveMonitorRecoversStaleInterface(t *testing.T) {
	if os.Getenv("TRELLIS_LIVE_MONITOR") == "" {
		t.Skip("set TRELLIS_LIVE_MONITOR=1 on a host with a monitor-capable adapter")
	}

	conn, family, err := dial()
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	_, managedName, wiphy, err := managedInterface(conn, family)
	if err != nil {
		t.Fatalf("managedInterface: %v", err)
	}

	// Reproduce exactly what a kill -9 leaves: our monitor interface present
	// and the managed link down.
	if _, err := addMonitorInterface(conn, family, wiphy); err != nil {
		t.Fatalf("addMonitorInterface: %v", err)
	}
	if err := writeInterfaceFlag(managedName, false); err != nil {
		t.Fatalf("down %s: %v", managedName, err)
	}

	scanner, err := NewMonitor()
	if err != nil {
		t.Fatalf("NewMonitor: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if _, err := scanner.Scan(ctx); err != nil {
		t.Fatalf("Scan after a simulated crash: %v", err)
	}

	if _, err := net.InterfaceByName(monitorIfName); err == nil {
		t.Errorf("%s survived the recovering sweep", monitorIfName)
	}
	assertManagedType(t, managedName)

	iface, err := net.InterfaceByName(managedName)
	if err != nil {
		t.Fatalf("look up %s: %v", managedName, err)
	}
	if iface.Flags&net.FlagUp == 0 {
		t.Errorf("%s is still down after recovery; a crashed survey must not "+
			"leave the operator's Wi-Fi switched off", managedName)
	}
}

// assertManagedType fails if the operator's interface is not a station.
//
// This is the invariant that a separate monitor interface buys: whatever
// happens to a sweep, the interface the operator associates with is never
// retyped, so no failure can leave the adapter in a mode nothing recorded as
// temporary.
func assertManagedType(t *testing.T, name string) {
	t.Helper()

	conn, family, err := dial()
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	_, got, _, err := managedInterface(conn, family)
	if err != nil {
		t.Fatalf("the managed interface is no longer a station: %v", err)
	}
	if got != name {
		t.Errorf("station interface = %q, want %q", got, name)
	}
}
