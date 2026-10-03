package api_test

// testhelper_test.go centralises survey-manager construction for this suite,
// mirroring core/survey's helper. NewManager opens a database and can fail; a
// test that ignored that would fail later on a nil handle, pointing at the
// symptom instead of the cause.

import (
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
)

func mustManager(
	tb testing.TB,
	path string,
	scanner survey.Scanner,
	conn survey.ConnectionMonitor,
	meter survey.ThroughputMeter,
	detector survey.AnomalyDetector,
) *survey.Manager {
	tb.Helper()
	m, err := survey.NewManager(path, scanner, conn, meter, detector)
	if err != nil {
		tb.Fatalf("NewManager(%q): %v", path, err)
	}
	tb.Cleanup(func() { _ = m.Close() })
	return m
}
