package survey

import (
	"errors"
	"testing"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

func TestBandLabel(t *testing.T) {
	tests := []struct {
		freq, ch int
		want     string
	}{
		{2412, 1, "2.4 GHz"},
		{2484, 14, "2.4 GHz"},
		{5180, 36, "5 GHz"},
		{5955, 1, "6 GHz"},
		{0, 6, "2.4 GHz"},  // frequency missing, channel fallback
		{0, 36, "Unknown"}, // missing frequency, non-2.4 channel
		{3000, 0, "Unknown"},
	}
	for _, tc := range tests {
		if got := bandLabel(tc.freq, tc.ch); got != tc.want {
			t.Errorf("bandLabel(%d,%d) = %q, want %q", tc.freq, tc.ch, got, tc.want)
		}
	}
}

func surveyWith(samples ...*SamplePoint) *Survey {
	floor := &Floor{ID: "f1", Name: "Floor 1", Level: 1, Samples: samples}
	return &Survey{
		ID:            "s1",
		Floors:        []*Floor{floor},
		ActiveFloorID: floor.ID,
		UpdatedAt:     time.Unix(500, 0),
	}
}

func TestBSSViewsDedupKeepsStrongest(t *testing.T) {
	now := time.Unix(1000, 0)
	later := time.Unix(2000, 0)
	s := surveyWith(
		&SamplePoint{Timestamp: now, SampleData: &PassiveSample{Networks: []*wifi.ScannedNetwork{
			{SSID: "corp", BSSID: "aa:bb:cc:00:00:01", Signal: -80, Channel: 6, Frequency: 2437, Security: "WPA2"},
		}}},
		&SamplePoint{Timestamp: later, SampleData: &PassiveSample{Networks: []*wifi.ScannedNetwork{
			{SSID: "corp", BSSID: "aa:bb:cc:00:00:01", Signal: -55, Channel: 6, Frequency: 2437, Security: "WPA2"},
		}}},
	)

	views, at := bssViews(s.GetAllSamples())
	if len(views) != 1 {
		t.Fatalf("views = %d, want 1 (deduped by BSSID)", len(views))
	}
	if views[0].SignalDBm != -55 {
		t.Errorf("kept signal = %d, want strongest -55", views[0].SignalDBm)
	}
	if views[0].Band != "2.4 GHz" {
		t.Errorf("band = %q, want 2.4 GHz", views[0].Band)
	}
	if !at.Equal(later) {
		t.Errorf("latest timestamp = %v, want %v", at, later)
	}
}

// stubDetector is a test double for AnomalyDetector. Seed's real detection
// rules (internal/anomaly + internal/wifi/anomaly) have not been ported to
// Trellis yet (TODO(trellis-anomaly)), so these tests exercise the
// survey-side plumbing — the BSS views AnalyzeAnomalies builds and hands to
// the detector — rather than a real open-network detection rule.
type stubDetector struct {
	got []wifi.BSSView
	at  time.Time
}

func (d *stubDetector) AnalyzeBSSes(bsses []wifi.BSSView, at time.Time) ([]wifi.Anomaly, error) {
	d.got = bsses
	d.at = at
	for _, b := range bsses {
		if b.Security == "Open" {
			return []wifi.Anomaly{{DefKey: wifi.DefOpenNetwork, Subject: wifi.SubjectRef{Kind: wifi.SubjectBSSID, ID: b.BSSID}}}, nil
		}
	}
	return nil, nil
}

func TestAnalyzeAnomaliesDetectsOpenNetwork(t *testing.T) {
	s := surveyWith(
		&SamplePoint{Timestamp: time.Unix(1000, 0), SampleData: &PassiveSample{Networks: []*wifi.ScannedNetwork{
			{SSID: "guest", BSSID: "aa:bb:cc:00:00:01", Signal: -60, Channel: 1, Frequency: 2412, Security: "Open"},
		}}},
	)

	det := &stubDetector{}
	anoms, err := AnalyzeAnomalies(s, det)
	if err != nil {
		t.Fatalf("AnalyzeAnomalies: %v", err)
	}
	if len(det.got) != 1 || det.got[0].Security != "Open" {
		t.Fatalf("detector received %+v, want one Open BSSView", det.got)
	}
	found := false
	for _, a := range anoms {
		if a.DefKey == wifi.DefOpenNetwork {
			found = true
		}
	}
	if !found {
		t.Errorf("expected %s anomaly, got %+v", wifi.DefOpenNetwork, anoms)
	}
}

func TestAnalyzeAnomaliesNilDetector(t *testing.T) {
	s := surveyWith(
		&SamplePoint{Timestamp: time.Unix(1000, 0), SampleData: &PassiveSample{Networks: []*wifi.ScannedNetwork{
			{SSID: "guest", BSSID: "aa:bb:cc:00:00:01", Signal: -60, Channel: 1, Frequency: 2412, Security: "Open"},
		}}},
	)

	anoms, err := AnalyzeAnomalies(s, nil)
	if err != nil {
		t.Fatalf("AnalyzeAnomalies: %v", err)
	}
	if len(anoms) != 0 {
		t.Errorf("anomalies = %v, want none (no detector wired)", anoms)
	}
}

func TestAnalyzeAnomaliesNoPassiveSamples(t *testing.T) {
	s := surveyWith(&SamplePoint{
		Timestamp:  time.Unix(1000, 0),
		SampleData: &ActiveSample{SSID: "corp", BSSID: "aa:bb:cc:00:00:01", RSSI: -50},
	})
	det := &failDetector{t: t}
	anoms, err := AnalyzeAnomalies(s, det)
	if err != nil {
		t.Fatalf("AnalyzeAnomalies: %v", err)
	}
	if len(anoms) != 0 {
		t.Errorf("anomalies = %v, want none (no passive APs)", anoms)
	}
}

// failDetector fails the test if AnalyzeBSSes is ever called — used to assert
// AnalyzeAnomalies short-circuits before invoking the detector.
type failDetector struct{ t *testing.T }

func (d *failDetector) AnalyzeBSSes([]wifi.BSSView, time.Time) ([]wifi.Anomaly, error) {
	d.t.Helper()
	d.t.Fatal("AnalyzeBSSes should not be called when a survey has no passive samples")
	return nil, errors.New("unreachable")
}
