package survey_test

// testhelper_test.go centralises manager construction for the suite.
//
// NewManager returns an error now that it opens a database, and a test that
// ignored it would fail later and elsewhere — on a nil map or a nil *sql.DB —
// with a message pointing at the symptom rather than the cause.

import (
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
)

// mustManager builds a Manager rooted at a caller-supplied path, failing the
// test immediately if the store cannot be opened. The manager is closed when
// the test ends so the SQLite file handle does not outlive it.
func mustManager(
	t *testing.T,
	path string,
	scanner survey.Scanner,
	conn survey.ConnectionMonitor,
	meter survey.ThroughputMeter,
	detector survey.AnomalyDetector,
) *survey.Manager {
	t.Helper()
	m, err := survey.NewManager(path, scanner, conn, meter, detector)
	if err != nil {
		t.Fatalf("NewManager(%q): %v", path, err)
	}
	t.Cleanup(func() { _ = m.Close() })
	return m
}
