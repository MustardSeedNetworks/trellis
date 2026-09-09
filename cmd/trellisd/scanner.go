// SPDX-License-Identifier: BUSL-1.1

//go:build !e2e

package main

import (
	"fmt"
	"os"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/internal/capture"
)

// captureModeEnv selects how the radio is read.
//
// "monitor" listens in monitor mode, which attributes every reading to a
// channel and a moment but takes the adapter away from its association. That
// makes it wrong for an active (throughput) survey and wrong on a host whose
// only radio is also its uplink, so it is opt-in and the OS scan stays the
// default.
const (
	captureModeEnv     = "TRELLIS_CAPTURE_MODE"
	captureModeMonitor = "monitor"
	captureModeScan    = "scan"
)

// newScanner returns the host's radio as the survey engine's capture backend.
//
// capture.Scanner and survey.Scanner are deliberately the same shape (ADR-0006),
// so the backend is consumed directly with no adapter in between. The e2e build
// tag replaces this with a scripted scanner; see scanner_e2e.go.
func newScanner() (survey.Scanner, error) {
	switch mode := os.Getenv(captureModeEnv); mode {
	case "", captureModeScan:
		return capture.New()
	case captureModeMonitor:
		return capture.NewMonitor()
	default:
		return nil, fmt.Errorf(
			"%s=%q: want %q or %q", captureModeEnv, mode, captureModeScan, captureModeMonitor)
	}
}
