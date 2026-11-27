// SPDX-License-Identifier: BUSL-1.1

package main

import "os"

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

// captureModeUnavailable is what the daemon reports on a host whose capture
// backend could not be built, as opposed to either mode it was asked for.
const captureModeUnavailable = "unavailable"

// captureModeName is the capture mode the daemon is actually running, for the
// Settings Daemon tab. A backend that failed to build is reported as such
// rather than as the mode the operator asked for, which it is not delivering.
func captureModeName(scannerErr error) string {
	if scannerErr != nil {
		return captureModeUnavailable
	}
	if os.Getenv(captureModeEnv) == captureModeMonitor {
		return captureModeMonitor
	}
	return captureModeScan
}
