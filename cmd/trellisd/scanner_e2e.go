// SPDX-License-Identifier: BUSL-1.1

//go:build e2e

package main

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// newScanner returns a scripted radio for the Playwright suite.
//
// The suite drives trellisd as it ships, but the runners that execute it have
// no Wi-Fi adapter, and the Mac that runs it locally withholds every identifier
// from a process outside the entitled bundle. A walk cannot be exercised end to
// end against either, so this build tag substitutes the one component the
// machine cannot supply and leaves everything downstream of it — the survey
// store, the handlers, the heatmap — as the real thing. It is a compile-time
// choice rather than a runtime switch so the shipping binary carries no knob
// that only a test would ever set.
func newScanner() (survey.Scanner, error) {
	return &scriptedScanner{}, nil
}

// scriptedScanner answers every scan with the same three BSSs, one of which
// weakens on each call and then recovers, cycling through four levels. A
// perfectly flat field is a degenerate heatmap with an empty colour range; a
// field with some variation is the picture an operator would expect from three
// points on a floor.
//
// The fade cycles rather than accumulates because the counter is shared by
// every survey the daemon serves. Two browser projects walking in parallel
// drove an earlier version past the store's -110 dBm floor by the twelfth scan,
// and every capture after that was refused.
type scriptedScanner struct {
	scans atomic.Int64
}

// fadePerScanDBm is how much the walked-away-from AP loses per capture, and
// fadeSteps how many captures it fades for before the cycle restarts. The
// weakest level, -66 dBm, stays above the default -75 dBm dead-zone floor so a
// walk reads as covered until the threshold is raised.
const (
	fadePerScanDBm = 6
	fadeSteps      = 4
	// Busy enough to be a real reading, quiet enough that the live view's
	// verdict is decided by the SNR the fade drives rather than by congestion.
	scriptedUtilizationPercent = 18
)

func (s *scriptedScanner) Scan(ctx context.Context) ([]wifi.ScannedNetwork, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fade := int((s.scans.Add(1)-1)%fadeSteps) * fadePerScanDBm
	seen := time.Now().UTC()
	// One BSS is associated and advertises a BSS Load element: the live view
	// reads the connection and the channel it sits on, and a scripted airspace
	// where nothing is joined could only ever exercise its unassociated branch.
	utilization := scriptedUtilizationPercent
	return []wifi.ScannedNetwork{
		{SSID: "Trellis Lab", BSSID: "02:00:00:00:00:01", Signal: -48 - fade,
			Channel: 36, Frequency: 5180, Security: "WPA3", ChannelWidth: 80,
			NoiseFloor: -95, SNR: 47 - fade, HTMode: "VHT80", LastSeen: seen,
			Associated: true, ChannelUtilization: &utilization},
		{SSID: "Trellis Lab", BSSID: "02:00:00:00:00:02", Signal: -62,
			Channel: 6, Frequency: 2437, Security: "WPA2", ChannelWidth: 20,
			NoiseFloor: -95, SNR: 33, HTMode: "HT20", LastSeen: seen},
		{SSID: "", BSSID: "02:00:00:00:00:03", Signal: -79,
			Channel: 100, Frequency: 5500, Security: "WPA2", ChannelWidth: 40,
			NoiseFloor: -95, SNR: 16, HTMode: "HT40", IsDFS: true, LastSeen: seen},
	}, nil
}
