// SPDX-License-Identifier: BUSL-1.1

package survey_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// scriptedScanner returns a fixed airspace, so a test can assert on the values
// that reach the survey rather than only on how many arrived.
type scriptedScanner struct {
	networks []wifi.ScannedNetwork
	err      error
	calls    int
}

func (s *scriptedScanner) Scan(ctx context.Context) ([]wifi.ScannedNetwork, error) {
	s.calls++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.err != nil {
		return nil, s.err
	}
	return s.networks, nil
}

// airspace is deliberately not in signal order: CapturePoint must sort it, and
// a fixture already sorted could not tell a working sort from a missing one.
func airspace() []wifi.ScannedNetwork {
	return []wifi.ScannedNetwork{
		{SSID: "Weak", BSSID: "aa:bb:cc:00:00:01", Signal: -81, Channel: 1, Frequency: 2412},
		{SSID: "Strong", BSSID: "aa:bb:cc:00:00:02", Signal: -42, Channel: 36, Frequency: 5180},
		{SSID: "Middle", BSSID: "aa:bb:cc:00:00:03", Signal: -60, Channel: 40, Frequency: 5200},
	}
}

// startedSurvey returns a manager and the ID of a survey already in progress,
// which is the only state that accepts samples.
func startedSurvey(t *testing.T, scanner survey.Scanner) (*survey.Manager, string) {
	t.Helper()
	mgr := mustManager(t, t.TempDir(), scanner, nil, nil, nil)
	s, err := mgr.CreateSurvey("live", "", "en0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	if err := mgr.StartSurvey(s.ID); err != nil {
		t.Fatalf("StartSurvey: %v", err)
	}
	return mgr, s.ID
}

func TestCapturePointRecordsWhatWasScanned(t *testing.T) {
	t.Parallel()

	scanner := &scriptedScanner{networks: airspace()}
	mgr, id := startedSurvey(t, scanner)

	sample, err := mgr.CapturePoint(context.Background(), id, 12, 34)
	if err != nil {
		t.Fatalf("CapturePoint: %v", err)
	}

	// Networks[0] is what the heatmap, coverage analysis and report all read as
	// "the AP serving this point". Asserting the count here would pass against
	// an unsorted sample and hide exactly that defect.
	if got := sample.Networks[0].SSID; got != "Strong" {
		t.Errorf("Networks[0].SSID = %q, want %q (strongest first)", got, "Strong")
	}
	if got := sample.Networks[0].Signal; got != -42 {
		t.Errorf("Networks[0].Signal = %d, want -42", got)
	}
	if got := sample.Networks[2].SSID; got != "Weak" {
		t.Errorf("Networks[2].SSID = %q, want %q", got, "Weak")
	}

	if sample.UniqueBSSIDs != 3 {
		t.Errorf("UniqueBSSIDs = %d, want 3", sample.UniqueBSSIDs)
	}
	if sample.APCount2_4 != 1 {
		t.Errorf("APCount2_4 = %d, want 1", sample.APCount2_4)
	}
	if sample.APCount5 != 2 {
		t.Errorf("APCount5 = %d, want 2", sample.APCount5)
	}
}

func TestCapturePointPersistsSampleAtPosition(t *testing.T) {
	t.Parallel()

	scanner := &scriptedScanner{networks: airspace()}
	mgr, id := startedSurvey(t, scanner)

	if _, err := mgr.CapturePoint(context.Background(), id, 12, 34); err != nil {
		t.Fatalf("CapturePoint: %v", err)
	}

	stored, err := mgr.GetSurvey(id)
	if err != nil {
		t.Fatalf("GetSurvey: %v", err)
	}
	samples := stored.GetAllSamples()
	if len(samples) != 1 {
		t.Fatalf("stored samples = %d, want 1", len(samples))
	}
	if samples[0].X != 12 || samples[0].Y != 34 {
		t.Errorf("sample position = (%d,%d), want (12,34)", samples[0].X, samples[0].Y)
	}

	// The sample must carry the scan, not merely exist at the right coordinates.
	passive, ok := samples[0].SampleData.(*survey.PassiveSample)
	if !ok {
		t.Fatalf("SampleData is %T, want *survey.PassiveSample", samples[0].SampleData)
	}
	if got := passive.Networks[0].BSSID; got != "aa:bb:cc:00:00:02" {
		t.Errorf("persisted Networks[0].BSSID = %q, want the strongest AP's", got)
	}
}

func TestCapturePointWithoutScanner(t *testing.T) {
	t.Parallel()

	mgr, id := startedSurvey(t, nil)

	_, err := mgr.CapturePoint(context.Background(), id, 1, 1)
	if !errors.Is(err, survey.ErrNoScanner) {
		t.Errorf("CapturePoint error = %v, want ErrNoScanner", err)
	}
}

