// SPDX-License-Identifier: BUSL-1.1

package survey_test

import (
	"testing"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// measuredAndAttempted is one real reading at (10,10) and one attempt far out
// at (900,900). Anything that reads the attempt as a measurement moves a
// number: the canvas grows to reach it, or a statistic is divided by two.
func measuredAndAttempted() *survey.Floor {
	return &survey.Floor{
		ID:    "floor-1",
		Name:  "Ground",
		Level: 1,
		Samples: []*survey.SamplePoint{
			{
				X: 10, Y: 10, Timestamp: time.Now(),
				SampleData: &survey.PassiveSample{
					Networks: []*wifi.ScannedNetwork{
						{SSID: "msn", BSSID: "aa:bb:cc:dd:ee:ff", Signal: -55, Channel: 36, SNR: 30},
					},
				},
			},
			{
				X: 900, Y: 900, Timestamp: time.Now(),
				Failed: &survey.Attempt{Kind: "throughput", Reason: "connection refused"},
			},
		},
	}
}

func TestAttemptDoesNotStretchTheHeatmapCanvas(t *testing.T) {
	t.Parallel()

	floor := measuredAndAttempted()
	// The canvas is the area that was surveyed. An attempt measured nothing,
	// so it cannot enlarge the area a survey claims to cover — the grid would
	// be mostly interpolation reaching towards a point with no value.
	cfg := survey.DefaultHeatmapConfig()
	cfg.Type = survey.HeatmapRSSI
	hm, err := survey.GenerateFloorHeatmap(floor, cfg)
	if err != nil {
		t.Fatalf("GenerateFloorHeatmap: %v", err)
	}
	if hm.Width > 500 || hm.Height > 500 {
		t.Errorf("canvas %dx%d reaches the attempted point at (900,900)", hm.Width, hm.Height)
	}
}

func TestAttemptDoesNotMoveTheStatistics(t *testing.T) {
	t.Parallel()

	withAttempt := measuredAndAttempted()
	measuredOnly := &survey.Floor{ID: "floor-1", Samples: withAttempt.Samples[:1]}

	a := survey.ExportCalculateSurveyStats(withAttempt.Samples)
	b := survey.ExportCalculateSurveyStats(measuredOnly.Samples)
	if a != b {
		t.Errorf("stats with an attempt = %+v, without = %+v; an attempt is not a measurement", a, b)
	}
}
