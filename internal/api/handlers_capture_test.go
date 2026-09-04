// SPDX-License-Identifier: BUSL-1.1

package api_test

import (
	"context"
	"strings"
	"testing"
	"time"

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
	// Asserted against the platform's own remedy rather than a literal: the fix
	// for a denied scan is Location Services on macOS and CAP_NET_ADMIN on
	// Linux, and a test pinning one of them would either pass everywhere while
	// checking nothing or hand a Linux operator macOS directions.
	if !strings.Contains(err.Error(), capture.PermissionRemedy) {
		t.Errorf("error %q does not carry this platform's remedy (%q)", err, capture.PermissionRemedy)
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

func TestScanReturnsTheAirspaceSortedAndTranslated(t *testing.T) {
	t.Parallel()

	busy := 62
	handler := api.NewSurveyServiceHandler(mustManager(t, t.TempDir(), scriptedScanner{
		networks: []wifi.ScannedNetwork{
			{SSID: "Faint", BSSID: "aa:bb:cc:00:00:01", Signal: -85, Channel: 6, Frequency: 2437,
				Security: "WPA2", ChannelWidth: 20, NoiseFloor: -95, SNR: 10, HTMode: "HT20"},
			{SSID: "Near", BSSID: "aa:bb:cc:00:00:02", Signal: -41, Channel: 52, Frequency: 5260,
				Security: "WPA3", ChannelWidth: 80, NoiseFloor: -96, SNR: 55, HTMode: "VHT80",
				IsDFS: true, Associated: true, ChannelUtilization: &busy},
		},
	}, nil, nil, nil))

	before := time.Now().Add(-time.Second)
	got, err := handler.Scan(context.Background(), connect.NewRequest(&surveyv1.ScanRequest{}))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	networks := got.Msg.GetNetworks()
	if len(networks) != 2 {
		t.Fatalf("networks = %d, want 2", len(networks))
	}
	// Strongest first: the live view reads top-down, and the row an operator
	// looks at first should be the one their radio is doing best with.
	if networks[0].GetSsid() != "Near" {
		t.Errorf("first network = %q, want the strongest (%q)", networks[0].GetSsid(), "Near")
	}
	if !networks[0].GetAssociated() {
		t.Error("the joined BSS did not reach the wire as associated")
	}
	if got := networks[0].GetChannelUtilizationPercent(); got != int32(busy) {
		t.Errorf("channel utilization = %d%%, want %d%%", got, busy)
	}
	// Absent is not zero: an AP that sent no BSS Load element must not read as
	// an idle channel.
	if networks[1].ChannelUtilizationPercent != nil {
		t.Errorf("unreported utilization reached the wire as %d%%",
			networks[1].GetChannelUtilizationPercent())
	}
	if at := got.Msg.GetScannedAt().AsTime(); at.Before(before) {
		t.Errorf("scanned_at = %s, want a stamp from this scan", at)
	}
}

func TestScanWithoutARadioIsUnimplemented(t *testing.T) {
	t.Parallel()

	handler := api.NewSurveyServiceHandler(mustManager(t, t.TempDir(), nil, nil, nil, nil))

	_, err := handler.Scan(context.Background(), connect.NewRequest(&surveyv1.ScanRequest{}))
	if connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Fatalf("Scan with no backend = %v (%s), want unimplemented", err, connect.CodeOf(err))
	}
}

func TestContinuousCaptureRidesOnTheSurveySummary(t *testing.T) {
	t.Parallel()

	handler, id := walkedSurvey(t, scriptedScanner{networks: []wifi.ScannedNetwork{
		{SSID: "lab", BSSID: "aa:bb:cc:00:00:01", Signal: -50, Channel: 36, Frequency: 5180},
	}})

	// Never started: absent, not a zero-valued "stopped at (0,0)".
	got, err := handler.GetSurvey(context.Background(),
		connect.NewRequest(&surveyv1.GetSurveyRequest{Id: id}))
	if err != nil {
		t.Fatalf("GetSurvey: %v", err)
	}
	if got.Msg.GetSurvey().Capture != nil {
		t.Errorf("capture = %v on a survey that never had one", got.Msg.GetSurvey().GetCapture())
	}

	started, err := handler.StartContinuousCapture(context.Background(),
		connect.NewRequest(&surveyv1.StartContinuousCaptureRequest{SurveyId: id, X: 140, Y: 260}))
	if err != nil {
		t.Fatalf("StartContinuousCapture: %v", err)
	}
	t.Cleanup(func() {
		_, _ = handler.StopContinuousCapture(context.Background(),
			connect.NewRequest(&surveyv1.StopContinuousCaptureRequest{SurveyId: id}))
	})
	if capture := started.Msg.GetCapture(); !capture.GetRunning() ||
		capture.GetX() != 140 || capture.GetY() != 260 {
		t.Fatalf("capture = %v, want running at (140,260)", capture)
	}

	// A client that reloads mid-walk reads it off the summary it already polls;
	// a walk a reload cannot see is a walk the operator starts twice.
	got, err = handler.GetSurvey(context.Background(),
		connect.NewRequest(&surveyv1.GetSurveyRequest{Id: id}))
	if err != nil {
		t.Fatalf("GetSurvey: %v", err)
	}
	if capture := got.Msg.GetSurvey().GetCapture(); !capture.GetRunning() || capture.GetX() != 140 {
		t.Fatalf("summary capture = %v, want the running walk", capture)
	}

	if _, err := handler.StopContinuousCapture(context.Background(),
		connect.NewRequest(&surveyv1.StopContinuousCaptureRequest{SurveyId: id})); err != nil {
		t.Fatalf("StopContinuousCapture: %v", err)
	}
	got, _ = handler.GetSurvey(context.Background(),
		connect.NewRequest(&surveyv1.GetSurveyRequest{Id: id}))
	if capture := got.Msg.GetSurvey().GetCapture(); capture.GetRunning() {
		t.Errorf("capture still running after a stop: %v", capture)
	}
}

func TestStartContinuousCaptureRefusedOnASurveyThatIsNotWalking(t *testing.T) {
	t.Parallel()

	handler := api.NewSurveyServiceHandler(mustManager(t, t.TempDir(), scriptedScanner{}, nil, nil, nil))
	created, err := handler.CreateSurvey(context.Background(),
		connect.NewRequest(&surveyv1.CreateSurveyRequest{Name: "idle", Interface: "en0"}))
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}

	_, err = handler.StartContinuousCapture(context.Background(),
		connect.NewRequest(&surveyv1.StartContinuousCaptureRequest{
			SurveyId: created.Msg.GetSurvey().GetId(),
		}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("start on an unstarted survey = %v (%s), want failed_precondition", err, connect.CodeOf(err))
	}

	_, err = handler.StartContinuousCapture(context.Background(),
		connect.NewRequest(&surveyv1.StartContinuousCaptureRequest{SurveyId: "no-such-survey"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("start on an unknown survey = %v (%s), want not_found", err, connect.CodeOf(err))
	}
}

func TestStoppingACaptureThatIsNotRunningIsNotAnError(t *testing.T) {
	t.Parallel()

	// Pause, complete and delete all stop the capture without knowing whether
	// there was one. A stop that errored would make each of them noisy.
	handler, id := walkedSurvey(t, scriptedScanner{})
	if _, err := handler.StopContinuousCapture(context.Background(),
		connect.NewRequest(&surveyv1.StopContinuousCaptureRequest{SurveyId: id})); err != nil {
		t.Fatalf("StopContinuousCapture with nothing running: %v", err)
	}
}

func TestListSamplesTellsPlacedReadingsFromPinnedOnes(t *testing.T) {
	t.Parallel()

	handler, id := walkedSurvey(t, scriptedScanner{networks: []wifi.ScannedNetwork{
		{SSID: "lab", BSSID: "aa:bb:cc:00:00:01", Signal: -50, Channel: 36, Frequency: 5180},
	}})

	// One pinned reading, then a walk that is marked twice so the readings
	// between the marks are placed along the way.
	if _, err := handler.CapturePoint(context.Background(),
		connect.NewRequest(&surveyv1.CapturePointRequest{SurveyId: id, X: 10, Y: 10})); err != nil {
		t.Fatalf("CapturePoint: %v", err)
	}
	for _, at := range []struct{ x, y int32 }{{20, 20}, {200, 100}} {
		if _, err := handler.StartContinuousCapture(context.Background(),
			connect.NewRequest(&surveyv1.StartContinuousCaptureRequest{
				SurveyId: id, X: at.x, Y: at.y,
			})); err != nil {
			t.Fatalf("StartContinuousCapture: %v", err)
		}
	}
	if _, err := handler.StopContinuousCapture(context.Background(),
		connect.NewRequest(&surveyv1.StopContinuousCaptureRequest{SurveyId: id})); err != nil {
		t.Fatalf("StopContinuousCapture: %v", err)
	}

	got, err := handler.ListSamples(context.Background(),
		connect.NewRequest(&surveyv1.ListSamplesRequest{SurveyId: id}))
	if err != nil {
		t.Fatalf("ListSamples: %v", err)
	}

	samples := got.Msg.GetSamples()
	if len(samples) == 0 {
		t.Fatal("no samples")
	}
	// The pin-drop is a record of where the operator was. Sending it as
	// interpolated would tell a client to draw a measurement as an estimate.
	if samples[0].GetInterpolated() {
		t.Error("the pinned reading reached the wire as interpolated")
	}
	if samples[0].GetX() != 10 || samples[0].GetY() != 10 {
		t.Errorf("pinned reading at (%d,%d), want (10,10)", samples[0].GetX(), samples[0].GetY())
	}
}
