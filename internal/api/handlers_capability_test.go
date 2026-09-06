// SPDX-License-Identifier: BUSL-1.1

package api_test

// handlers_capability_test.go pins what a host with no radio tells a client.
//
// The daemon has always logged "no Wi-Fi capture backend on this host" and then
// served a UI that offered to walk a floor anyway; the first thing an operator
// learned was a failed capture. This RPC is what lets the UI say it first, and
// the point of these tests is that "cannot measure" is never confused with
// "cannot be used": imports, analysis and reports are unaffected.

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/MustardSeedNetworks/trellis/internal/api"

	surveyv1 "github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1"
)

func TestCaptureCapabilityWithoutARadio(t *testing.T) {
	handler := api.NewSurveyServiceHandler(mustManager(t, t.TempDir(), nil, nil, nil, nil))

	resp, err := handler.GetCaptureCapability(context.Background(),
		connect.NewRequest(&surveyv1.GetCaptureCapabilityRequest{}))
	if err != nil {
		t.Fatalf("GetCaptureCapability: %v", err)
	}
	if resp.Msg.GetAvailable() {
		t.Error("a manager with no scanner reported a radio")
	}
	if resp.Msg.GetReason() == "" {
		t.Error("unavailable with no reason — the client has nothing to show the operator")
	}
}

func TestCaptureCapabilityReportsWhatTheDaemonLearned(t *testing.T) {
	handler := api.NewSurveyServiceHandler(mustManager(t, t.TempDir(), nil, nil, nil, nil))
	// What the readiness scan reports on a Mac whose Location Services
	// authorization is missing: a radio that scans and cannot name what it
	// hears, which is a different sentence from having no radio at all.
	handler.SetCaptureCapability(api.CaptureCapability{
		Reason: "capture: permission denied",
		Remedy: "enable Trellis in System Settings",
	})

	resp, err := handler.GetCaptureCapability(context.Background(),
		connect.NewRequest(&surveyv1.GetCaptureCapabilityRequest{}))
	if err != nil {
		t.Fatalf("GetCaptureCapability: %v", err)
	}
	if resp.Msg.GetReason() != "capture: permission denied" ||
		resp.Msg.GetRemedy() != "enable Trellis in System Settings" {
		t.Errorf("reported %q / %q", resp.Msg.GetReason(), resp.Msg.GetRemedy())
	}
}
