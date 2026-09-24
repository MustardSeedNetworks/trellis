package survey

// report_heatmap_key_test.go pins the printed heatmap key to the step it
// describes (#571).
//
// The coverage ramp steps at the operator's threshold with two stops a
// hundredth of a decibel apart. Printed at whole decibels both read "-60 dBm",
// so the orange and teal swatches either side of the step carried the same
// label and the key could not say which side of the requirement a colour was.
// The assertions read the labels out of the PDF the key actually drew.

import (
	"bytes"
	"regexp"
	"testing"

	"github.com/go-pdf/fpdf"
)

// pdfShownText matches a text-showing operator in an uncompressed content
// stream: "(label)Tj".
var pdfShownText = regexp.MustCompile(`\(((?:[^()\\]|\\.)*)\)\s*Tj`)

func printedHeatmapKey(t *testing.T, scale ColorScale, unit string) []string {
	t.Helper()
	g := &ReportGenerator{pdf: fpdf.New("P", "mm", "A4", "")}
	g.pdf.SetCompression(false)
	g.pdf.AddPage()
	g.addHeatmapKey(scale, unit)

	var out bytes.Buffer
	if err := g.pdf.Output(&out); err != nil {
		t.Fatalf("Output: %v", err)
	}
	var labels []string
	for _, m := range pdfShownText.FindAllSubmatch(out.Bytes(), -1) {
		labels = append(labels, string(m[1]))
	}
	return labels
}

func TestHeatmapKeySaysWhichSideOfTheThresholdEachSwatchIs(t *testing.T) {
	tests := []struct {
		name      string
		metric    HeatmapType
		threshold float64
		unit      string
		want      []string
	}{
		{
			name: "rssi at the operator's threshold", metric: HeatmapRSSI, threshold: -60, unit: "dBm",
			want: []string{"-100 dBm", "-80 dBm", "< -60 dBm", "-60 dBm", "-30 dBm"},
		},
		{
			name: "snr at the operator's threshold", metric: HeatmapSNR, threshold: 25, unit: "dB",
			want: []string{"0 dB", "12.5 dB", "< 25 dB", "25 dB", "50 dB"},
		},
		{
			// A threshold above the range is pulled in to -33.5 dBm. Whole
			// decibels would print the step as "< -34" beside "-34", which is
			// false of the orange swatch: -33.51 is not below -34.
			name: "rssi threshold clamped inside the range", metric: HeatmapRSSI, threshold: -20, unit: "dBm",
			want: []string{"-100 dBm", "-66.8 dBm", "< -33.5 dBm", "-33.5 dBm", "-30 dBm"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := printedHeatmapKey(t, CoverageScale(tt.metric, tt.threshold), tt.unit)
			if len(got) != len(tt.want) {
				t.Fatalf("key printed %q, want %q", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("label %d = %q, want %q (key %q)", i, got[i], tt.want[i], got)
				}
			}
		})
	}
}
