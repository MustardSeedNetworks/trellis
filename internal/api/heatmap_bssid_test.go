package api_test

// A surveyor finds a mis-powered or misplaced AP by looking at where that one
// AP serves. These tests pin the strongest-AP view the floor has always
// rendered, then filter it to each of two APs that cover opposite ends.

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
	surveyv1 "github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1"
	"github.com/MustardSeedNetworks/trellis/internal/api"
)

const (
	westAP = "aa:bb:cc:00:00:01"
	eastAP = "aa:bb:cc:00:00:02"
)

// walkScanner answers each capture with the next scan in its script, which is
// what lets two APs fade in opposite directions along one walk.
type walkScanner struct {
	mu    sync.Mutex
	scans [][]wifi.ScannedNetwork
}

func (s *walkScanner) Scan(context.Context) ([]wifi.ScannedNetwork, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.scans[0]
	s.scans = s.scans[1:]
	return next, nil
}

// twoAPFloorSurvey walks four points west to east. The west AP is strong at
// the west end and weak at the east; the east AP is the mirror image, and is
// not heard at all at the westernmost point. Strongest-AP coverage is good
// everywhere, so only a per-AP view shows either AP's weak end.
func twoAPFloorSurvey(t *testing.T) (*api.SurveyServiceHandler, string) {
	t.Helper()

	west := func(signal int) wifi.ScannedNetwork {
		return wifi.ScannedNetwork{
			SSID: "corp", BSSID: westAP, Signal: signal, NoiseFloor: -95, SNR: signal + 95,
			Channel: 1, Frequency: 2412,
		}
	}
	east := func(signal int) wifi.ScannedNetwork {
		return wifi.ScannedNetwork{
			SSID: "corp", BSSID: eastAP, Signal: signal, NoiseFloor: -95, SNR: signal + 95,
			Channel: 11, Frequency: 2462,
		}
	}
	scanner := &walkScanner{scans: [][]wifi.ScannedNetwork{
		{west(-45)},
		{west(-50), east(-80)},
		{west(-80), east(-50)},
		{west(-85), east(-45)},
	}}

	handler := api.NewSurveyServiceHandler(mustManager(t, t.TempDir(), scanner, nil, nil, nil))
	ctx := context.Background()
	created, err := handler.CreateSurvey(ctx,
		connect.NewRequest(&surveyv1.CreateSurveyRequest{Name: "two APs", Interface: "en0"}))
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	id := created.Msg.GetSurvey().GetId()
	if _, err := handler.StartSurvey(ctx,
		connect.NewRequest(&surveyv1.StartSurveyRequest{Id: id})); err != nil {
		t.Fatalf("StartSurvey: %v", err)
	}
	for _, x := range []int32{20, 120, 220, 320} {
		if _, err := handler.CapturePoint(ctx,
			connect.NewRequest(&surveyv1.CapturePointRequest{SurveyId: id, X: x, Y: 40})); err != nil {
			t.Fatalf("CapturePoint(%d): %v", x, err)
		}
	}
	return handler, id
}

