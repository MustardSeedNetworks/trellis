// SPDX-License-Identifier: BUSL-1.1

package capture

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"
)

// TestLiveScan drives the host's real radio.
//
// It is opt-in because it needs hardware CI does not have: set
// TRELLIS_LIVE_SCAN=1 on a machine with a Wi-Fi adapter. On macOS the process
// must also be inside the signed, entitled bundle and launched through
// LaunchServices, and on Linux it needs CAP_NET_ADMIN — without those the scan
// either names nothing or is refused outright, which this test reports as a
// failure rather than skipping past.
//
// This is the only test that can catch a backend that compiles, links, and
// returns plausible nonsense: wrong struct offsets on Windows, a misparsed
// netlink attribute on Linux, a unit confusion anywhere.
func TestLiveScan(t *testing.T) {
	if os.Getenv("TRELLIS_LIVE_SCAN") == "" {
		t.Skip("set TRELLIS_LIVE_SCAN=1 on a host with a Wi-Fi adapter to run this")
	}

	scanner, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := Authorize(); err != nil {
		t.Logf("authorize: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	networks, err := scanner.Scan(ctx)
	if err != nil {
		if errors.Is(err, ErrPermission) {
			t.Fatalf("scan was refused or redacted: %v", err)
		}
		t.Fatalf("Scan: %v", err)
	}
	if len(networks) == 0 {
		t.Fatal("scan returned no networks; expected at least one on a host with a radio")
	}

	t.Logf("observed %d networks", len(networks))

	for i, n := range networks {
		// A BSSID is the one field every real observation carries. Its absence
		// is how a redacted macOS scan looks, and how a wrong struct offset
		// looks on Windows.
		if _, err := net.ParseMAC(n.BSSID); err != nil {
			t.Errorf("networks[%d].BSSID = %q, not a MAC address", i, n.BSSID)
		}
		// Ranges wide enough for any real adapter, narrow enough to catch a
		// unit error: mBm read as dBm, or kHz read as MHz.
		if n.Signal > 0 || n.Signal < -120 {
			t.Errorf("networks[%d] (%s) signal = %d dBm, outside any plausible range",
				i, n.BSSID, n.Signal)
		}
		if n.Frequency < 2400 || n.Frequency > 7200 {
			t.Errorf("networks[%d] (%s) frequency = %d MHz, outside every Wi-Fi band",
				i, n.BSSID, n.Frequency)
		}
		if n.Channel <= 0 {
			t.Errorf("networks[%d] (%s) channel = %d at %d MHz",
				i, n.BSSID, n.Channel, n.Frequency)
		}
		if n.Security == "" {
			t.Errorf("networks[%d] (%s) has no security scheme", i, n.BSSID)
		}
		if n.SNR != n.Signal-n.NoiseFloor {
			t.Errorf("networks[%d] (%s) SNR = %d, want %d (signal %d - noise %d)",
				i, n.BSSID, n.SNR, n.Signal-n.NoiseFloor, n.Signal, n.NoiseFloor)
		}
		if n.LastSeen.IsZero() {
			t.Errorf("networks[%d] (%s) has no observation time", i, n.BSSID)
		}
	}

	named := 0
	for _, n := range networks {
		if n.SSID != "" {
			named++
		}
	}
	t.Logf("%d of %d networks are named (the rest are hidden)", named, len(networks))
	if named == 0 {
		t.Error("no network carried an SSID; every AP in range being hidden is not credible")
	}
}
