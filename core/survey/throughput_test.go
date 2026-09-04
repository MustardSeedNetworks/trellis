// SPDX-License-Identifier: BUSL-1.1

package survey_test

import (
	"context"
	"errors"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// scriptedMeter stands in for iperf3.
type scriptedMeter struct {
	sample survey.ThroughputSample
	err    error
	server string
	iface  string
	calls  int
}

func (m *scriptedMeter) Measure(
	_ context.Context,
	iface, server string,
	_ int,
) (survey.ThroughputSample, error) {
	m.calls++
	m.iface, m.server = iface, server
	return m.sample, m.err
}

// throughputSurvey returns a manager with a radio and a meter, and a survey
// already walking with a target to measure against.
func throughputSurvey(t *testing.T, meter survey.ThroughputMeter) (*survey.Manager, string) {
	t.Helper()

	mgr := mustManager(t, t.TempDir(), &countingScanner{}, nil, meter, nil)
	s, err := mgr.CreateSurvey("throughput", "", "en0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	if err := mgr.SetThroughputTarget(s.ID, "10.44.10.9", 5); err != nil {
		t.Fatalf("SetThroughputTarget: %v", err)
	}
	if err := mgr.StartSurvey(s.ID); err != nil {
		t.Fatalf("StartSurvey: %v", err)
	}
	return mgr, s.ID
}

func TestMeasureThroughputStoresTheReadingAtThePoint(t *testing.T) {
	t.Parallel()

	meter := &scriptedMeter{sample: survey.ThroughputSample{DownloadMbps: 221, UploadMbps: 88}}
	mgr, id := throughputSurvey(t, meter)

	got, err := mgr.MeasureThroughput(context.Background(), id, 120, 240)
	if err != nil {
		t.Fatalf("MeasureThroughput: %v", err)
	}
	if got.DownloadMbps != 221 || got.UploadMbps != 88 {
		t.Errorf("sample = %+v, want the meter's reading", got)
	}
	if meter.server != "10.44.10.9" {
		t.Errorf("measured against %q, want the survey's target", meter.server)
	}
	// The survey's own adapter, not whatever the host would route over. On a
	// machine with ethernet as well, an unbound test measures the wire and the
	// throughput layer reports a Wi-Fi number that was never Wi-Fi.
	if meter.iface != "en0" {
		t.Errorf("measured over %q, want the survey's interface en0", meter.iface)
	}
	// The AP the link runs over is what makes a throughput reading comparable
	// with the passive points around it; without it the layer is a set of
	// numbers with no radio behind them.
	if got.BSSID == "" || got.RSSI == 0 {
		t.Errorf("reading carries no association: %+v", got)
	}

	s, err := mgr.GetSurvey(id)
	if err != nil {
		t.Fatalf("GetSurvey: %v", err)
	}
	points := s.GetAllSamples()
	if len(points) != 1 {
		t.Fatalf("%d points stored, want 1", len(points))
	}
	if points[0].X != 120 || points[0].Y != 240 {
		t.Errorf("point at (%d,%d), want (120,240)", points[0].X, points[0].Y)
	}
	if points[0].Interpolated {
		t.Error("a throughput reading is taken standing still; it is not interpolated")
	}
	if _, ok := points[0].SampleData.(*survey.ThroughputSample); !ok {
		t.Errorf("stored %T, want a throughput sample", points[0].SampleData)
	}
}

func TestMeasureThroughputWithoutATarget(t *testing.T) {
	t.Parallel()

	mgr, id := startedSurvey(t, &countingScanner{})
	// A throughput test is against something. Without a server there is no
	// measurement to make, and the operator has to be told which of the two
	// missing things it is.
	if _, err := mgr.MeasureThroughput(context.Background(), id, 1, 1); !errors.Is(err, survey.ErrNoThroughputTarget) {
		t.Fatalf("without a target = %v, want ErrNoThroughputTarget", err)
	}
}

func TestMeasureThroughputWithoutAMeter(t *testing.T) {
	t.Parallel()

	mgr := mustManager(t, t.TempDir(), &countingScanner{}, nil, nil, nil)
	s, _ := mgr.CreateSurvey("no meter", "", "en0", survey.TypePassive)
	_ = mgr.SetThroughputTarget(s.ID, "10.44.10.9", 5)
	_ = mgr.StartSurvey(s.ID)

	if _, err := mgr.MeasureThroughput(context.Background(), s.ID, 1, 1); !errors.Is(err, survey.ErrNoThroughputMeter) {
		t.Fatalf("without a meter = %v, want ErrNoThroughputMeter", err)
	}
}

func TestAFailedMeasurementStoresNothing(t *testing.T) {
	t.Parallel()

	meter := &scriptedMeter{err: errors.New("the server is busy running a test")}
	mgr, id := throughputSurvey(t, meter)

	if _, err := mgr.MeasureThroughput(context.Background(), id, 1, 1); err == nil {
		t.Fatal("want the meter's error")
	}
	// A point stored for a test that failed is a zero-speed reading at a
	// position where nothing was measured — a dead spot the survey invented.
	s, _ := mgr.GetSurvey(id)
	if got := len(s.GetAllSamples()); got != 0 {
		t.Errorf("%d points stored for a failed measurement, want 0", got)
	}
}

func TestThroughputTargetSurvivesAReload(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mgr := mustManager(t, dir, &countingScanner{}, nil, nil, nil)
	s, _ := mgr.CreateSurvey("target", "", "en0", survey.TypePassive)
	if err := mgr.SetThroughputTarget(s.ID, "10.44.10.9", 12); err != nil {
		t.Fatalf("SetThroughputTarget: %v", err)
	}
	if err := mgr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened := mustManager(t, dir, nil, nil, nil, nil)
	if err := reopened.LoadSurveys(); err != nil {
		t.Fatalf("LoadSurveys: %v", err)
	}
	after, err := reopened.GetSurvey(s.ID)
	if err != nil {
		t.Fatalf("GetSurvey: %v", err)
	}
	if after.IperfServer != "10.44.10.9" || after.TestDuration != 12 {
		t.Errorf("target after reload = %q/%ds, want 10.44.10.9/12s",
			after.IperfServer, after.TestDuration)
	}
}

var _ = wifi.ScannedNetwork{}

// TestThroughputReadingsSurviveAReload is the same defect migration 00002 fixed
// for active samples: the point's kind was recorded and its payload was not, so
// every throughput measurement came back from the store empty. A real
// three-point active survey on this Mac reloaded with three points holding
// nothing — the rates, and the AP they ran over, were gone.
func TestThroughputReadingsSurviveAReload(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	meter := &scriptedMeter{sample: survey.ThroughputSample{DownloadMbps: 272.1, UploadMbps: 288.2}}
	mgr := mustManager(t, dir, &countingScanner{}, nil, meter, nil)
	s, err := mgr.CreateSurvey("reloaded", "", "en0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	if err := mgr.SetThroughputTarget(s.ID, "10.44.30.30", 4); err != nil {
		t.Fatalf("SetThroughputTarget: %v", err)
	}
	if err := mgr.StartSurvey(s.ID); err != nil {
		t.Fatalf("StartSurvey: %v", err)
	}
	if _, err := mgr.MeasureThroughput(context.Background(), s.ID, 700, 400); err != nil {
		t.Fatalf("MeasureThroughput: %v", err)
	}
	if err := mgr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened := mustManager(t, dir, nil, nil, nil, nil)
	if err := reopened.LoadSurveys(); err != nil {
		t.Fatalf("LoadSurveys: %v", err)
	}
	after, err := reopened.GetSurvey(s.ID)
	if err != nil {
		t.Fatalf("GetSurvey: %v", err)
	}

	points := after.GetAllSamples()
	if len(points) != 1 {
		t.Fatalf("%d points after reload, want 1", len(points))
	}
	stored, ok := points[0].SampleData.(*survey.ThroughputSample)
	if !ok {
		t.Fatalf("point holds %T after reload, want a throughput sample", points[0].SampleData)
	}
	if stored.DownloadMbps != 272.1 || stored.UploadMbps != 288.2 {
		t.Errorf("rates after reload = %v/%v, want 272.1/288.2",
			stored.DownloadMbps, stored.UploadMbps)
	}
	// The association is what makes the reading comparable with the passive
	// points around it, so it has to survive too.
	if stored.BSSID != "aa:bb:cc:00:00:01" {
		t.Errorf("association after reload = %q, want the AP the link ran over", stored.BSSID)
	}
}