// fieldDigest fingerprints what a heatmap reply draws: the image and the
// interpolated field under it.
func fieldDigest(msg *surveyv1.GetHeatmapResponse) string {
	h := sha256.New()
	h.Write(msg.GetPng())
	for _, v := range msg.GetGrid() {
		_ = binary.Write(h, binary.LittleEndian, math.Float32bits(v))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// The strongest-AP view, pinned on main before the BSSID filter existed: a
// request that names no BSSID must keep drawing exactly this.
func TestGetHeatmap_UnfilteredFloorIsTheStrongestAPView(t *testing.T) {
	t.Parallel()

	handler, id := twoAPFloorSurvey(t)
	ctx := context.Background()

	for _, tc := range []struct {
		metric string
		digest string
	}{
		{"rssi", "6fdaddc4b9d6d4b2a772f449cf0b2239bdd1d7159a0e77be293308dca652a6af"},
		{"snr", "be533788cb1ef946429fa052f180d6da892588ad9a4de95530363e81b96b4b12"},
	} {
		got, err := handler.GetHeatmap(ctx,
			connect.NewRequest(&surveyv1.GetHeatmapRequest{SurveyId: id, Metric: tc.metric}))
		if err != nil {
			t.Fatalf("GetHeatmap(%s): %v", tc.metric, err)
		}
		if d := fieldDigest(got.Msg); d != tc.digest {
			t.Errorf("%s heatmap digest = %s, want %s (main's strongest-AP view)", tc.metric, d, tc.digest)
		}
	}

	coverage, err := handler.GetCoverage(ctx,
		connect.NewRequest(&surveyv1.GetCoverageRequest{SurveyId: id}))
	if err != nil {
		t.Fatalf("GetCoverage: %v", err)
	}
	if got := coverage.Msg.GetDeadZoneCount(); got != 0 {
		t.Errorf("strongest-AP dead zones = %d, want 0: one AP or the other covers every point", got)
	}
	if got := coverage.Msg.GetCoverageScore(); got != 100 {
		t.Errorf("strongest-AP coverage score = %v, want 100", got)
	}
}

func TestGetHeatmap_ABSSIDDrawsWhereThatAPServes(t *testing.T) {
	t.Parallel()

	handler, id := twoAPFloorSurvey(t)
	ctx := context.Background()

	render := func(bssid *string) *surveyv1.GetHeatmapResponse {
		t.Helper()
		got, err := handler.GetHeatmap(ctx, connect.NewRequest(&surveyv1.GetHeatmapRequest{
			SurveyId: id, Metric: "rssi", Bssid: bssid,
		}))
		if err != nil {
			t.Fatalf("GetHeatmap(bssid=%v): %v", bssid, err)
		}
		return got.Msg
	}
	coverage := func(bssid *string) *surveyv1.GetCoverageResponse {
		t.Helper()
		got, err := handler.GetCoverage(ctx, connect.NewRequest(&surveyv1.GetCoverageRequest{
			SurveyId: id, Bssid: bssid,
		}))
		if err != nil {
			t.Fatalf("GetCoverage(bssid=%v): %v", bssid, err)
		}
		return got.Msg
	}

	all := render(nil)
	for _, tc := range []struct {
		bssid        string
		samples      int32
		strongest    float64
		weakest      float64
		weakFraction float64
	}{
		// Heard at all four points, weak at the eastern two.
		{westAP, 4, -45, -85, 0.5},
		// Not heard at the westernmost point: three readings, one weak. The
		// unheard point is left out, not scored as a dead zone.
		{eastAP, 3, -45, -80, 1.0 / 3},
	} {
		bssid := tc.bssid
		one := render(&bssid)
		if fieldDigest(one) == fieldDigest(all) {
			t.Errorf("%s: per-AP map is the strongest-AP map", bssid)
		}
		if got := one.GetSampleCount(); got != tc.samples {
			t.Errorf("%s: map drawn from %d points, want the %d that heard it", bssid, got, tc.samples)
		}
		// IDW never overshoots its inputs, so the field's range is bounded by
		// this AP's own readings and the strongest-AP -50 floor is gone.
		if one.GetMax() > tc.strongest+0.5 || one.GetMin() < tc.weakest-0.5 || one.GetMin() > -75 {
			t.Errorf("%s: field spans [%v, %v], want within [%v, %v] and reaching below -75",
				bssid, one.GetMin(), one.GetMax(), tc.weakest, tc.strongest)
		}

		cov := coverage(&bssid)
		if cov.GetDeadZoneCount() == 0 {
			t.Errorf("%s: no dead zones, want this AP's weak end", bssid)
		}
		wantScore := 100 * (1 - tc.weakFraction)
		if got := cov.GetCoverageScore(); math.Abs(got-wantScore) > 0.01 {
			t.Errorf("%s: coverage score = %v, want %v", bssid, got, wantScore)
		}
	}

	// Filtering is a view: the floor's own samples are untouched, so the
	// strongest-AP map and its verdict come back unchanged afterwards.
	if fieldDigest(render(nil)) != fieldDigest(all) {
		t.Error("strongest-AP map changed after a per-AP request")
	}
	if got := coverage(nil).GetDeadZoneCount(); got != 0 {
		t.Errorf("strongest-AP dead zones = %d after a per-AP request, want 0", got)
	}
}

func TestGetHeatmap_ListsTheFloorsAPsForTheSelector(t *testing.T) {
	t.Parallel()

	handler, id := twoAPFloorSurvey(t)
	east := eastAP

	// Named or not, the reply lists the whole floor: the list a caller picks
	// from must not collapse to the one AP it just picked.
	for _, bssid := range []*string{nil, &east} {
		got, err := handler.GetHeatmap(context.Background(),
			connect.NewRequest(&surveyv1.GetHeatmapRequest{SurveyId: id, Bssid: bssid}))
		if err != nil {
			t.Fatalf("GetHeatmap(bssid=%v): %v", bssid, err)
		}
		aps := got.Msg.GetAccessPoints()
		want := []struct {
			bssid   string
			channel int32
			samples int32
		}{{westAP, 1, 4}, {eastAP, 11, 3}}
		if len(aps) != len(want) {
			t.Fatalf("bssid=%v: %d access points listed, want %d", bssid, len(aps), len(want))
		}
		for i, w := range want {
			ap := aps[i]
			if ap.GetBssid() != w.bssid || ap.GetSsid() != "corp" ||
				ap.GetChannel() != w.channel || ap.GetSamples() != w.samples {
				t.Errorf("bssid=%v: access point %d = %v, want %s corp ch %d heard at %d",
					bssid, i, ap, w.bssid, w.channel, w.samples)
			}
		}
	}
}

func TestPerAPRequestsRefuseWhatTheyCannotAnswer(t *testing.T) {
	t.Parallel()

	handler, id := twoAPFloorSurvey(t)
	ctx := context.Background()
	unheard := "aa:bb:cc:00:00:99"
	west := westAP

	for _, tc := range []struct {
		name   string
		metric string
		bssid  string
	}{
		// An empty map would say the AP covers nothing here.
		{"unheard BSSID", "rssi", unheard},
		// One AP's co-channel count or density is a constant, not a layer.
		{"density for one AP", "density", west},
		{"interference for one AP", "interference", west},
		{"empty BSSID", "rssi", ""},
	} {
		bssid := tc.bssid
		_, err := handler.GetHeatmap(ctx, connect.NewRequest(&surveyv1.GetHeatmapRequest{
			SurveyId: id, Metric: tc.metric, Bssid: &bssid,
		}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("GetHeatmap %s: err = %v, want InvalidArgument", tc.name, err)
		}
		// Refused because nobody heard it, not because an empty floor happens
		// to fail rendering too.
		if heardNowhere := tc.bssid != west; heardNowhere != errors.Is(err, survey.ErrBSSIDNotHeard) {
			t.Errorf("GetHeatmap %s: err = %v, ErrBSSIDNotHeard want %v", tc.name, err, heardNowhere)
		}
	}

	for _, bssid := range []string{unheard, ""} {
		_, err := handler.GetCoverage(ctx, connect.NewRequest(&surveyv1.GetCoverageRequest{
			SurveyId: id, Bssid: &bssid,
		}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("GetCoverage(bssid=%q): err = %v, want InvalidArgument", bssid, err)
		}
		if !errors.Is(err, survey.ErrBSSIDNotHeard) {
			t.Errorf("GetCoverage(bssid=%q): err = %v, want ErrBSSIDNotHeard", bssid, err)
		}
	}
}
