package api_test

// GetCoverage used to take no metric: it ran the RSSI analysis against a dBm
// threshold whichever layer the caller was looking at. These tests are about
// the wire — that the metric reaches the analysis, that neither metric's
// default threshold is applied under the other, and that a layer with no
// dead-zone rule is refused rather than answered about signal strength.

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
	surveyv1 "github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1"
	"github.com/MustardSeedNetworks/trellis/internal/api"
	"google.golang.org/protobuf/proto"
)

// noisyFloorSurvey walks a floor where the signal is strong and the noise
// floor is right behind it: -55 dBm with 8 dB of margin.
func noisyFloorSurvey(t *testing.T) (*api.SurveyServiceHandler, string) {
	t.Helper()

	manager := mustManager(t, t.TempDir(), scriptedScanner{networks: []wifi.ScannedNetwork{
		{SSID: "noisy", BSSID: "aa:bb:cc:00:00:09", Signal: -55, NoiseFloor: -63, SNR: 8, Channel: 6, Frequency: 2437},
	}}, nil, nil, nil)
	handler := api.NewSurveyServiceHandler(manager)
	ctx := context.Background()

	created, err := handler.CreateSurvey(ctx,
		connect.NewRequest(&surveyv1.CreateSurveyRequest{Name: "noisy", Interface: "en0"}))
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	id := created.Msg.GetSurvey().GetId()
	if _, err := handler.StartSurvey(ctx,
		connect.NewRequest(&surveyv1.StartSurveyRequest{Id: id})); err != nil {
		t.Fatalf("StartSurvey: %v", err)
	}
	for _, at := range []struct{ x, y int32 }{{10, 10}, {60, 10}, {110, 10}, {160, 10}} {
		if _, err := handler.CapturePoint(ctx,
			connect.NewRequest(&surveyv1.CapturePointRequest{SurveyId: id, X: at.x, Y: at.y})); err != nil {
			t.Fatalf("CapturePoint: %v", err)
		}
	}
	return handler, id
}

func TestGetCoverage_SNRAndRSSIDisagreeOnTheSameFloor(t *testing.T) {
	t.Parallel()

	handler, id := noisyFloorSurvey(t)
	ctx := context.Background()

	rssi, err := handler.GetCoverage(ctx, connect.NewRequest(&surveyv1.GetCoverageRequest{
		SurveyId: id, Metric: "rssi",
	}))
	if err != nil {
		t.Fatalf("GetCoverage(rssi): %v", err)
	}
	if rssi.Msg.GetDeadZoneCount() != 0 || rssi.Msg.GetCoverageScore() != 100 {
		t.Errorf("rssi: %d dead zone(s) at %.1f%% on a -55 dBm floor, want none at 100%%",
			rssi.Msg.GetDeadZoneCount(), rssi.Msg.GetCoverageScore())
	}
	if rssi.Msg.GetMetric() != "rssi" || rssi.Msg.GetThreshold() != -75 {
		t.Errorf("rssi echoes metric=%q threshold=%d, want rssi and -75 dBm",
			rssi.Msg.GetMetric(), rssi.Msg.GetThreshold())
	}

	snr, err := handler.GetCoverage(ctx, connect.NewRequest(&surveyv1.GetCoverageRequest{
		SurveyId: id, Metric: "snr",
	}))
	if err != nil {
		t.Fatalf("GetCoverage(snr): %v", err)
	}
	// Neither analysis borrows the other's threshold: -75 would pass every
	// sample here, and the default that arrives must be the dB one.
	if snr.Msg.GetThreshold() != 20 {
		t.Errorf("snr threshold = %d, want the 20 dB default and not the -75 dBm one",
			snr.Msg.GetThreshold())
	}
	if snr.Msg.GetMetric() != "snr" {
		t.Errorf("snr echoes metric=%q, want snr", snr.Msg.GetMetric())
	}
	if snr.Msg.GetDeadZoneCount() == 0 {
		t.Error("snr: no dead zone at 8 dB of margin against a 20 dB floor")
	}
	if snr.Msg.GetCoverageScore() != 0 {
		t.Errorf("snr coverage score = %.1f, want 0", snr.Msg.GetCoverageScore())
	}
}

func TestGetCoverage_ExplicitThresholdIsHonouredPerMetric(t *testing.T) {
	t.Parallel()

	handler, id := noisyFloorSurvey(t)
	ctx := context.Background()

	// 5 dB of required margin: 8 dB clears it, so the same floor that fails at
	// the default passes here. A borrowed dBm threshold could not do this.
	resp, err := handler.GetCoverage(ctx, connect.NewRequest(&surveyv1.GetCoverageRequest{
		SurveyId: id, Metric: "snr", Threshold: proto.Int32(5),
	}))
	if err != nil {
		t.Fatalf("GetCoverage(snr, 5 dB): %v", err)
	}
	if resp.Msg.GetThreshold() != 5 {
		t.Errorf("threshold echoed as %d, want 5", resp.Msg.GetThreshold())
	}
	if resp.Msg.GetDeadZoneCount() != 0 || resp.Msg.GetCoverageScore() != 100 {
		t.Errorf("%d dead zone(s) at %.1f%% with 8 dB against a 5 dB floor, want none at 100%%",
			resp.Msg.GetDeadZoneCount(), resp.Msg.GetCoverageScore())
	}
}

func TestGetCoverage_MetricWithoutARuleIsRefused(t *testing.T) {
	t.Parallel()

	handler, id := noisyFloorSurvey(t)

	// The heatmap renders download throughput. Answering about RSSI under that
	// heading is the defect the metric parameter closes, so this is an error,
	// not a fallback.
	_, err := handler.GetCoverage(context.Background(), connect.NewRequest(&surveyv1.GetCoverageRequest{
		SurveyId: id, Metric: "download",
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("GetCoverage(download) = %v, want InvalidArgument", connect.CodeOf(err))
	}
}
