// SPDX-License-Identifier: BUSL-1.1

package main

// capture_test.go pins what the readiness scan tells the API, which is the
// only route by which an operator learns their host cannot measure — the log
// line it also writes is invisible on a packaged install.

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
	surveyv1 "github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1"
	"github.com/MustardSeedNetworks/trellis/internal/api"
	"github.com/MustardSeedNetworks/trellis/internal/capture"
)

// scriptedScanner answers one way, forever. The readiness probe scans once.
type scriptedScanner struct {
	networks []wifi.ScannedNetwork
	err      error
}

func (s scriptedScanner) Scan(context.Context) ([]wifi.ScannedNetwork, error) {
	return s.networks, s.err
}

func capabilityOf(t *testing.T, h *api.SurveyServiceHandler) *surveyv1.GetCaptureCapabilityResponse {
	t.Helper()
	resp, err := h.GetCaptureCapability(context.Background(),
		connect.NewRequest(&surveyv1.GetCaptureCapabilityRequest{}))
	if err != nil {
		t.Fatalf("GetCaptureCapability: %v", err)
	}
	return resp.Msg
}

func handlerForTest(t *testing.T, scanner survey.Scanner) *api.SurveyServiceHandler {
	t.Helper()
	mgr, err := survey.NewManager(t.TempDir(), scanner, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })
	return api.NewSurveyServiceHandler(mgr)
}

func TestReadinessReportsAWorkingRadio(t *testing.T) {
	scanner := scriptedScanner{networks: []wifi.ScannedNetwork{{SSID: "ap", BSSID: "00:00:00:00:00:01"}}}
	h := handlerForTest(t, scanner)

	reportCaptureReadiness(context.Background(), scanner, h)

	if got := capabilityOf(t, h); !got.GetAvailable() {
		t.Errorf("a scan that returned networks reported unavailable: %+v", got)
	}
}

// The failure that is not a missing radio: on macOS an unauthorised scan
// returns the right number of networks with every name emptied, which the
// backend reports as ErrPermission. An operator can fix that, so the remedy
// has to reach them.
func TestReadinessCarriesThePermissionRemedy(t *testing.T) {
	scanner := scriptedScanner{err: capture.ErrPermission}
	h := handlerForTest(t, scanner)

	reportCaptureReadiness(context.Background(), scanner, h)

	got := capabilityOf(t, h)
	if got.GetAvailable() {
		t.Fatal("a permission failure reported an available radio")
	}
	if got.GetReason() == "" {
		t.Error("no reason given")
	}
	if got.GetRemedy() != capture.PermissionRemedy {
		t.Errorf("remedy = %q, want the platform's own %q", got.GetRemedy(), capture.PermissionRemedy)
	}
}

func TestReadinessReportsAFailedScanWithoutARemedy(t *testing.T) {
	scanner := scriptedScanner{err: errors.New("radio is busy")}
	h := handlerForTest(t, scanner)

	reportCaptureReadiness(context.Background(), scanner, h)

	got := capabilityOf(t, h)
	switch {
	case got.GetAvailable():
		t.Error("a failed scan reported an available radio")
	case got.GetReason() != "radio is busy":
		t.Errorf("reason = %q, want the scanner's own error", got.GetReason())
	case got.GetRemedy() != "":
		// Inventing a step the operator cannot take is worse than silence.
		t.Errorf("remedy = %q for a failure with no known fix", got.GetRemedy())
	}
}

func TestManagerWithoutAScannerSaysSoOnItsOwn(t *testing.T) {
	// No readiness probe runs when the backend failed to build, so the store
	// itself is what answers — and it must not claim a radio it does not hold.
	if got := capabilityOf(t, handlerForTest(t, nil)); got.GetAvailable() {
		t.Error("a manager with no scanner reported an available radio")
	}
}
