package api_test

// A walk on Linux or Windows records no noise floor (#600), so its SNR layer
// has nothing to draw. That is a fact about the survey, and the wire has to
// say so in a way a client can tell apart from a failure without reading the
// message: FailedPrecondition, where a broken request stays InvalidArgument
// (trellis#607).

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
	surveyv1 "github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1"
	"github.com/MustardSeedNetworks/trellis/internal/api"
)

// floorlessSurvey walks a floor on a radio that reports signal and no noise
// figure, as every nl80211 scan and Native Wifi BSS does.
func floorlessSurvey(t *testing.T) (*api.SurveyServiceHandler, string) {
	t.Helper()

	manager := mustManager(t, t.TempDir(), scriptedScanner{networks: []wifi.ScannedNetwork{
		{SSID: "quiet", BSSID: "aa:bb:cc:00:00:0a", Signal: -58, Channel: 6, Frequency: 2437},
	}}, nil, nil, nil)
	handler := api.NewSurveyServiceHandler(manager)
	ctx := context.Background()

	created, err := handler.CreateSurvey(ctx,
		connect.NewRequest(&surveyv1.CreateSurveyRequest{Name: "floorless", Interface: "wlan0"}))
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	id := created.Msg.GetSurvey().GetId()
	if _, err := handler.StartSurvey(ctx,
		connect.NewRequest(&surveyv1.StartSurveyRequest{Id: id})); err != nil {
		t.Fatalf("StartSurvey: %v", err)
	}
	for _, at := range []struct{ x, y int32 }{{10, 10}, {60, 40}, {110, 70}} {
		if _, err := handler.CapturePoint(ctx,
			connect.NewRequest(&surveyv1.CapturePointRequest{SurveyId: id, X: at.x, Y: at.y})); err != nil {
			t.Fatalf("CapturePoint: %v", err)
		}
	}
	return handler, id
}

func TestUnmeasuredSNRIsAFactAboutTheSurvey(t *testing.T) {
	t.Parallel()

	handler, id := floorlessSurvey(t)
	ctx := context.Background()

	_, err := handler.GetCoverage(ctx, connect.NewRequest(&surveyv1.GetCoverageRequest{
		SurveyId: id, Metric: "snr",
	}))
	if got := connect.CodeOf(err); got != connect.CodeFailedPrecondition {
		t.Errorf("GetCoverage(snr) = %v (%v), want FailedPrecondition", got, err)
	}

	_, err = handler.GetHeatmap(ctx, connect.NewRequest(&surveyv1.GetHeatmapRequest{
		SurveyId: id, Metric: "snr",
	}))
	if got := connect.CodeOf(err); got != connect.CodeFailedPrecondition {
		t.Errorf("GetHeatmap(snr) = %v (%v), want FailedPrecondition", got, err)
	}

	// The layer that WAS measured is untouched: the precondition is about SNR,
	// not about the survey as a whole.
	if _, err := handler.GetCoverage(ctx, connect.NewRequest(&surveyv1.GetCoverageRequest{
		SurveyId: id, Metric: "rssi",
	})); err != nil {
		t.Errorf("GetCoverage(rssi) on the same survey: %v", err)
	}
	if _, err := handler.GetHeatmap(ctx, connect.NewRequest(&surveyv1.GetHeatmapRequest{
		SurveyId: id, Metric: "rssi",
	})); err != nil {
		t.Errorf("GetHeatmap(rssi) on the same survey: %v", err)
	}
}
