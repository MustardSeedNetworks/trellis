// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package capture

import (
	"net"
	"testing"
	"time"

	"github.com/mdlayher/netlink"
)

// bssMessage builds the nl80211 dump message the kernel sends for one BSS, so
// the decoder can be exercised without a radio.
func bssMessage(t *testing.T, attrs func(*netlink.AttributeEncoder)) []byte {
	t.Helper()

	ae := netlink.NewAttributeEncoder()
	ae.Nested(nl80211AttrBSS, func(nae *netlink.AttributeEncoder) error {
		attrs(nae)
		return nil
	})
	encoded, err := ae.Encode()
	if err != nil {
		t.Fatalf("encode BSS: %v", err)
	}
	return encoded
}

// TestNetworkFromBSS covers the decode that turns one kernel BSS into Trellis's
// scan model. It is the only Linux capture code that can be tested without a
// radio, and it is where a survey's every value comes from.
func TestNetworkFromBSS(t *testing.T) {
	t.Parallel()

	bssid := net.HardwareAddr{0x24, 0x5a, 0x4c, 0x6b, 0xb5, 0xc8}
	seen := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)

	t.Run("reads a BSS the kernel reported", func(t *testing.T) {
		t.Parallel()

		data := bssMessage(t, func(ae *netlink.AttributeEncoder) {
			ae.Bytes(nl80211BSSBSSID, bssid)
			ae.Uint32(nl80211BSSFrequency, 5180)
			// Milli-dBm, signed: -50 dBm. Read as unsigned this becomes a huge
			// positive number, which is the defect the field's comment records.
			ae.Int32(nl80211BSSSignalMBM, -50*mBmPerDBm)
			ae.Bytes(nl80211BSSInformationElements, append(
				ie(elemSSID, 'l', 'a', 'b'),
				ie(elemBSSLoad, 0x04, 0x00, 128, 0x00, 0x00)...,
			))
		})

		got, ok, err := networkFromBSS(data, seen)
		if err != nil || !ok {
			t.Fatalf("networkFromBSS: ok=%v err=%v", ok, err)
		}
		if got.BSSID != bssid.String() {
			t.Errorf("bssid = %q, want %q", got.BSSID, bssid.String())
		}
		if got.SSID != "lab" {
			t.Errorf("ssid = %q, want lab", got.SSID)
		}
		if got.Signal != -50 {
			t.Errorf("signal = %d dBm, want -50", got.Signal)
		}
		if got.Channel != 36 || got.Frequency != 5180 {
			t.Errorf("channel/frequency = %d/%d, want 36/5180", got.Channel, got.Frequency)
		}
		// nl80211 reports no noise with a scan, so the SNR is derived from the
		// same assumed floor every backend uses.
		if got.SNR != -50-defaultNoiseFloorDBm {
			t.Errorf("snr = %d dB, want %d", got.SNR, -50-defaultNoiseFloorDBm)
		}
		if got.ChannelUtilization == nil || *got.ChannelUtilization != 50 {
			t.Errorf("utilization = %v, want 50%%", got.ChannelUtilization)
		}
		if got.Associated {
			t.Error("a BSS with no status attribute reported as associated")
		}
	})

	t.Run("marks the BSS this host is joined to", func(t *testing.T) {
		t.Parallel()

		// The kernel flags it on the dump itself, which is why Linux needs no
		// second call to learn its own association.
		data := bssMessage(t, func(ae *netlink.AttributeEncoder) {
			ae.Bytes(nl80211BSSBSSID, bssid)
			ae.Uint32(nl80211BSSFrequency, 2437)
			ae.Int32(nl80211BSSSignalMBM, -40*mBmPerDBm)
			ae.Uint32(nl80211BSSStatus, nl80211BSSStatusAssociated)
		})

		got, ok, err := networkFromBSS(data, seen)
		if err != nil || !ok {
			t.Fatalf("networkFromBSS: ok=%v err=%v", ok, err)
		}
		if !got.Associated {
			t.Error("the joined BSS was not marked associated")
		}
		if got.Channel != 6 {
			t.Errorf("channel = %d, want 6", got.Channel)
		}
	})

	t.Run("an AP that sends no BSS Load element reports no utilization", func(t *testing.T) {
		t.Parallel()

		// Absent is not an idle channel, and the two must stay distinguishable
		// all the way to the wire.
		data := bssMessage(t, func(ae *netlink.AttributeEncoder) {
			ae.Bytes(nl80211BSSBSSID, bssid)
			ae.Uint32(nl80211BSSFrequency, 5180)
			ae.Int32(nl80211BSSSignalMBM, -60*mBmPerDBm)
			ae.Bytes(nl80211BSSInformationElements, ie(elemSSID, 'l', 'a', 'b'))
		})

		got, _, err := networkFromBSS(data, seen)
		if err != nil {
			t.Fatalf("networkFromBSS: %v", err)
		}
		if got.ChannelUtilization != nil {
			t.Errorf("utilization = %d%% from an AP that advertised none", *got.ChannelUtilization)
		}
	})

	t.Run("a message carrying no BSS is skipped, not an error", func(t *testing.T) {
		t.Parallel()

		// A scan dump interleaves other message types; refusing them would fail
		// the whole scan over a message that was never a BSS.
		ae := netlink.NewAttributeEncoder()
		ae.Uint32(nl80211AttrIfindex, 3)
		data, err := ae.Encode()
		if err != nil {
			t.Fatalf("encode: %v", err)
		}

		if _, ok, err := networkFromBSS(data, seen); ok || err != nil {
			t.Fatalf("got ok=%v err=%v, want it skipped", ok, err)
		}
	})

	t.Run("a BSS with no BSSID is skipped", func(t *testing.T) {
		t.Parallel()

		data := bssMessage(t, func(ae *netlink.AttributeEncoder) {
			ae.Uint32(nl80211BSSFrequency, 5180)
		})

		if _, ok, _ := networkFromBSS(data, seen); ok {
			t.Error("a BSS with nothing to identify it was accepted")
		}
	})

	t.Run("a DFS channel is marked", func(t *testing.T) {
		t.Parallel()

		data := bssMessage(t, func(ae *netlink.AttributeEncoder) {
			ae.Bytes(nl80211BSSBSSID, bssid)
			ae.Uint32(nl80211BSSFrequency, 5500) // channel 100, radar detection
			ae.Int32(nl80211BSSSignalMBM, -70*mBmPerDBm)
		})

		got, _, err := networkFromBSS(data, seen)
		if err != nil {
			t.Fatalf("networkFromBSS: %v", err)
		}
		if !got.IsDFS {
			t.Errorf("channel %d not marked DFS", got.Channel)
		}
	})
}
