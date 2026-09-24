package survey

import "testing"

// The raw-data appendix prints what each point measured. A point whose
// capture had no noise floor carries SNR 0, and printing "0" there reads as a
// measured zero-margin point rather than a missing figure (#600).
func TestSNRCell(t *testing.T) {
	t.Parallel()

	tests := []struct {
		snr  int
		want string
	}{
		{snr: 0, want: "-"},
		{snr: 27, want: "27"},
	}
	for _, tt := range tests {
		if got := snrCell(tt.snr); got != tt.want {
			t.Errorf("snrCell(%d) = %q, want %q", tt.snr, got, tt.want)
		}
	}
}
