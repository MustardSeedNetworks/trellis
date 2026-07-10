// SPDX-License-Identifier: BUSL-1.1

package wifi

import "time"

// StationView is one client station associated to a BSS. Lifted verbatim from
// Seed's internal/wifi/airspace.StationView (data-only, no capture code) so
// core/survey's anomaly seam and any future live-capture path share one wire
// shape.
type StationView struct {
	MAC       string    `json:"mac"`
	SignalDBm int       `json:"signalDbm"`
	Frames    int       `json:"frames"`
	LastSeen  time.Time `json:"lastSeen"`
}

// BSSView is one observed BSSID (radio) — the shared shape passive scans and
// live capture both project onto for anomaly detection. Lifted from Seed's
// internal/wifi/airspace.BSSView; fields a passive OS scan cannot observe
// (PMF/RRM/BTM/FT/BSS-Load/country, decoded standard) are left at their zero
// values by scan-only producers, so detection rules that need them simply do
// not fire.
type BSSView struct {
	BSSID        string `json:"bssid"`
	SSID         string `json:"ssid"`
	Hidden       bool   `json:"hidden"`
	Band         string `json:"band"`
	Channel      int    `json:"channel"`
	Security     string `json:"security"`
	Standard     string `json:"standard"`
	CountryCode  string `json:"countryCode,omitempty"`
	PMFRequired  bool   `json:"pmfRequired"`
	RRMNeighbor  bool   `json:"rrmNeighbor"`
	BTMSupported bool   `json:"btmSupported"`
	FTSupported  bool   `json:"ftSupported"`
	WPSEnabled   bool   `json:"wpsEnabled"`
	// ChannelWidthMHz is the operating width (20/40/80/160/320), 0 if unknown.
	ChannelWidthMHz int `json:"channelWidthMhz"`
	// ChannelUtil is the BSS Load channel-utilization figure (0-255); valid only
	// when HasBSSLoad. AdvertisedStations is the AP's advertised association count.
	ChannelUtil        int  `json:"channelUtil"`
	AdvertisedStations int  `json:"advertisedStations"`
	HasBSSLoad         bool `json:"hasBssLoad"`
	SignalDBm          int  `json:"signalDbm"`
	Beacons            int  `json:"beacons"`
	// RecentDeauths is the number of deauth/disassoc frames seen for this BSSID
	// within the current retention window; a spike feeds the deauth-flood rule.
	RecentDeauths int           `json:"recentDeauths"`
	LastSeen      time.Time     `json:"lastSeen"`
	Stations      []StationView `json:"stations"`
}
