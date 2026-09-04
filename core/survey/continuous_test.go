// SPDX-License-Identifier: BUSL-1.1

package survey_test

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// countingScanner answers every scan with a distinguishable airspace, so a test
// can tell which sweep a stored point came from rather than only counting them.
type countingScanner struct {
	scans atomic.Int32
}

func (s *countingScanner) Scan(ctx context.Context) ([]wifi.ScannedNetwork, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	n := s.scans.Add(1)
	return []wifi.ScannedNetwork{
		{SSID: "lab", BSSID: "aa:bb:cc:00:00:01", Signal: -40 - int(n), Channel: 36, Frequency: 5180},
	}, nil
}

// waitForSamples blocks until the survey holds at least want points, so a test
// asserts on the loop's output rather than on a sleep long enough to hope for it.
func waitForSamples(t *testing.T, mgr *survey.Manager, id string, want int) []*survey.SamplePoint {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		s, err := mgr.GetSurvey(id)
		if err != nil {
			t.Fatalf("GetSurvey: %v", err)
		}
		if points := s.GetAllSamples(); len(points) >= want {
			return points
		}
		if time.Now().After(deadline) {
			s, _ := mgr.GetSurvey(id)
			t.Fatalf("only %d samples after 5s, want %d", len(s.GetAllSamples()), want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// walkingSurvey is startedSurvey with the loop's cadence shortened. The
// shipping gap leaves the radio reachable between sweeps; a test asserting on
// the loop's output should not have to sleep out a real one.
func walkingSurvey(t *testing.T, scanner survey.Scanner) (*survey.Manager, string) {
	t.Helper()
	mgr, id := startedSurvey(t, scanner)
	mgr.SetCaptureGap(10 * time.Millisecond)
	return mgr, id
}

func TestContinuousCaptureRecordsRepeatedlyAtOnePosition(t *testing.T) {
	t.Parallel()

	mgr, id := walkingSurvey(t, &countingScanner{})
	if err := mgr.StartContinuousCapture(id, 120, 240); err != nil {
		t.Fatalf("StartContinuousCapture: %v", err)
	}
	t.Cleanup(func() { mgr.StopContinuousCapture(id) })

	points := waitForSamples(t, mgr, id, 3)
	for _, p := range points {
		if p.X != 120 || p.Y != 240 {
			t.Fatalf("point at (%d,%d), want the capture position (120,240)", p.X, p.Y)
		}
	}
	// Each point is its own sweep. A loop that stored one scan repeatedly would
	// produce the same count and a heatmap with nothing in it.
	first := points[0].SampleData.(*survey.PassiveSample).Networks[0].Signal
	second := points[1].SampleData.(*survey.PassiveSample).Networks[0].Signal
	if first == second {
		t.Errorf("consecutive points both read %d dBm; the loop is not rescanning", first)
	}
	// Timestamps must advance too — the interpolation T-B3 adds reads them.
	if !points[1].Timestamp.After(points[0].Timestamp) {
		t.Errorf("timestamps %s then %s do not advance", points[0].Timestamp, points[1].Timestamp)
	}
}

func TestContinuousCaptureFollowsTheOperator(t *testing.T) {
	t.Parallel()

	mgr, id := walkingSurvey(t, &countingScanner{})
	if err := mgr.StartContinuousCapture(id, 10, 10); err != nil {
		t.Fatalf("StartContinuousCapture: %v", err)
	}
	t.Cleanup(func() { mgr.StopContinuousCapture(id) })
	waitForSamples(t, mgr, id, 1)

	// Starting again is how the operator says "I am here now". A second loop
	// would double the radio's load and interleave two positions.
	if err := mgr.StartContinuousCapture(id, 700, 400); err != nil {
		t.Fatalf("relocate: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		points := waitForSamples(t, mgr, id, 2)
		last := points[len(points)-1]
		if last.X == 700 && last.Y == 400 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("last point still at (%d,%d) after moving to (700,400)", last.X, last.Y)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestContinuousCaptureStopsWhenTheSurveyDoes(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		end  func(*survey.Manager, string) error
	}{
		{"paused", (*survey.Manager).PauseSurvey},
		{"completed", (*survey.Manager).CompleteSurvey},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mgr, id := walkingSurvey(t, &countingScanner{})
			if err := mgr.StartContinuousCapture(id, 50, 50); err != nil {
				t.Fatalf("StartContinuousCapture: %v", err)
			}
			waitForSamples(t, mgr, id, 1)

			if err := tc.end(mgr, id); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}

			// A loop that outlived the survey would keep the radio busy for a
			// walk nobody is on, and AddSample would reject every sample it
			// took — quietly, in a goroutine nothing reads.
			s, err := mgr.GetSurvey(id)
			if err != nil {
				t.Fatalf("GetSurvey: %v", err)
			}
			settled := len(s.GetAllSamples())
			time.Sleep(200 * time.Millisecond)
			s, _ = mgr.GetSurvey(id)
			if got := len(s.GetAllSamples()); got != settled {
				t.Errorf("samples grew from %d to %d after %s", settled, got, tc.name)
			}
			if mgr.CapturingAt(id) != nil {
				t.Errorf("capture still reported as running after %s", tc.name)
			}
		})
	}
}

func TestContinuousCaptureRefusedWhenTheSurveyIsNotWalking(t *testing.T) {
	t.Parallel()

	mgr := mustManager(t, t.TempDir(), &countingScanner{}, nil, nil, nil)
	s, err := mgr.CreateSurvey("idle", "", "en0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}

	// Created but never started. Sampling into it would produce points on a
	// survey the operator has not begun.
	if err := mgr.StartContinuousCapture(s.ID, 1, 1); err == nil {
		t.Fatal("StartContinuousCapture on an unstarted survey: want an error, got nil")
	}
	if err := mgr.StartContinuousCapture("no-such-survey", 1, 1); !errors.Is(err, survey.ErrSurveyNotFound) {
		t.Fatalf("unknown survey = %v, want ErrSurveyNotFound", err)
	}
}

func TestContinuousCaptureNeedsARadio(t *testing.T) {
	t.Parallel()

	mgr, id := startedSurvey(t, nil)
	if err := mgr.StartContinuousCapture(id, 1, 1); !errors.Is(err, survey.ErrNoScanner) {
		t.Fatalf("without a scanner = %v, want ErrNoScanner", err)
	}
}

func TestCloseStopsEveryCapture(t *testing.T) {
	t.Parallel()

	mgr, id := walkingSurvey(t, &countingScanner{})
	if err := mgr.StartContinuousCapture(id, 5, 5); err != nil {
		t.Fatalf("StartContinuousCapture: %v", err)
	}
	waitForSamples(t, mgr, id, 1)

	// Close is what daemon shutdown calls. A loop still running past it holds
	// the radio and writes into a closed database.
	if err := mgr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if mgr.CapturingAt(id) != nil {
		t.Error("capture still running after Close")
	}
}

// TestContinuousCaptureCanBeRestartedAfterItStopsItself pins the registration a
// self-exit has to undo. A loop that ended because the survey stopped accepting
// samples used to stay in the manager's map: CapturingAt then reported a
// position for a dead goroutine, and the next start moved that corpse instead of
// beginning a walk.
func TestContinuousCaptureCanBeRestartedAfterItStopsItself(t *testing.T) {
	t.Parallel()

	mgr, id := walkingSurvey(t, &countingScanner{})
	if err := mgr.StartContinuousCapture(id, 10, 10); err != nil {
		t.Fatalf("StartContinuousCapture: %v", err)
	}
	waitForSamples(t, mgr, id, 1)

	if err := mgr.PauseSurvey(id); err != nil {
		t.Fatalf("PauseSurvey: %v", err)
	}
	if err := mgr.StartSurvey(id); err != nil {
		t.Fatalf("StartSurvey: %v", err)
	}

	s, _ := mgr.GetSurvey(id)
	before := len(s.GetAllSamples())
	if err := mgr.StartContinuousCapture(id, 20, 20); err != nil {
		t.Fatalf("restart: %v", err)
	}
	t.Cleanup(func() { mgr.StopContinuousCapture(id) })
	waitForSamples(t, mgr, id, before+1)
}

// TestContinuousCaptureStopsOnAnUnrecoverableRadio covers the failure that does
// not heal: a laptop outside its entitled bundle, or a host with no adapter.
// Warning about it once a second forever is not a walk, and a walk that stops
// with no reason on screen is worse than one that says why.
func TestContinuousCaptureStopsOnAnUnrecoverableRadio(t *testing.T) {
	t.Parallel()

	scanner := &failingScanner{err: errors.New("capture: no Wi-Fi interface")}
	mgr, id := walkingSurvey(t, scanner)
	if err := mgr.StartContinuousCapture(id, 1, 1); err != nil {
		t.Fatalf("StartContinuousCapture: %v", err)
	}
	t.Cleanup(func() { mgr.StopContinuousCapture(id) })

	deadline := time.Now().Add(5 * time.Second)
	for {
		status := mgr.CapturingAt(id)
		if status != nil && !status.Running {
			if status.LastError == "" {
				t.Fatal("capture stopped with no reason recorded")
			}
			if !strings.Contains(status.LastError, "no Wi-Fi interface") {
				t.Errorf("LastError = %q, want the radio's own message", status.LastError)
			}
			// Bounded, not infinite: the point of stopping is to stop.
			if got := scanner.calls.Load(); got > 5 {
				t.Errorf("radio was asked %d times before giving up", got)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("capture still running after 5s and %d failed sweeps", scanner.calls.Load())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestContinuousCaptureSurvivesOneBadSweep is the other half: a sweep that fails
// once is an adapter that was busy, and ending a building walk on it would lose
// everything after the first hiccup.
func TestContinuousCaptureSurvivesOneBadSweep(t *testing.T) {
	t.Parallel()

	scanner := &failingScanner{err: errors.New("scan busy"), failFirst: 2}
	mgr, id := walkingSurvey(t, scanner)
	if err := mgr.StartContinuousCapture(id, 1, 1); err != nil {
		t.Fatalf("StartContinuousCapture: %v", err)
	}
	t.Cleanup(func() { mgr.StopContinuousCapture(id) })

	waitForSamples(t, mgr, id, 2)
	if status := mgr.CapturingAt(id); status == nil || !status.Running {
		t.Fatalf("capture = %+v, want it still running after a transient failure", status)
	}
}

// failingScanner fails its first failFirst sweeps, or every sweep when
// failFirst is zero.
type failingScanner struct {
	err       error
	failFirst int32
	calls     atomic.Int32
}

func (s *failingScanner) Scan(ctx context.Context) ([]wifi.ScannedNetwork, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	n := s.calls.Add(1)
	if s.failFirst == 0 || n <= s.failFirst {
		return nil, s.err
	}
	return []wifi.ScannedNetwork{
		{SSID: "lab", BSSID: "aa:bb:cc:00:00:01", Signal: -40 - int(n), Channel: 36, Frequency: 5180},
	}, nil
}

// TestContinuousCaptureStoresOnlyFreshSweeps is the defect a real two-minute
// walk on a Mac exposed: 94 stored points holding two distinct readings, 52 of
// them consecutively identical.
//
// CoreWLAN sweeps the air about every fourteen seconds and answers anything
// asked in between from its cache, in a tenth of a second. A loop that stores
// whatever it is handed therefore manufactures confidence — a heatmap
// interpolated over ninety points that were nine measurements.
func TestContinuousCaptureStoresOnlyFreshSweeps(t *testing.T) {
	t.Parallel()

	scanner := &cachingScanner{}
	mgr, id := walkingSurvey(t, scanner)
	if err := mgr.StartContinuousCapture(id, 30, 30); err != nil {
		t.Fatalf("StartContinuousCapture: %v", err)
	}
	t.Cleanup(func() { mgr.StopContinuousCapture(id) })

	points := waitForSamples(t, mgr, id, 3)
	seen := make(map[int]int, len(points))
	for _, p := range points {
		seen[p.SampleData.(*PassiveSampleAlias).Networks[0].Signal]++
	}
	for signal, count := range seen {
		if count > 1 {
			t.Errorf("%d dBm stored %d times; the cache was recorded as measurements",
				signal, count)
		}
	}
	// The loop must still poll faster than the radio refreshes, or it would
	// miss sweeps rather than merely repeat them.
	if polls := int(scanner.polls.Load()); polls <= len(points) {
		t.Errorf("%d polls produced %d points; the loop is not polling between sweeps",
			polls, len(points))
	}
}

// PassiveSampleAlias keeps the assertion above readable.
type PassiveSampleAlias = survey.PassiveSample

// cachingScanner answers from a cache the way CoreWLAN does: the same airspace
// for several calls, then a genuinely new one.
type cachingScanner struct {
	polls  atomic.Int32
	sweeps atomic.Int32
}

// callsPerSweep stands in for CoreWLAN's fourteen seconds at the loop's cadence.
const callsPerSweep = 4

func (s *cachingScanner) Scan(ctx context.Context) ([]wifi.ScannedNetwork, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.polls.Add(1)%callsPerSweep == 1 {
		s.sweeps.Add(1)
	}
	return []wifi.ScannedNetwork{
		{
			SSID: "lab", BSSID: "aa:bb:cc:00:00:01",
			Signal:  -40 - int(s.sweeps.Load()),
			Channel: 36, Frequency: 5180,
		},
	}, nil
}
