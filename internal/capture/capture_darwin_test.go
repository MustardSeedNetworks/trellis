// SPDX-License-Identifier: BUSL-1.1

//go:build darwin && cgo

package capture

import (
	"testing"
	"time"

	"github.com/MustardSeedNetworks/foundation/pkg/corewlan"
)

func TestNetworkFrom(t *testing.T) {
	t.Parallel()

	seen := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		in   corewlan.Network
		want ScannedNetworkFields
	}{
		{
			name: "5GHz 802.11ax",
			in: corewlan.Network{
				SSID: "Neuroplasticity", BSSID: "24:5a:4c:6b:b5:c9",
				RSSI: -54, Noise: -87, Channel: 149, ChannelWidth: 40,
				Band: corewlan.Band5GHz, Security: "wpa3Transition",
			},
			want: ScannedNetworkFields{
				SSID: "Neuroplasticity", BSSID: "24:5a:4c:6b:b5:c9", Signal: -54,
				Channel: 149, Frequency: 5745, Security: "WPA3", ChannelWidth: 40,
				NoiseFloor: -87, SNR: 33, HTMode: "HT40", IsDFS: false,
			},
		},
		{
			name: "DFS channel flagged only in 5GHz",
			in: corewlan.Network{
				SSID: "Radar", BSSID: "aa:bb:cc:dd:ee:03", RSSI: -70, Noise: -95,
				Channel: 52, ChannelWidth: 20, Band: corewlan.Band5GHz, Security: "none",
			},
			want: ScannedNetworkFields{
				SSID: "Radar", BSSID: "aa:bb:cc:dd:ee:03", Signal: -70, Channel: 52,
				Frequency: 5260, Security: "Open", ChannelWidth: 20, NoiseFloor: -95,
				SNR: 25, HTMode: "HT20", IsDFS: true,
			},
		},
		{
			// The driver omits a noise measurement on some adapters. Falling
			// back keeps SNR meaningful instead of reporting it against 0 dBm.
			name: "unreported noise falls back to the estimate",
			in: corewlan.Network{
				SSID: "NoNoise", BSSID: "aa:bb:cc:dd:ee:02", RSSI: -50,
				Channel: 36, ChannelWidth: 80, Band: corewlan.Band5GHz,
				Security: "wpa2Personal",
			},
			want: ScannedNetworkFields{
				SSID: "NoNoise", BSSID: "aa:bb:cc:dd:ee:02", Signal: -50, Channel: 36,
				Frequency: 5180, Security: "WPA2", ChannelWidth: 80, NoiseFloor: -95,
				SNR: 45, HTMode: "VHT80",
			},
		},
		{
			// A 6 GHz radio reusing a low channel number must not be reported
			// as 2.4 GHz, and DFS does not apply outside 5 GHz.
			name: "6GHz low channel is not 2.4GHz and never DFS",
			in: corewlan.Network{
				BSSID: "aa:bb:cc:dd:ee:01", RSSI: -60, Noise: -90, Channel: 53,
				ChannelWidth: 160, Band: corewlan.Band6GHz, Security: "wpa3Personal",
			},
			want: ScannedNetworkFields{
				BSSID: "aa:bb:cc:dd:ee:01", Signal: -60, Channel: 53, Frequency: 6215,
				Security: "WPA3", ChannelWidth: 160, NoiseFloor: -90, SNR: 30,
				HTMode: "HE160", IsDFS: false,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := networkFrom(tc.in, seen)
			if !tc.want.matches(got) {
				t.Errorf("networkFrom()\n got = %+v\nwant = %+v", got, tc.want)
			}
			if !got.LastSeen.Equal(seen) {
				t.Errorf("LastSeen = %v, want %v", got.LastSeen, seen)
			}
		})
	}
}

func TestSecurityName(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"none":             "Open",
		"wep":              "WEP",
		"wpaPersonal":      "WPA",
		"wpa2Personal":     "WPA2",
		"wpa3Personal":     "WPA3",
		"wpa3Transition":   "WPA3",
		"wpa3Enterprise":   "WPA3",
		"enterprise":       "Enterprise",
		"somethingWeNever": "",
	}

	for in, want := range tests {
		if got := securityName(in); got != want {
			t.Errorf("securityName(%q) = %q, want %q", in, got, want)
		}
	}
}
