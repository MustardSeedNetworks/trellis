// SPDX-License-Identifier: BUSL-1.1

// Package capture turns a host Wi-Fi adapter's observations into Trellis's
// scan model.
//
// It is the only place OS-specific radio code lives, and the only place cgo is
// linked (docs/07-RISKS R5). core/wifi stays data-only; the survey engine and
// reporter consume its types without ever reaching a driver.
package capture

import (
	"errors"

	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// Sentinel conditions a caller can act on.
var (
	// ErrUnsupported means this platform has no capture backend yet.
	ErrUnsupported = errors.New("capture: not supported on this platform")

	// ErrPermission means the OS withheld the data rather than failing. On
	// macOS a scan without Location Services authorization returns the right
	// number of networks with every identifier stripped, so this must be
	// reported instead of an empty survey point.
	ErrPermission = errors.New("capture: OS permission required to read network identifiers")

	// ErrNoInterface means the host reported no Wi-Fi adapter.
	ErrNoInterface = errors.New("capture: no Wi-Fi interface")
)

// Frequency of the first channel in each band, in MHz.
const (
	freq24GHzChannel1  = 2407
	freq24GHzChannel14 = 2484
	freq5GHzBase       = 5000
	freq6GHzChannel1   = 5950
	channelSpacingMHz  = 5
	channel14          = 14
)

// Bands, in GHz, as reported by a driver.
const (
	band24GHz = 2
	band5GHz  = 5
	band6GHz  = 6
)

// Channel widths in MHz.
const (
	width20MHz  = 20
	width40MHz  = 40
	width80MHz  = 80
	width160MHz = 160
	width320MHz = 320
)

// Scanner observes nearby BSSs.
type Scanner interface {
	// Scan returns every BSS currently observable, including hidden networks.
	Scan() ([]wifi.ScannedNetwork, error)
}

// channelToFrequency converts a channel to MHz using the band the driver
// reported. Channel numbers collide across bands — 6 GHz channel 1 is 5955 MHz
// while 2.4 GHz channel 1 is 2412 MHz — so a channel number alone cannot be
// resolved.
func channelToFrequency(channel, bandGHz int) int {
	switch bandGHz {
	case band24GHz:
		if channel == channel14 {
			return freq24GHzChannel14
		}
		return freq24GHzChannel1 + (channel * channelSpacingMHz)
	case band5GHz:
		return freq5GHzBase + (channel * channelSpacingMHz)
	case band6GHz:
		return freq6GHzChannel1 + (channel * channelSpacingMHz)
	default:
		return 0
	}
}

// htModeForWidth names the widest PHY carrying a given channel width, using the
// vocabulary core/wifi already records.
func htModeForWidth(width int) string {
	switch width {
	case width40MHz:
		return "HT40"
	case width80MHz:
		return "VHT80"
	case width160MHz:
		return "HE160"
	case width320MHz:
		return "EHT320"
	default:
		return "HT20"
	}
}

// isDFSChannel reports whether a 5 GHz channel requires radar detection:
// 52-64 (UNII-2) and 100-144 (UNII-2 Extended).
func isDFSChannel(channel int) bool {
	return (channel >= 52 && channel <= 64) || (channel >= 100 && channel <= 144)
}