func TestCapturePointSurveyNotInProgress(t *testing.T) {
	t.Parallel()

	scanner := &scriptedScanner{networks: airspace()}
	mgr := mustManager(t, t.TempDir(), scanner, nil, nil, nil)
	s, err := mgr.CreateSurvey("not started", "", "en0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}

	if _, err := mgr.CapturePoint(context.Background(), s.ID, 1, 1); err == nil {
		t.Fatal("CapturePoint on a survey that was never started: want error, got nil")
	}

	stored, err := mgr.GetSurvey(s.ID)
	if err != nil {
		t.Fatalf("GetSurvey: %v", err)
	}
	if got := len(stored.GetAllSamples()); got != 0 {
		t.Errorf("samples recorded on a rejected capture = %d, want 0", got)
	}
}

func TestCapturePointScanFailure(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("radio unavailable")
	scanner := &scriptedScanner{err: sentinel}
	mgr, id := startedSurvey(t, scanner)

	_, err := mgr.CapturePoint(context.Background(), id, 5, 5)
	if !errors.Is(err, sentinel) {
		t.Errorf("CapturePoint error = %v, want it to wrap %v", err, sentinel)
	}

	stored, _ := mgr.GetSurvey(id)
	if got := len(stored.GetAllSamples()); got != 0 {
		t.Errorf("samples recorded after a failed scan = %d, want 0", got)
	}
}

func TestCapturePointCancelledContext(t *testing.T) {
	t.Parallel()

	scanner := &scriptedScanner{networks: airspace()}
	mgr, id := startedSurvey(t, scanner)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := mgr.CapturePoint(ctx, id, 7, 7)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("CapturePoint error = %v, want context.Canceled", err)
	}

	stored, _ := mgr.GetSurvey(id)
	if got := len(stored.GetAllSamples()); got != 0 {
		t.Errorf("samples recorded after cancellation = %d, want 0", got)
	}
}

func TestScanReadsTheAirspaceWithoutRecordingIt(t *testing.T) {
	t.Parallel()

	scanner := &scriptedScanner{networks: airspace()}
	mgr, id := startedSurvey(t, scanner)

	networks, err := mgr.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(networks) != len(airspace()) {
		t.Fatalf("networks = %d, want %d", len(networks), len(airspace()))
	}
	// Same order a stored point is aggregated into. The fixture is deliberately
	// unsorted, so a missing sort cannot pass this.
	for i := 1; i < len(networks); i++ {
		if networks[i-1].Signal < networks[i].Signal {
			t.Fatalf("networks are not strongest-first: %d dBm before %d dBm",
				networks[i-1].Signal, networks[i].Signal)
		}
	}
	// A live reading belongs to no survey. If it landed on the started one,
	// walking a floor with the live view open would fill it with points nobody
	// stood at.
	s, err := mgr.GetSurvey(id)
	if err != nil {
		t.Fatalf("GetSurvey: %v", err)
	}
	if got := len(s.GetAllSamples()); got != 0 {
		t.Errorf("survey holds %d samples after a live scan, want 0", got)
	}
}

func TestScanWithoutARadio(t *testing.T) {
	t.Parallel()

	mgr := mustManager(t, t.TempDir(), nil, nil, nil, nil)
	if _, err := mgr.Scan(context.Background()); !errors.Is(err, survey.ErrNoScanner) {
		t.Fatalf("Scan without a scanner = %v, want ErrNoScanner", err)
	}
}

// TestScanSerializesAdapterAccess pins the reason the live view and a walk can
// both be open: there is one radio, and two overlapping scans on it would race
// for the driver rather than queue.
func TestScanSerializesAdapterAccess(t *testing.T) {
	t.Parallel()

	scanner := &overlapDetectingScanner{}
	mgr, id := startedSurvey(t, scanner)

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = mgr.Scan(context.Background())
		}()
		go func() {
			defer wg.Done()
			_, _ = mgr.CapturePoint(context.Background(), id, 1, 1)
		}()
	}
	wg.Wait()

	if overlaps := scanner.overlaps.Load(); overlaps != 0 {
		t.Errorf("%d scans overlapped on the adapter, want 0", overlaps)
	}
}

// overlapDetectingScanner records any scan that begins while another is still
// running.
type overlapDetectingScanner struct {
	inFlight atomic.Int32
	overlaps atomic.Int32
}

func (s *overlapDetectingScanner) Scan(context.Context) ([]wifi.ScannedNetwork, error) {
	if s.inFlight.Add(1) > 1 {
		s.overlaps.Add(1)
	}
	// A real scan blocks in the driver for seconds; this is long enough for a
	// concurrent caller to be inside the window if nothing keeps it out.
	time.Sleep(time.Millisecond)
	s.inFlight.Add(-1)
	return airspace(), nil
}
