// SPDX-License-Identifier: BUSL-1.1

package api_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
	surveyv1 "github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1"
	"github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1/surveyv1connect"
	"github.com/MustardSeedNetworks/trellis/internal/api"
)

// buildAMP synthesizes a minimal but valid AirMapper (.amp) archive: a JSON
// `.serial` metadata file plus a real PNG floor plan. Mirrors the fixture
// builder in core/survey/import_test.go so the API test exercises the same
// real import path, not a mock.
func buildAMP(t *testing.T, w, h int, scalePpf float64) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	serial := survey.SerialMetadata{
		FileName:          "floorplan.png",
		FloorPlanScalePpf: scalePpf,
		Propagation:       15,
		PropagationUnit:   "feet",
		SurveyPointCount:  0,
	}
	serialJSON, err := json.Marshal(serial)
	if err != nil {
		t.Fatalf("marshal serial: %v", err)
	}

	var ampBuf bytes.Buffer
	zw := zip.NewWriter(&ampBuf)
	writeEntry := func(name string, data []byte) {
		f, wErr := zw.Create(name)
		if wErr != nil {
			t.Fatalf("zip create %s: %v", name, wErr)
		}
		if _, wErr := f.Write(data); wErr != nil {
			t.Fatalf("zip write %s: %v", name, wErr)
		}
	}
	writeEntry("survey.serial", serialJSON)
	writeEntry("floorplan.png", pngBuf.Bytes())
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return ampBuf.Bytes()
}

// TestSurveyServiceEndToEnd proves the connectrpc API is wired to the real
// survey engine: import a real .amp archive over ImportAirMapper, add
// measured samples directly through the shared manager (the API does not yet
// expose live capture), then verify GetHeatmap and GetCoverage return
// genuine, engine-computed results.
func TestSurveyServiceEndToEnd(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mgr := survey.NewManager(dir, nil, nil, nil, nil)
	handler := api.NewSurveyServiceHandler(mgr)

	mux := http.NewServeMux()
	path, connectHandler := surveyv1connect.NewSurveyServiceHandler(handler)
	mux.Handle(path, connectHandler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := surveyv1connect.NewSurveyServiceClient(srv.Client(), srv.URL)
	ctx := context.Background()

	// --- ImportAirMapper over the API ---------------------------------
	const (
		w, h     = 1600, 1200
		scalePpf = 20.0
	)
	amp := buildAMP(t, w, h, scalePpf)

	importResp, err := client.ImportAirMapper(ctx, connect.NewRequest(&surveyv1.ImportAirMapperRequest{
		Name:    "Everett HQ",
		AmpData: amp,
	}))
	if err != nil {
		t.Fatalf("ImportAirMapper: %v", err)
	}
	summary := importResp.Msg.GetSurvey()
	if summary.GetId() == "" {
		t.Fatal("ImportAirMapper: empty survey id")
	}
	if summary.GetName() != "Everett HQ" {
		t.Errorf("Name = %q, want Everett HQ", summary.GetName())
	}
	if !summary.GetHasFloorPlan() {
		t.Error("HasFloorPlan = false, want true after AirMapper import")
	}
	if summary.GetFloorCount() != 1 {
		t.Errorf("FloorCount = %d, want 1", summary.GetFloorCount())
	}

	// --- Seed measured samples directly through the manager -----------
	// The API doesn't expose live capture yet, so samples are added the same
	// way core/survey's own pipeline test does: through the manager.
	surveyID := summary.GetId()
	if err := mgr.StartSurvey(surveyID); err != nil {
		t.Fatalf("StartSurvey: %v", err)
	}
	svy, err := mgr.GetSurvey(surveyID)
	if err != nil {
		t.Fatalf("GetSurvey: %v", err)
	}
	floor := svy.GetActiveFloor()
	if floor == nil {
		t.Fatal("no active floor after import")
	}

	points := []struct {
		x, y, rssi int
	}{
		{100, 100, -42},
		{600, 200, -55},
		{1100, 300, -68},
		{300, 700, -60},
		{900, 900, -74},
		{1500, 1100, -88}, // dead corner
	}
	for _, p := range points {
		net := &wifi.ScannedNetwork{
			SSID: "MSN-Corp", BSSID: "aa:bb:cc:00:00:01",
			Signal: p.rssi, Channel: 36, Frequency: 5180,
			ChannelWidth: 80, NoiseFloor: -95, SNR: p.rssi + 95,
		}
		sample := &survey.PassiveSample{
			Networks:     []*wifi.ScannedNetwork{net},
			UniqueSSIDs:  1,
			UniqueBSSIDs: 1,
			APCount5:     1,
		}
		if err := mgr.AddSampleToFloor(surveyID, floor.ID, p.x, p.y, sample); err != nil {
			t.Fatalf("AddSampleToFloor(%d,%d): %v", p.x, p.y, err)
		}
	}

	// --- GetHeatmap over the API ---------------------------------------
	heatmapResp, err := client.GetHeatmap(ctx, connect.NewRequest(&surveyv1.GetHeatmapRequest{
		SurveyId: surveyID,
		Metric:   "rssi",
	}))
	if err != nil {
		t.Fatalf("GetHeatmap: %v", err)
	}
	hm := heatmapResp.Msg
	if len(hm.GetPng()) < 8 || !strings.HasPrefix(string(hm.GetPng()[:8]), "\x89PNG") {
		t.Errorf("heatmap response is not a PNG (len=%d)", len(hm.GetPng()))
	}
	if hm.GetWidth() != w || hm.GetHeight() != h {
		t.Errorf("heatmap dims = %dx%d, want %dx%d", hm.GetWidth(), hm.GetHeight(), w, h)
	}
	if hm.GetSampleCount() != int32(len(points)) {
		t.Errorf("heatmap SampleCount = %d, want %d", hm.GetSampleCount(), len(points))
	}
	if hm.GetMax() <= hm.GetMin() {
		t.Errorf("heatmap has no gradient: min=%.1f max=%.1f", hm.GetMin(), hm.GetMax())
	}

	// --- GetCoverage over the API ---------------------------------------
	coverageResp, err := client.GetCoverage(ctx, connect.NewRequest(&surveyv1.GetCoverageRequest{
		SurveyId:     surveyID,
		ThresholdDbm: -75,
	}))
	if err != nil {
		t.Fatalf("GetCoverage: %v", err)
	}
	cov := coverageResp.Msg
	if cov.GetCoverageScore() < 0 || cov.GetCoverageScore() > 100 {
		t.Errorf("CoverageScore = %.1f, want 0..100", cov.GetCoverageScore())
	}
	if cov.GetDeadZoneCount() == 0 {
		t.Error("expected at least one dead zone below -75 dBm, got none")
	}

	// --- ListSurveys / GetSurvey / DeleteSurvey round trip --------------
	listResp, err := client.ListSurveys(ctx, connect.NewRequest(&surveyv1.ListSurveysRequest{}))
	if err != nil {
		t.Fatalf("ListSurveys: %v", err)
	}
	if len(listResp.Msg.GetSurveys()) != 1 {
		t.Errorf("ListSurveys returned %d surveys, want 1", len(listResp.Msg.GetSurveys()))
	}

	getResp, err := client.GetSurvey(ctx, connect.NewRequest(&surveyv1.GetSurveyRequest{Id: surveyID}))
	if err != nil {
		t.Fatalf("GetSurvey: %v", err)
	}
	if getResp.Msg.GetSurvey().GetSampleCount() != int32(len(points)) {
		t.Errorf("GetSurvey SampleCount = %d, want %d", getResp.Msg.GetSurvey().GetSampleCount(), len(points))
	}

	if _, err := client.DeleteSurvey(ctx, connect.NewRequest(&surveyv1.DeleteSurveyRequest{Id: surveyID})); err != nil {
		t.Fatalf("DeleteSurvey: %v", err)
	}
	if _, err := client.GetSurvey(ctx, connect.NewRequest(&surveyv1.GetSurveyRequest{Id: surveyID})); err == nil {
		t.Error("GetSurvey after delete: expected error, got nil")
	} else if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("GetSurvey after delete: code = %v, want CodeNotFound", connect.CodeOf(err))
	}
}

