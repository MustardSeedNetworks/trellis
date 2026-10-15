package survey_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

func predictedConfig(band string) survey.HeatmapConfig {
	config := survey.DefaultHeatmapConfig()
	config.Type = survey.HeatmapPredictedRSSI
	config.Band = band
	return config
}

// plannedFloor is a 250×170 px plan at 0.1 m/px, deliberately not a multiple
// of the 10 px cell so the edge cells are partial, with one AP and a short
// walk.
func plannedFloor() *survey.Floor {
	power := 17.0
	return &survey.Floor{
		ID:        "f1",
		FloorPlan: &survey.FloorPlan{Width: 250, Height: 170, ScaleM: 0.1},
		APLocations: []survey.APLocation{
			{ID: "ap1", X: 55, Y: 55, Band: "5 GHz", TxPowerDBm: &power},
		},
		Samples: []*survey.SamplePoint{
			walkedPoint(20, 20, -48), walkedPoint(200, 40, -66), walkedPoint(120, 150, -71),
		},
	}
}

func walkedPoint(x, y, rssi int) *survey.SamplePoint {
	sample := &survey.PassiveSample{Networks: []*wifi.ScannedNetwork{
		{SSID: "lab", BSSID: "aa:bb:cc:00:00:01", Signal: rssi, SNR: rssi + 95, Channel: 36, Frequency: 5180},
	}}
	sample.CalculateAggregations()
	return &survey.SamplePoint{X: x, Y: y, Timestamp: time.Now(), SampleData: sample}
}

func TestPredictedHeatmapGolden(t *testing.T) {
	result, err := survey.GenerateFloorHeatmap(plannedFloor(), predictedConfig("5 GHz"))
	if err != nil {
		t.Fatalf("GenerateFloorHeatmap: %v", err)
	}
	// Cell (5, 5) is centred on the AP at (55, 55) px, so the distance floors
	// at 1 m: 17 dBm − FSPL(5500 MHz, 1 m) = 17 − 47.257.
	if got, want := result.Grid[5][5], -30.257; math.Abs(got-want) > 0.1 {
		t.Errorf("cell under the AP = %.3f dBm, want %.3f", got, want)
	}
	// Cell (5, 15) is 100 px = 10 m east: −30.257 − 30·log10(10).
	if got, want := result.Grid[5][15], -60.257; math.Abs(got-want) > 0.1 {
		t.Errorf("cell 10 m east = %.3f dBm, want %.3f", got, want)
	}
	if result.Type != string(survey.HeatmapPredictedRSSI) {
		t.Errorf("type = %q, want %q", result.Type, survey.HeatmapPredictedRSSI)
	}
	if len(result.Image) == 0 || result.Stats.Max <= result.Stats.Min {
		t.Errorf("image %d bytes, stats %+v: want a painted map with a range", len(result.Image), result.Stats)
	}
}

func TestPredictedHeatmapHasTheMeasuredGridShape(t *testing.T) {
	floor := plannedFloor()
	measured, err := survey.GenerateFloorHeatmap(floor, survey.DefaultHeatmapConfig())
	if err != nil {
		t.Fatalf("measured: %v", err)
	}
	predicted, err := survey.GenerateFloorHeatmap(floor, predictedConfig("5 GHz"))
	if err != nil {
		t.Fatalf("predicted: %v", err)
	}
	if len(predicted.Grid) != len(measured.Grid) || len(predicted.Grid[0]) != len(measured.Grid[0]) ||
		predicted.CellSize != measured.CellSize ||
		predicted.Width != measured.Width || predicted.Height != measured.Height {
		t.Errorf("predicted %d×%d cells of %d px on %d×%d, measured %d×%d cells of %d px on %d×%d",
			len(predicted.Grid[0]), len(predicted.Grid), predicted.CellSize, predicted.Width, predicted.Height,
			len(measured.Grid[0]), len(measured.Grid), measured.CellSize, measured.Width, measured.Height)
	}
}

func TestPredictedHeatmapDefaultsAndBands(t *testing.T) {
	floor := plannedFloor()
	floor.APLocations = []survey.APLocation{
		{ID: "unstated", X: 55, Y: 55},
		{ID: "other-band", X: 155, Y: 55, Band: "2.4 GHz"},
	}
	result, err := survey.GenerateFloorHeatmap(floor, predictedConfig("5 GHz"))
	if err != nil {
		t.Fatalf("GenerateFloorHeatmap: %v", err)
	}
	// No stated power is the 10 dBm default; no stated band radiates on 5 GHz;
	// the 2.4 GHz AP 10 m away must not lift the 5 GHz map.
	if got, want := result.Grid[5][5], -37.257; math.Abs(got-want) > 0.1 {
		t.Errorf("cell under the unstated AP = %.3f dBm, want %.3f", got, want)
	}
	if got, want := result.Grid[5][15], -67.257; math.Abs(got-want) > 0.1 {
		t.Errorf("cell under the 2.4 GHz AP on the 5 GHz map = %.3f dBm, want %.3f", got, want)
	}
}

func TestPredictedHeatmapRefusals(t *testing.T) {
	for _, tc := range []struct {
		name  string
		floor func(*survey.Floor)
		band  string
		want  error
	}{
		{"no plan", func(f *survey.Floor) { f.FloorPlan = nil }, "5 GHz", survey.ErrFloorUnscaled},
		{"no scale", func(f *survey.Floor) { f.FloorPlan.ScaleM = 0 }, "5 GHz", survey.ErrFloorUnscaled},
		{"no placements", func(f *survey.Floor) { f.APLocations = nil }, "5 GHz", survey.ErrNoPlacements},
		{"none on this band", func(*survey.Floor) {}, "2.4 GHz", survey.ErrNoPlacements},
		{"unknown band", func(*survey.Floor) {}, "", survey.ErrUnknownBand},
	} {
		t.Run(tc.name, func(t *testing.T) {
			floor := plannedFloor()
			tc.floor(floor)
			result, err := survey.GenerateFloorHeatmap(floor, predictedConfig(tc.band))
			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
			if result != nil {
				t.Errorf("a refused prediction returned a map")
			}
		})
	}
}

func TestPredictionIsNotASurveyWideLayer(t *testing.T) {
	svy := &survey.Survey{Floors: []*survey.Floor{plannedFloor()}}
	if _, err := survey.GenerateHeatmap(svy, predictedConfig("5 GHz")); err == nil {
		t.Error("GenerateHeatmap drew a prediction across a whole survey; want a refusal")
	}
}

func TestParseHeatmapTypePredicted(t *testing.T) {
	if got := survey.ParseHeatmapType("predicted_rssi"); got != survey.HeatmapPredictedRSSI {
		t.Errorf("ParseHeatmapType(predicted_rssi) = %q", got)
	}
}
