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

// blackRuns measures how a row of the image reads: the lengths of the
// consecutive runs of marker-black pixels along it. A walk drawn one marker
// per reading collapses into a single enormous run, which is the defect in
// #326; individually readable markers are many short runs with the heat
// showing between them.
func blackRuns(img image.Image, y, fromX, toX int) []int {
	var runs []int
	run := 0
	for x := fromX; x <= toX; x++ {
		r, g, b, a := img.At(x, y).RGBA()
		if r == 0 && g == 0 && b == 0 && a == 0xffff {
			run++
			continue
		}
		if run > 0 {
			runs = append(runs, run)
			run = 0
		}
	}
	if run > 0 {
		runs = append(runs, run)
	}
	return runs
}

func maxRun(runs []int) int {
	longest := 0
	for _, r := range runs {
		if r > longest {
			longest = r
		}
	}
	return longest
}

// A 600-point walk is what continuous capture and every import produce. Drawn
// at the marker size a 3-point stop-and-go walk needs, they merge into one
// solid line over the values they measured.
func TestRenderSamplePointsDenseWalkStaysReadable(t *testing.T) {
	const (
		count = 600
		lineY = 50
	)
	img := survey.CreateTestImage(count+20, 100)
	samples := make([]survey.SampleValue, 0, count)
	for i := range count {
		samples = append(samples, survey.SampleValue{
			Point: survey.Point2D{X: float64(10 + i), Y: lineY},
			Value: -60,
		})
	}

	survey.ExportRenderSamplePoints(img, samples)

	runs := blackRuns(img, lineY, 0, count+19)
	if len(runs) < 100 {
		t.Errorf("expected the walk to read as many separate markers, got %d runs", len(runs))
	}
	if longest := maxRun(runs); longest > 9 {
		t.Errorf("expected no marker run longer than one full marker (9 px), got %d", longest)
	}
}

// The sparse case is what the marker size was designed against and must not
// change: an operator who dropped three pins still sees where they stood.
func TestRenderSamplePointsSparseWalkKeepsFullMarkers(t *testing.T) {
	const lineY = 50
	img := survey.CreateTestImage(200, 100)
	xs := []int{20, 60, 100}
	samples := make([]survey.SampleValue, 0, len(xs))
	for _, x := range xs {
		samples = append(samples, survey.SampleValue{
			Point: survey.Point2D{X: float64(x), Y: lineY},
			Value: -60,
		})
	}

	survey.ExportRenderSamplePoints(img, samples)

	for _, x := range xs {
		edge := img.At(x+4, lineY).(color.RGBA)
		if edge.R != 0 || edge.G != 0 || edge.B != 0 {
			t.Errorf("expected the full marker edge at (%d,%d), got %v", x+4, lineY, edge)
		}
		centre := img.At(x, lineY).(color.RGBA)
		if centre.R != 255 || centre.G != 255 || centre.B != 255 {
			t.Errorf("expected the white centre at (%d,%d), got %v", x, lineY, centre)
		}
	}
}

// The Coverage page and the PDF report both render through
// GenerateFloorHeatmap, so proving the density rule here proves both surfaces
// #326 names.
func TestGenerateFloorHeatmapDenseWalkStaysReadable(t *testing.T) {
	const (
		count = 600
		lineY = 50
	)
	floor := &survey.Floor{ID: "floor-1", Name: "Floor 1"}
	for i := range count {
		floor.Samples = append(floor.Samples, &survey.SamplePoint{
			X: 10 + i,
			Y: lineY,
			SampleData: &survey.PassiveSample{
				Networks: []*wifi.ScannedNetwork{{Signal: -60, SNR: 30}},
			},
		})
	}

	result, err := survey.GenerateFloorHeatmap(floor, survey.DefaultHeatmapConfig())
	if err != nil {
		t.Fatalf("GenerateFloorHeatmap: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(result.Image))
	if err != nil {
		t.Fatalf("decode heatmap PNG: %v", err)
	}
	runs := blackRuns(img, lineY, 0, img.Bounds().Max.X-1)
	if len(runs) < 100 {
		t.Errorf("expected many separate markers in the rendered heatmap, got %d runs", len(runs))
	}
	if longest := maxRun(runs); longest > 9 {
		t.Errorf("expected no marker run longer than one full marker (9 px), got %d", longest)
	}
}
