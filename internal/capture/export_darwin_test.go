// SPDX-License-Identifier: BUSL-1.1

//go:build darwin && cgo

package capture

import "github.com/MustardSeedNetworks/trellis/core/wifi"

// ScannedNetworkFields is the subset of wifi.ScannedNetwork the mapping tests
// assert on, so a timestamp set from the clock does not have to be threaded
// through every expectation.
type ScannedNetworkFields struct {
	SSID         string
	BSSID        string
	Signal       int
	Channel      int
	Frequency    int
	Security     string
	ChannelWidth int
	NoiseFloor   int
	SNR          int
	HTMode       string
	IsDFS        bool
}

func (f ScannedNetworkFields) matches(n wifi.ScannedNetwork) bool {
	return f.SSID == n.SSID && f.BSSID == n.BSSID && f.Signal == n.Signal &&
		f.Channel == n.Channel && f.Frequency == n.Frequency &&
		f.Security == n.Security && f.ChannelWidth == n.ChannelWidth &&
		f.NoiseFloor == n.NoiseFloor && f.SNR == n.SNR &&
		f.HTMode == n.HTMode && f.IsDFS == n.IsDFS
}
