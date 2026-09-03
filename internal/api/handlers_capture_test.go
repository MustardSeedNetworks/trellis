// SPDX-License-Identifier: BUSL-1.1

package api_test

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"github.com/MustardSeedNetworks/trellis/core/wifi"
	surveyv1 "github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1"
	"github.com/MustardSeedNetworks/trellis/internal/api"
	"github.com/MustardSeedNetworks/trellis/internal/capture"
)

// scriptedScanner stands in for a radio so the handler's translation can be
// asserted on known values.
type scriptedScanner struct {
	networks []wifi.ScannedNetwork
	err      error
}

func (s scriptedScanner) Scan(context.Context) ([]wifi.ScannedNetwork, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.networks, nil
}

// walkedSurvey returns a handler with a survey already in progress, which is
// the only state CapturePoint accepts.
func walkedSurvey(t *testing.T, scanner scriptedScanner) (*api.SurveyServiceHandler, string) {
	t.Helper()
	handler := api.NewSurveyServiceHandler(mustManager(t, t.TempDir(), scanner, nil, nil, nil))

	created, err := handler.CreateSurvey(context.Background(),
		connect.NewRequest(&surveyv1.CreateSurveyRequest{Name: "live", Interface: "en0"}))
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	id := created.Msg.GetSurvey().GetId()

	if _, err := handler.StartSurvey(context.Background(),
		connect.NewRequest(&surveyv1.StartSurveyRequest{Id: id})); err != nil {
		t.Fatalf("StartSurvey: %v", err)
	}
	return handler, id
}

func TestCapturePointReturnsTheScan(t *testing.T) {
	t.Parallel()

	handler, id := walkedSurvey(t, scriptedScanner{networks: []wifi.ScannedNetwork{
		{SSID: "Faint", BSSID: "aa:bb:cc:00:00:01", Signal: -85, Channel: 6, Frequency: 2437,
			Security: "WPA2", ChannelWidth: 20, NoiseFloor: -95, SNR: 10, HTMode: "HT20"},
		{SSID: "Near", BSSID: "aa:bb:cc:00:00:02", Signal: -41, Channel: 52, Frequency: 5260,
			Security: "WPA3", ChannelWidth: 80, NoiseFloor: -96, SNR: 55, HTMode: "VHT80", IsDFS: true},
	}})

	resp, err := handler.CapturePoint(context.Background(),
		connect.NewRequest(&surveyv1.CapturePointRequest{SurveyId: id, X: 100, Y: 200}))
	if err != nil {
		t.Fatalf("CapturePoint: %v", err)
	}

	got := resp.Msg.GetNetworks()
	if len(got) != 2 {
		t.Fatalf("networks = %d, want 2", len(got))
	}

	// Strongest first, and every field carried across — a count assertion here
	// would pass against a handler that dropped security, width or the DFS flag.
	strongest := got[0]
	if strongest.GetSsid() != "Near" {
		t.Errorf("networks[0].ssid = %q, want %q", strongest.GetSsid(), "Near")
	}
	if strongest.GetBssid() != "aa:bb:cc:00:00:02" {
		t.Errorf("networks[0].bssid = %q, want the near AP's", strongest.GetBssid())
	}
	if strongest.GetSignalDbm() != -41 {
		t.Errorf("networks[0].signal_dbm = %d, want -41", strongest.GetSignalDbm())
	}
	if strongest.GetChannel() != 52 || strongest.GetFrequencyMhz() != 5260 {
		t.Errorf("networks[0] channel/frequency = %d/%d, want 52/5260",
			strongest.GetChannel(), strongest.GetFrequencyMhz())
	}
	if strongest.GetSecurity() != "WPA3" || strongest.GetHtMode() != "VHT80" {
		t.Errorf("networks[0] security/htMode = %q/%q, want WPA3/VHT80",
			strongest.GetSecurity(), strongest.GetHtMode())
	}
	if strongest.GetChannelWidthMhz() != 80 || strongest.GetSnrDb() != 55 {
		t.Errorf("networks[0] width/snr = %d/%d, want 80/55",
			strongest.GetChannelWidthMhz(), strongest.GetSnrDb())
	}
	if !strongest.GetIsDfs() {
		t.Error("networks[0].is_dfs = false, want true for channel 52")
	}
	if strongest.GetNoiseFloorDbm() != -96 {
		t.Errorf("networks[0].noise_floor_dbm = %d, want -96", strongest.GetNoiseFloorDbm())
	}

	if resp.Msg.GetUniqueBssids() != 2 {
		t.Errorf("unique_bssids = %d, want 2", resp.Msg.GetUniqueBssids())
	}
	if resp.Msg.GetApCount_2_4Ghz() != 1 || resp.Msg.GetApCount_5Ghz() != 1 {
		t.Errorf("band counts = 2.4:%d 5:%d, want 1 and 1",
			resp.Msg.GetApCount_2_4Ghz(), resp.Msg.GetApCount_5Ghz())
	}
}

