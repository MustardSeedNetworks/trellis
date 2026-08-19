// SPDX-License-Identifier: BUSL-1.1

package survey_test

import (
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// TestHeatmapSurvivesReload proves a stored survey still yields a heatmap and a
// coverage analysis after a restart.
//
// A survey's SampleData is typed `any`, so a loaded survey hands back a
// map[string]any rather than a *PassiveSample and the value extractor takes its
// JSON path. That path read a "rssi" key, which nothing writes — the field
// marshals as "signal" — so every point read as NaN and both callers reported
// "no samples found" for a survey whose samples were all present on disk. The
// failure only appeared across a process boundary, which is why it survived a
// suite that keeps one manager in memory.
func TestHeatmapSurvivesReload(t *testing.T) {
	dir := t.TempDir()
	mgr := survey.NewManager(dir, nil, nil, nil, nil)

	svy, err := mgr.CreateSurvey("Everett HQ", "", "wlan0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	if err := mgr.StartSurvey(svy.ID); err != nil {
		t.Fatalf("StartSurvey: %v", err)
	}

	points := []struct{ x, y, rssi int }{
		{100, 100, -42}, {400, 120, -55}, {800, 160, -63},
		{150, 400, -58}, {600, 420, -47}, {900, 460, -88},
	}
	for _, p := range points {
		network := &wifi.ScannedNetwork{
			SSID: "MSN-Corp", BSSID: "aa:bb:cc:00:00:01", Signal: p.rssi,
			Channel: 36, Frequency: 5180, ChannelWidth: 80,
			NoiseFloor: -95, SNR: p.rssi + 95,
		}
		sample := &survey.PassiveSample{
			Networks: []*wifi.ScannedNetwork{network}, UniqueSSIDs: 1, UniqueBSSIDs: 1, APCount5: 1,
		}
		if err := mgr.AddSample(svy.ID, p.x, p.y, sample); err != nil {
			t.Fatalf("AddSample(%d,%d): %v", p.x, p.y, err)
		}
	}

	reopened := survey.NewManager(dir, nil, nil, nil, nil)
	if err := reopened.LoadSurveys(); err != nil {
		t.Fatalf("LoadSurveys: %v", err)
	}
	loaded, err := reopened.GetSurvey(svy.ID)
	if err != nil {
		t.Fatalf("GetSurvey after reload: %v", err)
	}

	for _, metric := range []survey.HeatmapType{survey.HeatmapRSSI, survey.HeatmapSNR} {
		config := survey.DefaultHeatmapConfig()
		config.Type = metric
		result, hmErr := survey.GenerateHeatmap(loaded, config)
		if hmErr != nil {
			t.Fatalf("GenerateHeatmap(%s) after reload: %v", metric, hmErr)
		}
		if result.SampleCount != len(points) {
			t.Errorf("GenerateHeatmap(%s) SampleCount = %d, want %d", metric, result.SampleCount, len(points))
		}
		if result.Stats.Max <= result.Stats.Min {
			t.Errorf("GenerateHeatmap(%s) has no gradient: min=%.1f max=%.1f", metric, result.Stats.Min, result.Stats.Max)
		}
	}

	analysis, err := survey.DetectDeadZones(loaded, -75, nil)
	if err != nil {
		t.Fatalf("DetectDeadZones after reload: %v", err)
	}
	if len(analysis.DeadZones) == 0 {
		t.Error("expected the -88 dBm corner to read as a dead zone after reload, got none")
	}
}
