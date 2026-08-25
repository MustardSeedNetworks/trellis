// SPDX-License-Identifier: BUSL-1.1

// Package wifi holds Trellis's shared Wi-Fi domain model — the scan-result and
// BSS types the survey engine and (later) the live/troubleshooting paths both
// consume. It is deliberately data-only: no capture, no I/O, no OS-specific
// code. internal/capture (per docs/03-TECH-STACK) produces these values;
// core/survey and the reporter consume them.
package wifi

import "time"

// ScannedNetwork is one observed BSS at one moment — the atom of a passive
// Wi-Fi scan. Lifted from Seed's measured-survey subsystem (the shared scan
// primitive under the "all Wi-Fi → Trellis" migration, docs/09) so Trellis
// owns the model rather than importing Seed. Field semantics and JSON wire
// names are preserved so existing AirMapper imports and survey fixtures round-
// trip unchanged.
type ScannedNetwork struct {
	SSID         string    `json:"ssid"`
	BSSID        string    `json:"bssid"`
	Signal       int       `json:"signal"`       // dBm
	Channel      int       `json:"channel"`      // 802.11 channel number
	Frequency    int       `json:"frequency"`    // MHz
	Security     string    `json:"security"`     // e.g. "WPA3", "Open"
	ChannelWidth int       `json:"channelWidth"` // 20, 40, 80, 160, 320 MHz
	NoiseFloor   int       `json:"noiseFloor"`   // dBm (typically -90 to -100)
	SNR          int       `json:"snr"`          // Signal-to-Noise Ratio (Signal - NoiseFloor)
	HTMode       string    `json:"htMode"`       // HT20, HT40, VHT80, HE160, EHT320, etc.
	IsDFS        bool      `json:"isDFS"`        // channel is DFS (Dynamic Frequency Selection)
	LastSeen     time.Time `json:"lastSeen"`
}
