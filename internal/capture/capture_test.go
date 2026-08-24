// SPDX-License-Identifier: BUSL-1.1

package capture

import "testing"

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
