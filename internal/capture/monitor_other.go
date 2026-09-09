// SPDX-License-Identifier: BUSL-1.1

//go:build !linux

package capture

// NewMonitor reports that this platform has no monitor-mode backend.
//
// The gap is the OS, not the work: CoreWLAN offers no monitor mode at all, so
// macOS cannot have one whatever Trellis does, and Windows Native Wifi exposes
// none through the documented API either — a monitor capture there means a
// third-party driver (Npcap in monitor mode), which is a packaging decision
// rather than a port. Both are recorded in docs/10-WIFI-CAPTURE.md so the
// limitation is written down rather than discovered.
func NewMonitor() (Scanner, error) {
	return nil, ErrUnsupported
}
