// SPDX-License-Identifier: BUSL-1.1

package api_test

import (
	"bytes"
	"context"
	"fmt"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
	surveyv1 "github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1"
	"github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1/surveyv1connect"
	"github.com/MustardSeedNetworks/trellis/internal/api"
)

// TestLegendSwatchIsTheColourOnTheMap ties the two halves of one reply
// together: the swatch a client paints from GetHeatmapResponse.legend is the
// colour that same reply's PNG has at that value.
//
// The existing tests each assert against GetRSSIColorScale() — the image in
// core/survey, the legend here — so they agree only as long as both keep
// reading the same constructor. This one names no scale at all. It asserts a
// property of the response, which is what a client actually has, and it holds
// through any ramp change: UI-TRL-9 replaces the colours and this test still
// asks the only question that matters, whether the key describes the picture.
func TestLegendSwatchIsTheColourOnTheMap(t *testing.T) {
	t.Parallel()

	// A stop value rather than a point between two, so the pixel is the
	// swatch itself rather than an interpolation of two of them.
	const uniformRSSI = -75

	mgr := mustManager(t, t.TempDir(), nil, nil, nil, nil)
	handler := api.NewSurveyServiceHandler(mgr)

	mux := http.NewServeMux()
	path, connectHandler := surveyv1connect.NewSurveyServiceHandler(handler)
	mux.Handle(path, connectHandler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := surveyv1connect.NewSurveyServiceClient(srv.Client(), srv.URL)
	ctx := context.Background()

	svy, err := mgr.CreateSurvey("Uniform floor", "", "wlan0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	if err := mgr.StartSurvey(svy.ID); err != nil {
		t.Fatalf("StartSurvey: %v", err)
	}
	// Every sample reads the same, so every interpolated cell is that value
	// and any cell the probe lands in is the one colour under test.
	for _, p := range []struct{ x, y int }{{50, 50}, {350, 60}, {80, 300}, {360, 320}} {
		sample := &survey.PassiveSample{
			Networks: []*wifi.ScannedNetwork{{
				SSID: "MSN-Corp", BSSID: "aa:bb:cc:00:00:01", Signal: uniformRSSI,
				Channel: 36, Frequency: 5180, NoiseFloor: -95, SNR: uniformRSSI + 95,
			}},
			UniqueSSIDs: 1, UniqueBSSIDs: 1, APCount5: 1,
		}
		if err := mgr.AddSample(svy.ID, p.x, p.y, sample); err != nil {
			t.Fatalf("AddSample(%d,%d): %v", p.x, p.y, err)
		}
	}

	resp, err := client.GetHeatmap(ctx, connect.NewRequest(&surveyv1.GetHeatmapRequest{
		SurveyId: svy.ID,
		Metric:   "rssi",
	}))
	if err != nil {
		t.Fatalf("GetHeatmap: %v", err)
	}

	var swatch string
	for _, stop := range resp.Msg.GetLegend() {
		if stop.GetValue() == uniformRSSI {
			swatch = stop.GetColor()
		}
	}
	if swatch == "" {
		t.Fatalf("legend has no stop at %d dBm: %v", uniformRSSI, resp.Msg.GetLegend())
	}

	decoded, err := png.Decode(bytes.NewReader(resp.Msg.GetPng()))
	if err != nil {
		t.Fatalf("decode heatmap png: %v", err)
	}
	b := decoded.Bounds()
	// Inside the sampled region but clear of the sample markers, which the
	// default config draws and which paint their own colour.
	px, ok := decoded.At(b.Min.X+b.Dx()/4, b.Min.Y+b.Dy()/4).(color.NRGBA)
	if !ok {
		t.Fatalf("heatmap pixel is %T, want color.NRGBA", decoded.At(b.Min.X+b.Dx()/4, b.Min.Y+b.Dy()/4))
	}
	if px.A == 0 {
		t.Fatal("probed heatmap pixel is fully transparent — nothing was painted there")
	}

	// The cell is drawn at the config's opacity and png.Encode round-trips
	// that through a premultiplied byte, so the recovered colour is within a
	// channel step or two of the swatch — orders of magnitude tighter than
	// the drift this test exists to catch.
	const tolerance = 3
	got := fmt.Sprintf("#%02x%02x%02x", px.R, px.G, px.B)
	var want color.NRGBA
	if _, err := fmt.Sscanf(swatch, "#%02x%02x%02x", &want.R, &want.G, &want.B); err != nil {
		t.Fatalf("legend colour %q is not #rrggbb: %v", swatch, err)
	}
	if !within(px.R, want.R, tolerance) || !within(px.G, want.G, tolerance) || !within(px.B, want.B, tolerance) {
		t.Errorf("map pixel at %d dBm is %s, but the legend swatch for %d dBm is %s",
			uniformRSSI, got, uniformRSSI, swatch)
	}
}

func within(got, want, tolerance uint8) bool {
	if got > want {
		return got-want <= tolerance
	}
	return want-got <= tolerance
}