// A missing Location Services grant is the one capture failure an operator
// fixes by doing something, so it must arrive as a precondition with the
// remedy attached rather than as an opaque internal error.
func TestCapturePointPermissionDeniedCarriesTheRemedy(t *testing.T) {
	t.Parallel()

	handler, id := walkedSurvey(t, scriptedScanner{err: capture.ErrPermission})

	_, err := handler.CapturePoint(context.Background(),
		connect.NewRequest(&surveyv1.CapturePointRequest{SurveyId: id, X: 1, Y: 1}))
	if err == nil {
		t.Fatal("CapturePoint with a denied scan: want error, got nil")
	}
	if got := connect.CodeOf(err); got != connect.CodeFailedPrecondition {
		t.Errorf("code = %v, want %v", got, connect.CodeFailedPrecondition)
	}
	if !strings.Contains(err.Error(), "Location Services") {
		t.Errorf("error %q does not tell the operator what to enable", err)
	}
	if !strings.Contains(err.Error(), "Trellis.app") {
		t.Errorf("error %q does not mention the launch path, the other half of the fix", err)
	}
}

func TestCapturePointWithoutABackend(t *testing.T) {
	t.Parallel()

	handler := api.NewSurveyServiceHandler(mustManager(t, t.TempDir(), nil, nil, nil, nil))
	created, err := handler.CreateSurvey(context.Background(),
		connect.NewRequest(&surveyv1.CreateSurveyRequest{Name: "no radio"}))
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	id := created.Msg.GetSurvey().GetId()
	if _, err := handler.StartSurvey(context.Background(),
		connect.NewRequest(&surveyv1.StartSurveyRequest{Id: id})); err != nil {
		t.Fatalf("StartSurvey: %v", err)
	}

	_, err = handler.CapturePoint(context.Background(),
		connect.NewRequest(&surveyv1.CapturePointRequest{SurveyId: id}))
	if got := connect.CodeOf(err); got != connect.CodeUnimplemented {
		t.Errorf("code = %v, want %v (a host with no radio, not a broken request)", got, connect.CodeUnimplemented)
	}
}

func TestCapturePointBeforeStartIsRefused(t *testing.T) {
	t.Parallel()

	handler := api.NewSurveyServiceHandler(
		mustManager(t, t.TempDir(), scriptedScanner{}, nil, nil, nil))
	created, err := handler.CreateSurvey(context.Background(),
		connect.NewRequest(&surveyv1.CreateSurveyRequest{Name: "not started"}))
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}

	_, err = handler.CapturePoint(context.Background(),
		connect.NewRequest(&surveyv1.CapturePointRequest{
			SurveyId: created.Msg.GetSurvey().GetId(),
		}))
	if got := connect.CodeOf(err); got != connect.CodeFailedPrecondition {
		t.Errorf("code = %v, want %v", got, connect.CodeFailedPrecondition)
	}
}

func TestCapturePointUnknownSurvey(t *testing.T) {
	t.Parallel()

	handler := api.NewSurveyServiceHandler(
		mustManager(t, t.TempDir(), scriptedScanner{}, nil, nil, nil))

	_, err := handler.CapturePoint(context.Background(),
		connect.NewRequest(&surveyv1.CapturePointRequest{SurveyId: "does-not-exist"}))
	if got := connect.CodeOf(err); got != connect.CodeNotFound {
		t.Errorf("code = %v, want %v", got, connect.CodeNotFound)
	}
}

func TestSurveyLifecycleReportsStatus(t *testing.T) {
	t.Parallel()

	handler, id := walkedSurvey(t, scriptedScanner{})

	paused, err := handler.PauseSurvey(context.Background(),
		connect.NewRequest(&surveyv1.PauseSurveyRequest{Id: id}))
	if err != nil {
		t.Fatalf("PauseSurvey: %v", err)
	}
	if got := paused.Msg.GetSurvey().GetStatus(); got != "paused" {
		t.Errorf("status after pause = %q, want %q", got, "paused")
	}

	// A paused survey must refuse samples; that is the point of pausing.
	if _, err := handler.CapturePoint(context.Background(),
		connect.NewRequest(&surveyv1.CapturePointRequest{SurveyId: id})); err == nil {
		t.Error("CapturePoint on a paused survey: want error, got nil")
	}

	if _, err := handler.StartSurvey(context.Background(),
		connect.NewRequest(&surveyv1.StartSurveyRequest{Id: id})); err != nil {
		t.Fatalf("StartSurvey after pause: %v", err)
	}
	done, err := handler.CompleteSurvey(context.Background(),
		connect.NewRequest(&surveyv1.CompleteSurveyRequest{Id: id}))
	if err != nil {
		t.Fatalf("CompleteSurvey: %v", err)
	}
	if got := done.Msg.GetSurvey().GetStatus(); got != "completed" {
		t.Errorf("status after complete = %q, want %q", got, "completed")
	}
}

