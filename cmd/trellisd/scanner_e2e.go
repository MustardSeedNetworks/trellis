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
// weakens a little on each call. A perfectly flat field is a degenerate heatmap
// with an empty colour range; a field with some variation is the picture an
// operator would expect from three points on a floor.
type scriptedScanner struct {
	scans atomic.Int64
}

// fadePerScanDBm is how much the walked-away-from AP loses per capture.
const fadePerScanDBm = 6

func (s *scriptedScanner) Scan(ctx context.Context) ([]wifi.ScannedNetwork, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	n := int(s.scans.Add(1))
	seen := time.Now().UTC()
	return []wifi.ScannedNetwork{
		{SSID: "Trellis Lab", BSSID: "02:00:00:00:00:01", Signal: -48 - n*fadePerScanDBm,
			Channel: 36, Frequency: 5180, Security: "WPA3", ChannelWidth: 80,
			NoiseFloor: -95, SNR: 47 - n*fadePerScanDBm, HTMode: "VHT80", LastSeen: seen},
		{SSID: "Trellis Lab", BSSID: "02:00:00:00:00:02", Signal: -62,
			Channel: 6, Frequency: 2437, Security: "WPA2", ChannelWidth: 20,
			NoiseFloor: -95, SNR: 33, HTMode: "HT20", LastSeen: seen},
		{SSID: "", BSSID: "02:00:00:00:00:03", Signal: -79,
			Channel: 100, Frequency: 5500, Security: "WPA2", ChannelWidth: 40,
			NoiseFloor: -95, SNR: 16, HTMode: "HT40", IsDFS: true, LastSeen: seen},
	}, nil
}
