// SPDX-License-Identifier: BUSL-1.1

package capture

import (
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

func TestChannelToFrequency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		channel int
		band    int
		want    int
	}{
		{"2.4GHz ch1", 1, band24GHz, 2412},
		{"2.4GHz ch11", 11, band24GHz, 2462},
		{"2.4GHz ch14 is the exception", 14, band24GHz, 2484},
		{"5GHz ch36", 36, band5GHz, 5180},
		{"5GHz ch149", 149, band5GHz, 5745},
		// Channel numbers collide across bands: 6 GHz channel 1 is 5955 MHz,
		// not the 2.4 GHz 2412 MHz. The band must drive the conversion.
		{"6GHz ch1 does not collide with 2.4GHz", 1, band6GHz, 5955},
		{"6GHz ch233", 233, band6GHz, 7115},
		// A band the driver did not report cannot be guessed from the channel.
		{"unknown band yields zero", 149, 0, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := channelToFrequency(tc.channel, tc.band); got != tc.want {
				t.Errorf("channelToFrequency(%d, %d) = %d, want %d",
					tc.channel, tc.band, got, tc.want)
			}
		})
	}
}

func TestHTModeForWidth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		width int
		want  string
	}{
		{width20MHz, "HT20"},
		{width40MHz, "HT40"},
		{width80MHz, "VHT80"},
		{width160MHz, "HE160"},
		{width320MHz, "EHT320"},
		// An unreported width is treated as the narrowest, not as unknown.
		{0, "HT20"},
	}

	for _, tc := range tests {
		if got := htModeForWidth(tc.width); got != tc.want {
			t.Errorf("htModeForWidth(%d) = %q, want %q", tc.width, got, tc.want)
		}
	}
}

func TestIsDFSChannel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		channel int
		want    bool
	}{
		{36, false},  // UNII-1, no radar detection
		{52, true},   // UNII-2 lower bound
		{64, true},   // UNII-2 upper bound
		{100, true},  // UNII-2 Extended lower bound
		{144, true},  // UNII-2 Extended upper bound
		{149, false}, // UNII-3
		{165, false},
	}

	for _, tc := range tests {
		if got := isDFSChannel(tc.channel); got != tc.want {
			t.Errorf("isDFSChannel(%d) = %v, want %v", tc.channel, got, tc.want)
		}
	}
}

// TestWithAssociation covers marking the BSS a host is joined to, the fact a
// live view needs and a scan alone does not carry on every platform.
func TestWithAssociation(t *testing.T) {
	t.Parallel()

	scan := func() []wifi.ScannedNetwork {
		return []wifi.ScannedNetwork{
			{SSID: "lab", BSSID: "02:00:00:00:00:01", Signal: -48},
			{SSID: "lab", BSSID: "02:00:00:00:00:02", Signal: -62},
		}
	}

	t.Run("no association leaves the scan alone", func(t *testing.T) {
		t.Parallel()

		got := withAssociation(scan(), nil)
		if len(got) != 2 {
			t.Fatalf("networks = %d, want 2", len(got))
		}
		for _, n := range got {
			if n.Associated {
				t.Errorf("%s reported associated with no association", n.BSSID)
			}
		}
	})

	t.Run("marks the observed BSS rather than duplicating it", func(t *testing.T) {
		t.Parallel()

		// The driver's own casing is not guaranteed to match the scan's, and a
		// case-sensitive compare would show the connection as a second entry.
		current := wifi.ScannedNetwork{SSID: "lab", BSSID: "02:00:00:00:00:02", Signal: -60}
		got := withAssociation(scan(), &current)

		if len(got) != 2 {
			t.Fatalf("networks = %d, want 2 — the association duplicated a scanned BSS", len(got))
		}
		if got[0].Associated || !got[1].Associated {
			t.Fatalf("associated flags = %v/%v, want false/true", got[0].Associated, got[1].Associated)
		}
		// The scan's own reading wins: it and every other entry were measured
		// in the same sweep, and mixing a reading from another moment into the
		// list would make the connected AP incomparable with its neighbours.
		if got[1].Signal != -62 {
			t.Errorf("signal = %d dBm, want the scan's -62", got[1].Signal)
		}
	})

	t.Run("uppercase BSSID still matches", func(t *testing.T) {
		t.Parallel()

		current := wifi.ScannedNetwork{BSSID: "02:00:00:00:00:01"}
		got := withAssociation([]wifi.ScannedNetwork{{BSSID: "02:00:00:00:00:01"}}, &current)
		if len(got) != 1 || !got[0].Associated {
			t.Fatalf("got %+v, want the single BSS marked associated", got)
		}
	})

	t.Run("an association the scan missed is still reported", func(t *testing.T) {
		t.Parallel()

		// A scan can come back without the BSS the host is joined to — a sweep
		// that missed a beacon, or a driver that omits the serving AP. Dropping
		// the association there would blank the one row the operator most wants.
		current := wifi.ScannedNetwork{SSID: "other", BSSID: "02:00:00:00:00:09", Signal: -55}
		got := withAssociation(scan(), &current)

		if len(got) != 3 {
			t.Fatalf("networks = %d, want 3", len(got))
		}
		if !got[2].Associated || got[2].BSSID != "02:00:00:00:00:09" {
			t.Errorf("appended = %+v, want the association marked", got[2])
		}
	})
}

