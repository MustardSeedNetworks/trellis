package survey_test

// report_ap_table_test.go pins the per-floor access point table (T-C23): the
// PRD's MVP report is "coverage maps + AP table + summary", and the report had
// no AP table. The assertions read the rows out of the PDF Generate produced.

import (
	"bytes"
	"compress/zlib"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

var (
	pdfStream = regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`)
	pdfShown  = regexp.MustCompile(`\(((?:[^()\\]|\\.)*)\)\s*Tj`)
)

// reportText is every string the report's content streams print, one per line.
func reportText(t *testing.T, pdf []byte) string {
	t.Helper()
	var text strings.Builder
	for _, m := range pdfStream.FindAllSubmatch(pdf, -1) {
		r, err := zlib.NewReader(bytes.NewReader(m[1]))
		if err != nil {
			continue // not a deflated stream
		}
		content, err := io.ReadAll(r)
		if err != nil {
			continue // image data, not text
		}
		for _, shown := range pdfShown.FindAllSubmatch(content, -1) {
			text.Write(shown[1])
			text.WriteByte('\n')
		}
	}
	return text.String()
}

func passivePoint(x int, nets ...*wifi.ScannedNetwork) *survey.SamplePoint {
	return &survey.SamplePoint{
		X: x, Y: x, Timestamp: time.Now(),
		SampleData: &survey.PassiveSample{Networks: nets},
	}
}

func TestReportListsEveryAccessPointHeardOnTheFloor(t *testing.T) {
	const (
		corp24 = "aa:bb:cc:00:00:01"
		corp5  = "aa:bb:cc:00:00:02"
		hidden = "aa:bb:cc:00:00:03"
	)
	a := func(signal int) *wifi.ScannedNetwork {
		return &wifi.ScannedNetwork{
			BSSID: corp24, SSID: "Corp", Signal: signal, Channel: 6, Frequency: 2437, ChannelWidth: 20,
		}
	}
	b := func(signal int) *wifi.ScannedNetwork {
		return &wifi.ScannedNetwork{
			BSSID: corp5, SSID: "Corp", Signal: signal, Channel: 36, Frequency: 5180, ChannelWidth: 80,
		}
	}
	c := &wifi.ScannedNetwork{BSSID: hidden, Signal: -81, Channel: 1, Frequency: 5955}

	floor := &survey.Floor{
		ID: "floor-1", Name: "Ground", Level: 0,
		Samples: []*survey.SamplePoint{
			passivePoint(10, a(-50), b(-72)),
			// The same BSSID twice in one scan is one point heard, at its best.
			passivePoint(20, a(-60), b(-58), b(-80)),
			passivePoint(30, a(-70), c),
			passivePoint(40, a(-65)),
		},
	}
	s := &survey.Survey{
		ID: "s", Name: "AP table", Status: survey.StatusCompleted,
		CreatedAt: time.Now(), Floors: []*survey.Floor{floor},
	}

	pdf, err := survey.NewReportGenerator(s, survey.DefaultReportOptions()).Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	text := reportText(t, pdf)

	// One row per BSSID, most-heard first:
	// BSSID, SSID, band, channel, width, samples, best, median.
	rows := [][]string{
		{corp24, "Corp", "2.4 GHz", "6", "20", "4", "-50", "-63"},
		{corp5, "Corp", "5 GHz", "36", "80", "2", "-58", "-65"},
		{hidden, "-", "6 GHz", "1", "-", "1", "-81", "-81"},
	}
	header := strings.Join([]string{
		"BSSID", "SSID", "Band", "Channel", "Width", "Samples", "Best RSSI", "Median RSSI",
	}, "\n") + "\n"
	want := header
	for _, row := range rows {
		want += strings.Join(row, "\n") + "\n"
	}
	if !strings.Contains(text, want) {
		t.Errorf("report has no AP table matching\n%s\nprinted text:\n%s", want, text)
	}
}
