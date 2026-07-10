// SPDX-License-Identifier: BUSL-1.1

package survey_test

import (
	"strings"
	"testing"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// TestMeasuredSurveyPipelineEndToEnd is the proof-of-life for Trellis's lifted
// measured-survey engine: it drives a realistic passive survey through the
// whole pipeline — sample extraction → IDW interpolation → heatmap raster →
// dead-zone / coverage analysis → PDF report — and asserts real behavior at
// each stage, not just that the code compiles.
//
// The scene: a single 2000×1500 px floor with one strong AP near the top-left
// corner. Passive samples fall off with distance from that corner, so the
// engine must (a) render a heatmap whose min/max span the observed RSSI, and
// (b) flag the far (bottom-right) corner as a dead zone below the threshold.
func TestMeasuredSurveyPipelineEndToEnd(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	// (x, y) px → strongest-AP RSSI dBm. Strong near the top-left AP, fading to
	// a clear dead zone in the bottom-right.
	points := []struct {
		x, y  int
		rssi  int
		ssid  string
		bssid string
		chan_ int
		freq  int
		width int
		noise int
	}{
		{100, 100, -42, "MSN-Corp", "aa:bb:cc:00:00:01", 36, 5180, 80, -95},
		{600, 200, -55, "MSN-Corp", "aa:bb:cc:00:00:01", 36, 5180, 80, -95},
		{1100, 300, -68, "MSN-Corp", "aa:bb:cc:00:00:01", 36, 5180, 80, -95},
		{300, 700, -60, "MSN-Corp", "aa:bb:cc:00:00:01", 36, 5180, 80, -95},
		{900, 900, -74, "MSN-Corp", "aa:bb:cc:00:00:01", 36, 5180, 80, -95},
		{1900, 1400, -88, "MSN-Corp", "aa:bb:cc:00:00:01", 36, 5180, 80, -95}, // dead corner
	}

	samples := make([]*survey.SamplePoint, 0, len(points))
	for _, p := range points {
		net := &wifi.ScannedNetwork{
			SSID: p.ssid, BSSID: p.bssid, Signal: p.rssi,
			Channel: p.chan_, Frequency: p.freq, ChannelWidth: p.width,
			NoiseFloor: p.noise, SNR: p.rssi - p.noise, Security: "WPA3",
			HTMode: "HE80", LastSeen: now,
		}
		samples = append(samples, &survey.SamplePoint{
			X: p.x, Y: p.y, Timestamp: now,
			SampleData: &survey.PassiveSample{
				Networks:     []*wifi.ScannedNetwork{net},
				UniqueSSIDs:  1,
				UniqueBSSIDs: 1,
				APCount5:     1,
			},
		})
	}

	svy := &survey.Survey{
		ID:         "e2e-everett-like",
		Name:       "E2E measured survey",
		SurveyType: survey.TypePassive,
		Status:     survey.StatusCompleted,
		CreatedAt:  now,
		UpdatedAt:  now,
		Floors: []*survey.Floor{{
			ID:    "floor-1",
			Name:  "Floor 1",
			Level: 1,
			FloorPlan: &survey.FloorPlan{
				Width: 2000, Height: 1500, ScaleM: 0.05, // 5 cm/px
			},
			Samples:   samples,
			CreatedAt: now,
			UpdatedAt: now,
		}},
		ActiveFloorID: "floor-1",
	}

	// --- Stage 1: sample extraction ----------------------------------------
	extracted := survey.ExtractSamplesFromSurvey(svy, string(survey.HeatmapRSSI))
	if len(extracted) != len(points) {
		t.Fatalf("ExtractSamplesFromSurvey: got %d samples, want %d", len(extracted), len(points))
	}

	// --- Stage 2 + 3: heatmap (extraction → IDW → raster) ------------------
	cfg := survey.DefaultHeatmapConfig()
	hm, err := survey.GenerateHeatmap(svy, cfg)
	if err != nil {
		t.Fatalf("GenerateHeatmap: %v", err)
	}
	if len(hm.Image) == 0 || !strings.HasPrefix(string(hm.Image[:8]), "\x89PNG") {
		t.Errorf("heatmap image is not a PNG (len=%d)", len(hm.Image))
	}
	if hm.Width <= 0 || hm.Height <= 0 {
		t.Errorf("heatmap dims non-positive: %dx%d", hm.Width, hm.Height)
	}
	if hm.SampleCount != len(points) {
		t.Errorf("heatmap SampleCount = %d, want %d", hm.SampleCount, len(points))
	}
	// The interpolated grid must span the observed RSSI range (strongest -42,
	// weakest -88); IDW never extrapolates beyond the input extremes.
	if hm.Stats.Min < -88 || hm.Stats.Min > -42 {
		t.Errorf("heatmap Stats.Min = %.1f, want within observed [-88,-42]", hm.Stats.Min)
	}
	if hm.Stats.Max > -42 || hm.Stats.Max < -88 {
		t.Errorf("heatmap Stats.Max = %.1f, want within observed [-88,-42]", hm.Stats.Max)
	}
	if hm.Stats.Max <= hm.Stats.Min {
		t.Errorf("heatmap has no gradient: min=%.1f max=%.1f", hm.Stats.Min, hm.Stats.Max)
	}

	// --- Stage 4: dead-zone + coverage analysis ----------------------------
	// Threshold -75 dBm (typical "usable Wi-Fi" floor): the -88 corner must
	// register as a dead zone. AnomalyDetector is nil (that engine stays in
	// Seed); dead-zone/coverage analysis must still work.
	dz, err := survey.DetectDeadZones(svy, -75, nil)
	if err != nil {
		t.Fatalf("DetectDeadZones: %v", err)
	}
	if len(dz.DeadZones) == 0 {
		t.Errorf("expected at least one dead zone below -75 dBm, got none")
	}
	if dz.CoverageScore < 0 || dz.CoverageScore > 100 {
		t.Errorf("CoverageScore = %.1f, want 0..100", dz.CoverageScore)
	}
	// Not every point is dead, so coverage should be neither 0 nor 100.
	if dz.CoverageScore == 0 || dz.CoverageScore == 100 {
		t.Errorf("CoverageScore = %.1f, expected a partial score for a mixed survey", dz.CoverageScore)
	}

	// --- Stage 5: PDF report -----------------------------------------------
	gen := survey.NewReportGenerator(svy, survey.DefaultReportOptions())
	pdf, err := gen.Generate()
	if err != nil {
		t.Fatalf("report Generate: %v", err)
	}
	if len(pdf) == 0 || string(pdf[:5]) != "%PDF-" {
		t.Errorf("report is not a PDF (len=%d)", len(pdf))
	}
}
