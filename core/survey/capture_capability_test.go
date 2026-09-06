// SPDX-License-Identifier: BUSL-1.1

package survey_test

import (
	"testing"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// TestHasScannerSaysWhetherTheStoreCanMeasure covers the one question a client
// asks before offering to walk a floor. A store with no backend still serves
// everything else, so this is a capability, not a failure.
func TestHasScannerSaysWhetherTheStoreCanMeasure(t *testing.T) {
	if mustManager(t, t.TempDir(), nil, nil, nil, nil).HasScanner() {
		t.Error("a manager built with no scanner claims one")
	}
	withRadio := mustManager(t, t.TempDir(), &scriptedScanner{}, nil, nil, nil)
	if !withRadio.HasScanner() {
		t.Error("a manager built with a scanner denies it")
	}
}

// TestAThroughputSampleSurvivesAReload guards the hole that has now been closed
// twice in this store: a sample kind whose row is written and whose payload is
// dropped, so the walk looks intact until the daemon restarts and the readings
// are gone. The report's throughput layers are read from exactly this path.
func TestAThroughputSampleSurvivesAReload(t *testing.T) {
	dir := t.TempDir()
	mgr := mustManager(t, dir, nil, nil, nil, nil)
	svy, err := mgr.CreateSurvey("Reloaded", "throughput persistence", "wlan0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	if err := mgr.StartSurvey(svy.ID); err != nil {
		t.Fatalf("StartSurvey: %v", err)
	}
	if err := mgr.AddSample(svy.ID, 12, 34, &survey.ThroughputSample{
		SSID: "ap", BSSID: "00:00:00:00:00:01", RSSI: -52,
		DownloadMbps: 274.5, UploadMbps: 91.25, Latency: 3.5, Jitter: 0.4, PacketLoss: 0.1,
	}); err != nil {
		t.Fatalf("AddSample: %v", err)
	}
	if err := mgr.AddSample(svy.ID, 20, 40, &survey.PassiveSample{
		Networks: []*wifi.ScannedNetwork{{
			SSID: "ap", BSSID: "00:00:00:00:00:01", Signal: -52,
			Channel: 36, Frequency: 5180, LastSeen: time.Now().UTC(),
		}},
	}); err != nil {
		t.Fatalf("AddSample passive: %v", err)
	}
	if err := mgr.CompleteSurvey(svy.ID); err != nil {
		t.Fatalf("CompleteSurvey: %v", err)
	}
	if err := mgr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// A second manager over the same directory is what an operator gets after
	// restarting the daemon.
	reopened := mustManager(t, dir, nil, nil, nil, nil)
	if err := reopened.LoadSurveys(); err != nil {
		t.Fatalf("LoadSurveys: %v", err)
	}
	loaded, err := reopened.GetSurvey(svy.ID)
	if err != nil {
		t.Fatalf("GetSurvey: %v", err)
	}
	floor := loaded.GetActiveFloor()
	if floor == nil || len(floor.Samples) != 2 {
		t.Fatalf("samples after reload = %v", floor)
	}

	var throughput *survey.ThroughputSample
	for _, sample := range floor.Samples {
		if got, ok := sample.SampleData.(*survey.ThroughputSample); ok {
			throughput = got
		}
	}
	if throughput == nil {
		t.Fatal("the throughput sample came back as a different kind, or as nothing")
	}
	if throughput.DownloadMbps != 274.5 || throughput.UploadMbps != 91.25 {
		t.Errorf("readings after reload = %.2f down / %.2f up, want 274.50 / 91.25",
			throughput.DownloadMbps, throughput.UploadMbps)
	}
}
