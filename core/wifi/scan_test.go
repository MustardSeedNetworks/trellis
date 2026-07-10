// SPDX-License-Identifier: BUSL-1.1

package wifi

import (
	"encoding/json"
	"testing"
	"time"
)

// TestScannedNetworkJSONRoundTrip pins the wire contract: the JSON tags must
// match Seed's original ScannedNetwork so AirMapper imports and survey
// fixtures produced against Seed round-trip unchanged after the migration.
func TestScannedNetworkJSONRoundTrip(t *testing.T) {
	t.Parallel()
	want := ScannedNetwork{
		SSID:         "MSN-Corp",
		BSSID:        "aa:bb:cc:dd:ee:ff",
		Signal:       -57,
		Channel:      44,
		Frequency:    5220,
		Security:     "WPA3",
		ChannelWidth: 80,
		NoiseFloor:   -95,
		SNR:          38,
		HTMode:       "HE80",
		IsDFS:        true,
		LastSeen:     time.Unix(1_780_000_000, 0).UTC(),
	}

	blob, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Assert the wire field names are the lowerCamel Seed contract, not Go
	// field names — a rename here would silently break imported survey data.
	for _, key := range []string{
		`"ssid"`, `"bssid"`, `"signal"`, `"channel"`, `"frequency"`,
		`"security"`, `"channelWidth"`, `"noiseFloor"`, `"snr"`, `"htMode"`, `"isDFS"`, `"lastSeen"`,
	} {
		if !containsKey(blob, key) {
			t.Errorf("marshaled JSON missing wire key %s: %s", key, blob)
		}
	}

	var got ScannedNetwork
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != want {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", got, want)
	}
}

func containsKey(b []byte, key string) bool {
	s := string(b)
	for i := 0; i+len(key) <= len(s); i++ {
		if s[i:i+len(key)] == key {
			return true
		}
	}
	return false
}
