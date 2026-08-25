// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package capture

import (
	"testing"
	"unsafe"
)

// WLAN_BSS_ENTRY is read straight out of memory wlanapi allocated, so its
// layout is a contract with the OS rather than a Go detail. One wrong offset
// does not fail loudly — every field after the mistake is read from the wrong
// bytes, and the survey records plausible nonsense. These are the offsets from
// wlanapi.h under MSVC's default packing on a 64-bit build.
func TestWLANBSSEntryLayout(t *testing.T) {
	t.Parallel()

	if got, want := unsafe.Sizeof(wlanBSSEntry{}), uintptr(360); got != want {
		t.Errorf("sizeof(WLAN_BSS_ENTRY) = %d, want %d", got, want)
	}

	tests := []struct {
		field string
		got   uintptr
		want  uintptr
	}{
		{"dot11Ssid", unsafe.Offsetof(wlanBSSEntry{}.SSID), 0},
		{"uPhyId", unsafe.Offsetof(wlanBSSEntry{}.PhyID), 36},
		{"dot11Bssid", unsafe.Offsetof(wlanBSSEntry{}.BSSID), 40},
		{"dot11BssType", unsafe.Offsetof(wlanBSSEntry{}.BSSType), 48},
		{"dot11BssPhyType", unsafe.Offsetof(wlanBSSEntry{}.BSSPhyType), 52},
		{"lRssi", unsafe.Offsetof(wlanBSSEntry{}.RSSI), 56},
		{"uLinkQuality", unsafe.Offsetof(wlanBSSEntry{}.LinkQuality), 60},
		{"bInRegDomain", unsafe.Offsetof(wlanBSSEntry{}.InRegDomain), 64},
		{"usBeaconPeriod", unsafe.Offsetof(wlanBSSEntry{}.BeaconPeriod), 66},
		{"ullTimestamp", unsafe.Offsetof(wlanBSSEntry{}.Timestamp), 72},
		{"ullHostTimestamp", unsafe.Offsetof(wlanBSSEntry{}.HostTimestamp), 80},
		{"usCapabilityInformation", unsafe.Offsetof(wlanBSSEntry{}.CapabilityInformation), 88},
		{"ulChCenterFrequency", unsafe.Offsetof(wlanBSSEntry{}.ChCenterFrequency), 92},
		{"wlanRateSet", unsafe.Offsetof(wlanBSSEntry{}.RateSet), 96},
		{"ulIeOffset", unsafe.Offsetof(wlanBSSEntry{}.IeOffset), 352},
		{"ulIeSize", unsafe.Offsetof(wlanBSSEntry{}.IeSize), 356},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("offsetof(WLAN_BSS_ENTRY.%s) = %d, want %d", tt.field, tt.got, tt.want)
		}
	}
}

func TestSupportingStructLayout(t *testing.T) {
	t.Parallel()

	if got, want := unsafe.Sizeof(dot11SSID{}), uintptr(36); got != want {
		t.Errorf("sizeof(DOT11_SSID) = %d, want %d", got, want)
	}
	if got, want := unsafe.Sizeof(wlanRateSet{}), uintptr(256); got != want {
		t.Errorf("sizeof(WLAN_RATE_SET) = %d, want %d", got, want)
	}
	// WLAN_INTERFACE_INFO: GUID(16) + WCHAR[256](512) + enum(4).
	if got, want := unsafe.Sizeof(wlanInterfaceInfo{}), uintptr(532); got != want {
		t.Errorf("sizeof(WLAN_INTERFACE_INFO) = %d, want %d", got, want)
	}
	// The offset bssList() uses to reach the first entry past
	// dwTotalSize + dwNumberOfItems.
	if got := unsafe.Alignof(wlanBSSEntry{}); got != 8 {
		t.Errorf("alignof(WLAN_BSS_ENTRY) = %d, want 8 — the array would not start at offset 8", got)
	}
}
