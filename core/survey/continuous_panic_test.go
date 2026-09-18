// SPDX-License-Identifier: BUSL-1.1

package survey_test

// continuous_panic_test.go pins D-TRL-6's first half: the capture loop is the
// one long-lived goroutine in the product that runs for as long as an operator
// is walking a building, and until foundation's supervisor was put under it a
// panic anywhere beneath Scan took the whole daemon down mid-walk — every
// sample since the last write lost, and systemd's Restart=on-failure bringing
// back a daemon that has forgotten the survey was in progress.

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// panickingScanner panics on its first panics sweeps and then answers like
// countingScanner. A driver that faults on one call and not the next is the
// shape of the failure this guards: recoverable, and invisible to a loop that
// cannot outlive it.
type panickingScanner struct {
	remaining atomic.Int32
	good      countingScanner
}

func (s *panickingScanner) Scan(ctx context.Context) ([]wifi.ScannedNetwork, error) {
	if s.remaining.Add(-1) >= 0 {
		panic("radio driver faulted mid-sweep")
	}
	return s.good.Scan(ctx)
}

// captureLog redirects the default logger for one test and returns what was
// written. Safe because it is only used by tests that do not call t.Parallel:
// Go runs those to completion before any parallel test resumes.
func captureLog(t *testing.T) func() string {
	t.Helper()

	var mu sync.Mutex
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&lockedWriter{mu: &mu, buf: &buf}, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	return func() string {
		mu.Lock()
		defer mu.Unlock()
		return buf.String()
	}
}

type lockedWriter struct {
	mu  *sync.Mutex
	buf *bytes.Buffer
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func TestCaptureLoopSurvivesAPanickingRadio(t *testing.T) {
	logged := captureLog(t)

	scanner := &panickingScanner{}
	scanner.remaining.Store(1)

	mgr, id := walkingSurvey(t, scanner)
	if err := mgr.StartContinuousCapture(id, 120, 240); err != nil {
		t.Fatalf("StartContinuousCapture: %v", err)
	}
	t.Cleanup(func() { mgr.StopContinuousCapture(id) })

	// The walk stored points after the panic, which is the restart: the loop
	// that panicked is not the loop that took these.
	waitForSamples(t, mgr, id, 2)

	status := mgr.CapturingAt(id)
	if status == nil || !status.Running {
		t.Errorf("CapturingAt = %+v, want a running capture after a recovered panic", status)
	}

	out := logged()
	if !strings.Contains(out, "panicked") {
		t.Errorf("the panic was not logged\nlog:\n%s", out)
	}
	// One line for the operator, not a runtime dump: the supervisor keeps the
	// stack on the error value and logs it below the default level.
	for _, marker := range []string{"goroutine ", "runtime/"} {
		if strings.Contains(out, marker) {
			t.Errorf("a stack trace reached the operator's log (%q)\nlog:\n%s", marker, out)
		}
	}
}

func TestCaptureLoopThatKeepsPanickingStopsAndSaysWhy(t *testing.T) {
	logged := captureLog(t)

	// More panics than the loop is allowed restarts, so the supervisor gives up.
	scanner := &panickingScanner{}
	scanner.remaining.Store(1000)

	mgr, id := walkingSurvey(t, scanner)
	if err := mgr.StartContinuousCapture(id, 120, 240); err != nil {
		t.Fatalf("StartContinuousCapture: %v", err)
	}
	t.Cleanup(func() { mgr.StopContinuousCapture(id) })

	// A walk whose pins stop appearing with nothing on screen saying why is the
	// worst of the outcomes this package already names; a capture the supervisor
	// gave up on must read the same as one the radio killed.
	deadline := time.Now().Add(5 * time.Second)
	var status *survey.CaptureStatus
	for {
		status = mgr.CapturingAt(id)
		if status != nil && !status.Running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("capture still reports Running 5s after the radio began panicking on every sweep\nlog:\n%s", logged())
		}
		time.Sleep(5 * time.Millisecond)
	}

	if status.LastError == "" {
		t.Error("the capture stopped without a reason a client could show")
	}
	if strings.Contains(status.LastError, "goroutine ") {
		t.Errorf("LastError carries a stack trace: %q", status.LastError)
	}
}
