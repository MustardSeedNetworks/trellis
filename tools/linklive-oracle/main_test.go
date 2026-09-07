// SPDX-License-Identifier: BUSL-1.1

package main

import "testing"

// The two sides write the same address differently: Link-Live prefixes the
// vendor and splits the OUI from the NIC with a hyphen, the archive writes
// colon-separated octets. A comparison that missed this would report every
// BSSID as seen by only one side and call it a defect.
func TestNormalizeBSSID(t *testing.T) {
	for _, tc := range []struct {
		name  string
		given string
		want  string
	}{
		{"Link-Live form", "RuckusWi:58b633-c652d9", "58b633c652d9"},
		{"archive form", "58:B6:33:C6:52:D9", "58b633c652d9"},
		{"vendor name holding a hyphen", "Link-Sys:58b633-c652d9", "58b633c652d9"},
		{"neither form", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeBSSID(tc.given); got != tc.want {
				t.Errorf("normalizeBSSID(%q) = %q, want %q", tc.given, got, tc.want)
			}
		})
	}
}