func TestCreateSurveyRequiresAName(t *testing.T) {
	t.Parallel()

	handler := api.NewSurveyServiceHandler(
		mustManager(t, t.TempDir(), scriptedScanner{}, nil, nil, nil))

	_, err := handler.CreateSurvey(context.Background(),
		connect.NewRequest(&surveyv1.CreateSurveyRequest{Name: "   "}))
	if got := connect.CodeOf(err); got != connect.CodeInvalidArgument {
		t.Errorf("code = %v, want %v", got, connect.CodeInvalidArgument)
	}
}

// A walk has to be able to see where it has been. SurveySummary carries a
// count; ListSamples is the read path for the points themselves, and the
// assertions are on position and signal, not on how many came back.
func TestListSamplesReturnsStoredPointsInCaptureOrder(t *testing.T) {
	t.Parallel()

	handler, id := walkedSurvey(t, scriptedScanner{networks: []wifi.ScannedNetwork{
		{SSID: "Faint", BSSID: "aa:bb:cc:00:00:01", Signal: -85, Channel: 6, Frequency: 2437},
		{SSID: "Near", BSSID: "aa:bb:cc:00:00:02", Signal: -41, Channel: 52, Frequency: 5260},
	}})

	for _, p := range [][2]int32{{100, 200}, {300, 50}} {
		if _, err := handler.CapturePoint(context.Background(),
			connect.NewRequest(&surveyv1.CapturePointRequest{SurveyId: id, X: p[0], Y: p[1]})); err != nil {
			t.Fatalf("CapturePoint(%v): %v", p, err)
		}
	}

	resp, err := handler.ListSamples(context.Background(),
		connect.NewRequest(&surveyv1.ListSamplesRequest{SurveyId: id}))
	if err != nil {
		t.Fatalf("ListSamples: %v", err)
	}

	got := resp.Msg.GetSamples()
	if len(got) != 2 {
		t.Fatalf("samples = %d, want 2", len(got))
	}
	if got[0].GetX() != 100 || got[0].GetY() != 200 || got[1].GetX() != 300 || got[1].GetY() != 50 {
		t.Errorf("positions = (%d,%d),(%d,%d), want (100,200),(300,50) in capture order",
			got[0].GetX(), got[0].GetY(), got[1].GetX(), got[1].GetY())
	}
	for i, s := range got {
		if s.GetNetworkCount() != 2 {
			t.Errorf("samples[%d].network_count = %d, want 2", i, s.GetNetworkCount())
		}
		if s.StrongestDbm == nil || s.GetStrongestDbm() != -41 {
			t.Errorf("samples[%d].strongest_dbm = %d (set=%v), want -41", i, s.GetStrongestDbm(), s.StrongestDbm != nil)
		}
		if s.GetCapturedAt() == nil || s.GetCapturedAt().AsTime().IsZero() {
			t.Errorf("samples[%d].captured_at is unset", i)
		}
	}
}

func TestListSamplesEmptyAirspaceHasNoStrongest(t *testing.T) {
	t.Parallel()

	handler, id := walkedSurvey(t, scriptedScanner{networks: []wifi.ScannedNetwork{}})
	if _, err := handler.CapturePoint(context.Background(),
		connect.NewRequest(&surveyv1.CapturePointRequest{SurveyId: id, X: 1, Y: 1})); err != nil {
		t.Fatalf("CapturePoint: %v", err)
	}

	resp, err := handler.ListSamples(context.Background(),
		connect.NewRequest(&surveyv1.ListSamplesRequest{SurveyId: id}))
	if err != nil {
		t.Fatalf("ListSamples: %v", err)
	}
	got := resp.Msg.GetSamples()
	if len(got) != 1 || got[0].GetNetworkCount() != 0 || got[0].StrongestDbm != nil {
		t.Fatalf("empty scan should store one point with no networks and no strongest signal; got %v", got)
	}
}

func TestListSamplesUnknownSurveyIsNotFound(t *testing.T) {
	t.Parallel()

	handler := api.NewSurveyServiceHandler(mustManager(t, t.TempDir(), nil, nil, nil, nil))
	_, err := handler.ListSamples(context.Background(),
		connect.NewRequest(&surveyv1.ListSamplesRequest{SurveyId: "nope"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("code = %v, want NotFound", connect.CodeOf(err))
	}
}
