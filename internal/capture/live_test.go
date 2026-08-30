// SPDX-License-Identifier: BUSL-1.1

package capture

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/wifi"
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

// TestLiveScanCadence measures how fast the host's radio can actually be
// re-scanned, which is the number that decides survey UX: a surveyor walking a
// floor cannot take points faster than the adapter sweeps.
//
// Same hardware gate as TestLiveScan. It reports rather than asserts a rate —
// the answer is a property of the driver, not of this code — but it does fail
// when *no* scan came back fresh, because a backend serving nothing but its
// cache would let a survey record the same observation at every point on the
// floor and look like it was working.
//
// Freshness cannot come from LastSeen: every backend stamps that with
// time.Now() at parse. It comes from the readings themselves. Real sweeps
// jitter RSSI across the network set, so two consecutive scans with a
// byte-identical BSSID-to-signal fingerprint were served from one sweep. That
// is how a rate-limited Windows WlanScan looks: it returns quickly, reports
// success, and leaves the BSS list exactly as it was.
func TestLiveScanCadence(t *testing.T) {
	if os.Getenv("TRELLIS_LIVE_SCAN") == "" {
		t.Skip("set TRELLIS_LIVE_SCAN=1 on a host with a Wi-Fi adapter to run this")
	}

	const (
		cadenceSamples = 20
		cadenceWindow  = 60 * time.Second
	)

	scanner, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := Authorize(); err != nil {
		t.Logf("authorize: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cadenceWindow)
	defer cancel()

	var (
		latencies []time.Duration
		previous  string
		fresh     int
	)
	start := time.Now()

	for len(latencies) < cadenceSamples {
		before := time.Now()
		networks, err := scanner.Scan(ctx)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				break
			}
			t.Fatalf("scan %d: %v", len(latencies)+1, err)
		}
		latencies = append(latencies, time.Since(before))

		current := signalFingerprint(networks)
		if len(latencies) > 1 && current != previous {
			fresh++
		}
		previous = current
	}

	elapsed := time.Since(start)

	slices.Sort(latencies)
	t.Logf("%d scans in %s — %.2f scans/s end to end", len(latencies), elapsed.Round(time.Millisecond),
		float64(len(latencies))/elapsed.Seconds())
	t.Logf("per-scan latency: min %s median %s max %s",
		latencies[0].Round(time.Millisecond),
		latencies[len(latencies)/2].Round(time.Millisecond),
		latencies[len(latencies)-1].Round(time.Millisecond))
	t.Logf("%d of %d scans returned changed readings — %.2f fresh sweeps/s",
		fresh, len(latencies)-1, float64(fresh)/elapsed.Seconds())

	if err := assessCadence(len(latencies), fresh); err != nil {
		t.Error(err)
	}
}

// minCadenceSamples is the smallest number of completed scans that can say
// anything about freshness. One scan has nothing to differ from.
const minCadenceSamples = 2

// assessCadence decides whether a cadence run proved anything.
//
// The freshness check used to be guarded by `len(latencies) > 1`, so exactly
// one completed scan inside the window skipped it and the test passed having
// made zero comparisons -- the precise case a rate-limited backend produces.
// Separated from the measurement loop so it can be tested without a radio.
func assessCadence(scans, fresh int) error {
	if scans < minCadenceSamples {
		return fmt.Errorf(
			"%d scan(s) completed inside the measurement window, need at least %d: "+
				"a single scan has nothing to compare against, so freshness was never tested",
			scans, minCadenceSamples)
	}
	if fresh == 0 {
		return fmt.Errorf(
			"every scan after the first returned identical readings: the backend is "+
				"serving a cache, and a survey would record %d copies of one observation",
			scans)
	}
	return nil
}

// signalFingerprint reduces a scan to the part that moves between real sweeps:
// which BSSIDs were seen and how strong each was.
func signalFingerprint(networks []wifi.ScannedNetwork) string {
	readings := make([]string, 0, len(networks))
	for _, n := range networks {
		readings = append(readings, fmt.Sprintf("%s=%d", n.BSSID, n.Signal))
	}
	slices.Sort(readings)
	return strings.Join(readings, ",")
}

// TestAssessCadence pins the decision the live cadence run makes, so the rule
// is verified on every CI run rather than only on a host with a radio.
func TestAssessCadence(t *testing.T) {
	tests := []struct {
		name    string
		scans   int
		fresh   int
		wantErr string
	}{
		{
			name:    "no scan completed",
			scans:   0,
			fresh:   0,
			wantErr: "need at least 2",
		},
		{
			// The case the old guard let through: one scan, zero comparisons,
			// green.
			name:    "one scan proves nothing",
			scans:   1,
			fresh:   0,
			wantErr: "need at least 2",
		},
		{
			name:    "two scans, both identical",
			scans:   2,
			fresh:   0,
			wantErr: "serving a cache",
		},
		{
			name:    "twenty scans, none fresh",
			scans:   20,
			fresh:   0,
			wantErr: "serving a cache",
		},
		{
			name:    "two scans, one fresh",
			scans:   2,
			fresh:   1,
			wantErr: "",
		},
		{
			name:    "twenty scans, all fresh",
			scans:   20,
			fresh:   19,
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := assessCadence(tt.scans, tt.fresh)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("assessCadence(%d, %d) = %v, want nil", tt.scans, tt.fresh, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("assessCadence(%d, %d) = nil, want error containing %q",
					tt.scans, tt.fresh, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("assessCadence(%d, %d) = %q, want it to contain %q",
					tt.scans, tt.fresh, err, tt.wantErr)
			}
		})
	}
}
