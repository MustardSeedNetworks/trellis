// SPDX-License-Identifier: BUSL-1.1

package survey_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// TestHeatmapPixelsMatchTheScale proves the image says what the colour scale
// says.
//
// image.RGBA holds premultiplied alpha and png.Encode un-premultiplies on the
// way out. The renderer used to store straight-alpha colours, so that division
// overflowed and wrapped: a -75 dBm cell that the scale paints orange left the
// encoder as olive, and weaker signal came out magenta. Nothing caught it,
// because every test asserted on the scale or on the image's dimensions and
// none on the colour of a pixel.
func TestHeatmapPixelsMatchTheScale(t *testing.T) {
	const rssi = -75

	mgr := survey.NewManager(t.TempDir(), nil, nil, nil, nil)
	svy, err := mgr.CreateSurvey("Uniform floor", "", "wlan0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	if err := mgr.StartSurvey(svy.ID); err != nil {
		t.Fatalf("StartSurvey: %v", err)
	}
	// Every sample reads the same, so every interpolated cell is the same
	// value and the whole image is one colour off the scale.
	for _, p := range []struct{ x, y int }{{50, 50}, {350, 60}, {80, 300}, {360, 320}} {
		sample := &survey.PassiveSample{
			Networks: []*wifi.ScannedNetwork{{
				SSID: "MSN-Corp", BSSID: "aa:bb:cc:00:00:01", Signal: rssi,
				Channel: 36, Frequency: 5180, NoiseFloor: -95, SNR: rssi + 95,
			}},
			UniqueSSIDs: 1, UniqueBSSIDs: 1, APCount5: 1,
		}
		if err := mgr.AddSample(svy.ID, p.x, p.y, sample); err != nil {
			t.Fatalf("AddSample(%d,%d): %v", p.x, p.y, err)
		}
	}

	config := survey.DefaultHeatmapConfig()
	config.Type = survey.HeatmapRSSI
	config.ShowSamples = false // sample markers paint their own colour
	result, err := survey.GenerateHeatmap(svy, config)
	if err != nil {
		t.Fatalf("GenerateHeatmap: %v", err)
	}

	decoded, err := png.Decode(bytes.NewReader(result.Image))
	if err != nil {
		t.Fatalf("decode heatmap png: %v", err)
	}

	scale := survey.GetRSSIColorScale()
	want := scale.GetColor(rssi)
	got := color.NRGBAModel.Convert(decoded.At(sampledPixel(decoded))).(color.NRGBA)

	// Premultiplying to a byte and back is lossy, so the comparison is to
	// within a channel step or two — far tighter than a hue shift.
	const tolerance = 3
	if !within(got.R, want.R, tolerance) || !within(got.G, want.G, tolerance) || !within(got.B, want.B, tolerance) {
		t.Errorf("pixel at %.0f dBm = RGB(%d,%d,%d), want the scale's RGB(%d,%d,%d)",
			float64(rssi), got.R, got.G, got.B, want.R, want.G, want.B)
	}
	if got.A == 0 {
		t.Error("heatmap cell is fully transparent")
	}
}

// sampledPixel picks a point inside the sampled region rather than the image
// centre, which on a sparse survey can fall outside the interpolated area.
func sampledPixel(img image.Image) (int, int) {
	b := img.Bounds()
	return b.Min.X + (b.Dx() / 4), b.Min.Y + (b.Dy() / 4)
}

func within(got, want, tolerance uint8) bool {
	if got > want {
		return got-want <= tolerance
	}
	return want-got <= tolerance
}
