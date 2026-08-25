// SPDX-License-Identifier: BUSL-1.1

package capture

import "testing"

func TestChannelForFrequency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		freq        int
		wantChannel int
		wantBand    int
	}{
		{"2.4 GHz channel 1", 2412, 1, band24GHz},
		{"2.4 GHz channel 6", 2437, 6, band24GHz},
		{"2.4 GHz channel 11", 2462, 11, band24GHz},
		// Channel 14 is 12 MHz above 13, not 5, so the arithmetic that works
		// for every other 2.4 GHz channel gives 15 here.
		{"2.4 GHz channel 14 is not on the 5 MHz grid", 2484, 14, band24GHz},
		{"5 GHz channel 36", 5180, 36, band5GHz},
		{"5 GHz channel 52 (DFS)", 5260, 52, band5GHz},
		{"5 GHz channel 165", 5825, 165, band5GHz},
		// The band that makes a frequency-to-channel table necessary: read as
		// 5 GHz this would be channel 191.
		{"6 GHz channel 1 is not 5 GHz channel 191", 5955, 1, band6GHz},
		{"6 GHz channel 37", 6135, 37, band6GHz},
		{"6 GHz channel 233", 7115, 233, band6GHz},
		{"below every band", 2400, 0, 0},
		{"in the 5/6 GHz gap", 5900, 0, 0},
		{"above every band", 7200, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			channel, band := channelForFrequency(tt.freq)
			if channel != tt.wantChannel || band != tt.wantBand {
				t.Errorf("channelForFrequency(%d) = (%d, %d), want (%d, %d)",
					tt.freq, channel, band, tt.wantChannel, tt.wantBand)
			}
		})
	}
}

// Every channel the scanner can report must survive a round trip, or a Linux
// or Windows sample would land on a different channel than the same BSS
// observed from macOS.
func TestFrequencyChannelRoundTrip(t *testing.T) {
	t.Parallel()

	bands := map[int][]int{
		band24GHz: {1, 6, 11, 14},
		band5GHz:  {36, 44, 52, 100, 149, 165},
		band6GHz:  {1, 37, 101, 233},
	}

	for band, channels := range bands {
		for _, want := range channels {
			freq := channelToFrequency(want, band)
			got, gotBand := channelForFrequency(freq)
			if got != want || gotBand != band {
				t.Errorf("channel %d band %d -> %d MHz -> (%d, %d), want (%d, %d)",
					want, band, freq, got, gotBand, want, band)
			}
		}
	}
}