// TestGetSurveyNotFound proves unknown IDs surface as CodeNotFound, not a
// generic internal error.
func TestGetSurveyNotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mgr := survey.NewManager(dir, nil, nil, nil, nil)
	handler := api.NewSurveyServiceHandler(mgr)

	mux := http.NewServeMux()
	path, connectHandler := surveyv1connect.NewSurveyServiceHandler(handler)
	mux.Handle(path, connectHandler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := surveyv1connect.NewSurveyServiceClient(srv.Client(), srv.URL)

	_, err := client.GetSurvey(context.Background(), connect.NewRequest(&surveyv1.GetSurveyRequest{Id: "nonexistent"}))
	if err == nil {
		t.Fatal("expected error for unknown survey id")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("code = %v, want CodeNotFound", connect.CodeOf(err))
	}
}

// TestImportAirMapperInvalidArgument proves empty required fields surface as
// CodeInvalidArgument.
func TestImportAirMapperInvalidArgument(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mgr := survey.NewManager(dir, nil, nil, nil, nil)
	handler := api.NewSurveyServiceHandler(mgr)

	mux := http.NewServeMux()
	path, connectHandler := surveyv1connect.NewSurveyServiceHandler(handler)
	mux.Handle(path, connectHandler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := surveyv1connect.NewSurveyServiceClient(srv.Client(), srv.URL)

	_, err := client.ImportAirMapper(context.Background(), connect.NewRequest(&surveyv1.ImportAirMapperRequest{
		Name:    "",
		AmpData: []byte("irrelevant"),
	}))
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("code = %v, want CodeInvalidArgument", connect.CodeOf(err))
	}
}
