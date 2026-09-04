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
	"strings"

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

// withAssociation marks the scanned BSS the host is joined to.
//
// Which BSS that is comes from a different place on every platform — CoreWLAN
// reports the current network, nl80211 flags it on the scan dump itself — so
// the reconciliation lives here, once, and each backend supplies the answer in
// whatever way its OS offers.
//
// The scan's own reading of the associated BSS is kept rather than the
// association's: every other row was measured in the same sweep, and a signal
// from a different moment would not be comparable with them. current is only
// appended when the sweep did not see it at all, which happens when a beacon is
// missed or a driver omits the serving AP — blanking the row an operator most
// wants to read would be worse than one entry measured a moment apart.
func withAssociation(
	networks []wifi.ScannedNetwork,
	current *wifi.ScannedNetwork,
) []wifi.ScannedNetwork {
	if current == nil || current.BSSID == "" {
		return networks
	}

	// Drivers do not agree on the case of a BSSID, and a case-sensitive compare
	// would report the connection as a network of its own beside itself.
	want := strings.ToLower(current.BSSID)
	for i := range networks {
		if strings.ToLower(networks[i].BSSID) == want {
			networks[i].Associated = true
			return networks
		}
	}

	associated := *current
	associated.Associated = true
	return append(networks, associated)
}

// dedupeBSSes collapses repeated observations of one BSS into a single entry.
//
// CoreWLAN reports a scan as a set of observations, not a set of BSSs, and a
// sweep of a real office returned the same hidden BSS three times with
// identical values. Every count downstream reads that as three access points:
// the co-channel and adjacent-channel figures on a survey point, the BSS count
// in the live view, and — because a BSSID is what identifies a row — the same
// key rendered three times.
//
// The key is BSSID *and* SSID because a multi-SSID radio broadcasts several
// networks from one BSSID, and those are genuinely different networks on the
// air. The strongest observation wins, so the reading does not depend on the
// order the radio happened to report; the first-seen position is kept, so
// whatever ordering the caller applies afterwards starts from a stable list.
func dedupeBSSes(networks []wifi.ScannedNetwork) []wifi.ScannedNetwork {
	type key struct{ bssid, ssid string }

	at := make(map[key]int, len(networks))
	deduped := make([]wifi.ScannedNetwork, 0, len(networks))
	for _, n := range networks {
		k := key{strings.ToLower(n.BSSID), n.SSID}
		if i, seen := at[k]; seen {
			if n.Signal > deduped[i].Signal {
				deduped[i] = n
			}
			continue
		}
		at[k] = len(deduped)
		deduped = append(deduped, n)
	}
	return deduped
}
