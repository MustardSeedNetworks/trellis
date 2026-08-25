// SPDX-License-Identifier: BUSL-1.1

// Package capture turns a host Wi-Fi adapter's observations into Trellis's
// scan model.
//
// It is the only place OS-specific radio code lives, and the only place cgo is
// linked (docs/07-RISKS R5). core/wifi stays data-only; the survey engine and
// reporter consume its types without ever reaching a driver.
package capture

import (
	"context"
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

// defaultNoiseFloorDBm stands in when the driver reports no noise measurement:
// CoreWLAN omits it for scanned networks on some adapters, and nl80211 never
// reports one with scan results at all. Reporting 0 dBm would make the derived
// SNR meaningless, and every backend using the same assumption keeps points
// comparable across the hosts that walked them.
const defaultNoiseFloorDBm = -95

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
//
// This is the same shape as core/survey.Scanner, deliberately: the survey
// engine consumes a capture backend directly, with no adapter in between.
type Scanner interface {
	// Scan returns every BSS currently observable, including hidden networks.
	//
	// Cancellation is coarse. An active scan is a blocking call into the OS
	// radio stack — three to four seconds on macOS — and no platform offers a
	// way to abort one in flight. ctx is honoured before the call starts, so a
	// cancelled context prevents a scan rather than interrupting one.
	Scan(ctx context.Context) ([]wifi.ScannedNetwork, error)
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

// channelForFrequency converts MHz back to a channel number and its band in
// GHz. Linux and Windows report frequency where CoreWLAN reports a channel, and
// the 2.4/5 GHz split at 2484 MHz plus the 5/6 GHz overlap make this more than
// arithmetic: 5955 MHz is 6 GHz channel 1, while 5955 read as 5 GHz would be
// channel 191.
//
// A frequency outside every allocated band returns 0, 0 rather than a plausible
// wrong channel.
func channelForFrequency(freqMHz int) (channel, bandGHz int) {
	switch {
	case freqMHz == freq24GHzChannel14:
		return channel14, band24GHz
	case freqMHz >= 2412 && freqMHz <= 2472:
		return (freqMHz - freq24GHzChannel1) / channelSpacingMHz, band24GHz
	// The 5 and 6 GHz bands are contiguous in Hz but not in channel numbering:
	// 5 GHz stops at 5885 (channel 177) and 6 GHz starts at 5955 (channel 1).
	case freqMHz >= 5160 && freqMHz <= 5885:
		return (freqMHz - freq5GHzBase) / channelSpacingMHz, band5GHz
	case freqMHz >= 5955 && freqMHz <= 7115:
		return (freqMHz - freq6GHzChannel1) / channelSpacingMHz, band6GHz
	default:
		return 0, 0
	}
}

// isDFSChannel reports whether a 5 GHz channel requires radar detection:
// 52-64 (UNII-2) and 100-144 (UNII-2 Extended).
func isDFSChannel(channel int) bool {
	return (channel >= 52 && channel <= 64) || (channel >= 100 && channel <= 144)
}
