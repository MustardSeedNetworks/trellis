package survey_test

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
)

func deltaConfig(band string) survey.HeatmapConfig {
	config := predictedConfig(band)
	config.Type = survey.HeatmapPredictedDelta
	return config
}

// comparedFloor is plannedFloor with its AP's BSSID recorded, so the walk's
// readings of it can be joined to the prediction.
func comparedFloor() *survey.Floor {
	floor := plannedFloor()
	floor.APLocations[0].BSSID = "AA:BB:CC:00:00:01"
	return floor
}

func TestPredictedDeltaIsPredictionMinusWalk(t *testing.T) {
	floor := comparedFloor()
	predicted, err := survey.GenerateFloorHeatmap(floor, predictedConfig("5 GHz"))
	if err != nil {
		t.Fatalf("predicted: %v", err)
	}
	delta, err := survey.GenerateFloorHeatmap(floor, deltaConfig("5 GHz"))
	if err != nil {
		t.Fatalf("delta: %v", err)
	}

	// plannedFloor's walk: one reading per cell, so each walked cell's delta
	// is the prediction there minus that reading, and every other cell is empty.
	walked := map[[2]int]float64{{2, 2}: -48, {4, 20}: -66, {15, 12}: -71}
	for row := range delta.Grid {
		for col, d := range delta.Grid[row] {
			reading, ok := walked[[2]int{row, col}]
			if !ok {
				if !math.IsNaN(d) {
					t.Errorf("cell (%d,%d) = %v with no walk in it, want empty", row, col, d)
				}
				continue
			}
			if want := predicted.Grid[row][col] - reading; math.Abs(d-want) > 1e-9 {
				t.Errorf("cell (%d,%d) = %.3f dB, want %.3f", row, col, d, want)
			}
		}
	}
	if delta.PredictionError == nil || delta.PredictionError.Cells != 3 {
		t.Fatalf("prediction error = %+v, want 3 cells", delta.PredictionError)
	}
	if delta.SampleCount != 3 || delta.Type != string(survey.HeatmapPredictedDelta) {
		t.Errorf("sample count %d, type %q", delta.SampleCount, delta.Type)
	}
	if math.IsNaN(delta.Stats.Min) || delta.Stats.Count != 3 {
		t.Errorf("stats %+v: want the three defined cells only", delta.Stats)
	}
}

func TestPredictedDeltaRefusals(t *testing.T) {
	for _, tc := range []struct {
		name  string
		floor func(*survey.Floor)
		band  string
		want  error
	}{
		// A placement with no BSSID cannot be found in the walk.
		{"no BSSID placed", func(f *survey.Floor) { f.APLocations[0].BSSID = "" }, "5 GHz", survey.ErrNoPlacements},
		{"placed AP never heard", func(f *survey.Floor) { f.APLocations[0].BSSID = "aa:bb:cc:00:00:99" }, "5 GHz",
			survey.ErrNothingToCompare},
		{"no scale", func(f *survey.Floor) { f.FloorPlan.ScaleM = 0 }, "5 GHz", survey.ErrFloorUnscaled},
		{"none on this band", func(*survey.Floor) {}, "2.4 GHz", survey.ErrNoPlacements},
	} {
		t.Run(tc.name, func(t *testing.T) {
			floor := comparedFloor()
			tc.floor(floor)
			result, err := survey.GenerateFloorHeatmap(floor, deltaConfig(tc.band))
			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
			if result != nil {
				t.Error("a refused delta returned a map")
			}
		})
	}
}

func TestParseHeatmapTypePredictedDelta(t *testing.T) {
	if got := survey.ParseHeatmapType("predicted_delta"); got != survey.HeatmapPredictedDelta {
		t.Errorf("ParseHeatmapType(predicted_delta) = %q", got)
	}
}

func TestReportCarriesThePredictionPages(t *testing.T) {
	floor := comparedFloor()
	floor.Name = "Ground"
	render := func(floor *survey.Floor) []byte {
		t.Helper()
		pdf, err := survey.NewReportGenerator(
			&survey.Survey{ID: "s", Name: "Compared", Floors: []*survey.Floor{floor}},
			survey.DefaultReportOptions(),
		).Generate()
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		return pdf
	}

	with := render(floor)
	unplaced := comparedFloor()
	unplaced.Name = "Ground"
	unplaced.APLocations = nil
	without := render(unplaced)

	pages := regexp.MustCompile(`/Type /Page\b`)
	if got, base := len(pages.FindAll(with, -1)), len(pages.FindAll(without, -1)); got != base+2 {
		t.Errorf("pages = %d with a placed AP, %d without; want two more", got, base)
	}
	// Each map is a translucent PNG, which fpdf embeds as an image and its
	// soft mask: two maps are four image objects.
	if got, base := len(imageXObject.FindAll(with, -1)), len(imageXObject.FindAll(without, -1)); got != base+4 {
		t.Errorf("image objects = %d with a placed AP, %d without; want two maps more", got, base)
	}

	delta, err := survey.GenerateFloorHeatmap(floor, deltaConfig("5 GHz"))
	if err != nil {
		t.Fatalf("delta: %v", err)
	}
	// A wrapped caption prints one line per Tj; joining them restores the words.
	text := strings.ReplaceAll(reportText(t, with), "\n", " ")
	for _, want := range []string{
		"Predicted Signal 5 GHz: Ground",
		"Prediction Error 5 GHz: Ground",
		fmt.Sprintf("mean absolute error %.1f dB", delta.PredictionError.MeanAbsDB),
		fmt.Sprintf("95th percentile %.1f dB", delta.PredictionError.P95AbsDB),
	} {
		if !strings.Contains(text, want) {
			t.Errorf("report does not say %q", want)
		}
	}
}
