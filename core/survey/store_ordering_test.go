package survey_test

// store_ordering_test.go pins the strongest-AP-first ordering of
// PassiveSample.Networks.
//
// The heatmap, the coverage score and the report all read Networks[0] as "the
// AP serving this point", and CalculateAggregations measures co-channel
// interference against its channel. Nothing about a scan or a database row
// delivers that order on its own: an AirMapper capture lists BSSes in the order
// the radio saw them, and the store returns them by insertion id. So the order
// is established in code, and this test is what says so.
//
// It asserts values rather than counts. The suite already counted points and
// observations and was green while every point reported the wrong signal.

import (
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

func TestStrongestAPLeadsNetworksAfterReload(t *testing.T) {
	// Deliberately weakest-first, and the strongest AP's channel is the busy
	// one: read in slice order this point reports -80 dBm and two co-channel
	// APs, read strongest-first it reports -35 dBm and three.
	const (
		strongest   = -35
		wantCoChan  = 3
		wantNetsLen = 5
	)
	nets := []*wifi.ScannedNetwork{
		{SSID: "far", BSSID: "00:00:00:00:00:01", Signal: -80, Channel: 1, Frequency: 2412},
		{SSID: "far", BSSID: "00:00:00:00:00:02", Signal: -70, Channel: 1, Frequency: 2412},
		{SSID: "near", BSSID: "00:00:00:00:00:03", Signal: -60, Channel: 11, Frequency: 2462},
		{SSID: "near", BSSID: "00:00:00:00:00:04", Signal: strongest, Channel: 11, Frequency: 2462},
		{SSID: "near", BSSID: "00:00:00:00:00:05", Signal: -50, Channel: 11, Frequency: 2462},
	}

	dir := t.TempDir()
	mgr := mustManager(t, dir, nil, nil, nil, nil)
	svy, err := mgr.CreateSurvey("Ordering", "strongest-first", "wlan0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	if err := mgr.StartSurvey(svy.ID); err != nil {
		t.Fatalf("StartSurvey: %v", err)
	}
	if err := mgr.AddSample(svy.ID, 10, 20, &survey.PassiveSample{Networks: nets}); err != nil {
		t.Fatalf("AddSample: %v", err)
	}

	// Both paths produce a PassiveSample and both must order it: the one the
	// caller just built, and the one read back from SQLite.
	check := func(t *testing.T, path string, s *survey.Survey) {
		t.Helper()
		floor := s.GetActiveFloor()
		if floor == nil || len(floor.Samples) != 1 {
			t.Fatalf("%s: want one point on the active floor", path)
		}
		ps, ok := floor.Samples[0].SampleData.(*survey.PassiveSample)
		if !ok {
			t.Fatalf("%s: point came back as %T, not a passive sample", path, floor.Samples[0].SampleData)
		}
		if len(ps.Networks) != wantNetsLen {
			t.Fatalf("%s: networks = %d, want %d", path, len(ps.Networks), wantNetsLen)
		}
		if got := ps.Networks[0].Signal; got != strongest {
			t.Errorf("%s: Networks[0].Signal = %d dBm, want %d — the slice is not strongest-first", path, got, strongest)
		}
		if got := ps.CoChannelAPs; got != wantCoChan {
			t.Errorf("%s: CoChannelAPs = %d, want %d — counted against the wrong AP's channel", path, got, wantCoChan)
		}
		samples := survey.ExtractSamplesFromSurvey(s, "rssi")
		if len(samples) != 1 {
			t.Fatalf("%s: extracted %d rssi samples, want 1", path, len(samples))
		}
		if got := samples[0].Value; got != strongest {
			t.Errorf("%s: heatmap reads %.0f dBm at this point, want %d", path, got, strongest)
		}
	}

	inMemory, err := mgr.GetSurvey(svy.ID)
	if err != nil {
		t.Fatalf("GetSurvey: %v", err)
	}
	check(t, "in memory", inMemory)

	reopened := mustManager(t, dir, nil, nil, nil, nil)
	if err := reopened.LoadSurveys(); err != nil {
		t.Fatalf("LoadSurveys: %v", err)
	}
	loaded, err := reopened.GetSurvey(svy.ID)
	if err != nil {
		t.Fatalf("GetSurvey after reload: %v", err)
	}
	check(t, "after reload", loaded)
}