// TestDedupeBSSes covers a real CoreWLAN result: scanning this office returned
// the same hidden BSS three times, identically. Left alone that is three rows
// in the live view for one AP, three co-channel neighbours in a survey point's
// aggregation, and a duplicate React key.
func TestDedupeBSSes(t *testing.T) {
	t.Parallel()

	t.Run("keeps one entry per BSS", func(t *testing.T) {
		t.Parallel()

		got := dedupeBSSes([]wifi.ScannedNetwork{
			{BSSID: "26:5a:4c:2b:71:7d", Signal: -83, Channel: 153},
			{BSSID: "26:5a:4c:2b:71:7d", Signal: -83, Channel: 153},
			{BSSID: "24:5a:4c:6b:b5:c8", SSID: "lab", Signal: -50},
		})
		if len(got) != 2 {
			t.Fatalf("networks = %d, want 2", len(got))
		}
	})

	t.Run("keeps the strongest observation", func(t *testing.T) {
		t.Parallel()

		// Observations of one BSS differ by a few dB. Keeping the first would
		// make the reading depend on the order the radio happened to report.
		got := dedupeBSSes([]wifi.ScannedNetwork{
			{BSSID: "aa:bb:cc:00:00:01", Signal: -80},
			{BSSID: "aa:bb:cc:00:00:01", Signal: -62},
			{BSSID: "aa:bb:cc:00:00:01", Signal: -75},
		})
		if len(got) != 1 || got[0].Signal != -62 {
			t.Fatalf("got %+v, want a single BSS at -62 dBm", got)
		}
	})

	t.Run("a shared BSSID under two SSIDs is two BSSs", func(t *testing.T) {
		t.Parallel()

		// Multi-SSID radios do this. Collapsing on the BSSID alone would drop a
		// network that is genuinely on the air.
		got := dedupeBSSes([]wifi.ScannedNetwork{
			{BSSID: "aa:bb:cc:00:00:01", SSID: "corp"},
			{BSSID: "aa:bb:cc:00:00:01", SSID: "guest"},
		})
		if len(got) != 2 {
			t.Fatalf("networks = %d, want 2", len(got))
		}
	})

	t.Run("preserves the radio's order", func(t *testing.T) {
		t.Parallel()

		got := dedupeBSSes([]wifi.ScannedNetwork{
			{BSSID: "aa:bb:cc:00:00:01", Signal: -80},
			{BSSID: "aa:bb:cc:00:00:02", Signal: -40},
			{BSSID: "aa:bb:cc:00:00:01", Signal: -70},
		})
		if len(got) != 2 || got[0].BSSID != "aa:bb:cc:00:00:01" {
			t.Fatalf("got %+v, want the first-seen BSS still first", got)
		}
	})
}
