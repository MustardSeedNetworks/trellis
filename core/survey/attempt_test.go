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

func TestAttemptIsNotCountedAsASample(t *testing.T) {
	t.Parallel()

	floor := measuredAndAttempted()
	// One reading and one attempt. A rail, a summary or a report header that
	// says "2 samples" while the layer draws one is telling the operator the
	// survey measured something it did not.
	if got := len(floor.MeasuredSamples()); got != 1 {
		t.Errorf("MeasuredSamples = %d, want 1: the attempt measured nothing", got)
	}

	s := &survey.Survey{ID: "s1", Floors: []*survey.Floor{floor}}
	if got := len(s.GetAllMeasuredSamples()); got != 1 {
		t.Errorf("GetAllMeasuredSamples = %d, want 1", got)
	}
	// The points themselves are all still there: the map draws the attempt.
	if got := len(s.GetAllSamples()); got != 2 {
		t.Errorf("GetAllSamples = %d, want both points", got)
	}
}

func TestAReportOnAFloorOfNothingButAttempts(t *testing.T) {
	t.Parallel()

	floor := &survey.Floor{
		ID: "floor-1", Name: "Ground", Level: 1,
		Samples: []*survey.SamplePoint{
			{X: 20, Y: 20, Timestamp: time.Now(),
				Failed: &survey.Attempt{Kind: "throughput", Reason: "connection refused"}},
		},
	}
	s := &survey.Survey{
		ID: "s1", Name: "All attempts", SurveyType: survey.TypeThroughput,
		Status: survey.StatusCompleted, Floors: []*survey.Floor{floor},
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	// Nothing was measured on this floor, so there is no map to draw and no
	// statistic to state — but the report still has to be produced. Gating the
	// map on "has points" rather than "has measurements" walks into a heatmap
	// with no dimensions.
	pdf, err := survey.NewReportGenerator(s, survey.DefaultReportOptions()).Generate()
	if err != nil {
		t.Fatalf("GenerateReport on a floor of attempts: %v", err)
	}
	if len(pdf) == 0 {
		t.Error("report is empty")
	}
}
