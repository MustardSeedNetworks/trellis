package survey_test

// heatmap_floorplan_test.go pins the floor plan as the heatmap's base layer.
//
// A coverage map is read against the building: colour alone does not say which
// room is weak. The plan arrives with every AirMapper capture and is stored on
// the floor, and the heatmap canvas is sized from it, so the only thing that
// can go wrong is the drawing — which is what this covers.
//
// It asserts pixels rather than "an image came back". Heat is composited at
// 70% alpha, so a plan that is dropped, or drawn and then overwritten by the
// heat layer, still yields a perfectly plausible PNG of the right size.

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// solidPlanDataURI builds a PNG data URI of one flat colour, standing in for a
// real floor plan. A flat colour is what makes the assertions readable: any
// difference in the output is the plan showing through.
func solidPlanDataURI(t *testing.T, c color.NRGBA, w, h int) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode plan: %v", err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

// renderWithPlan surveys the same three points over the supplied plan and
// returns the decoded heatmap, so only the plan varies between cases.
func renderWithPlan(t *testing.T, planData string, w, h int) image.Image {
	t.Helper()
	mgr := mustManager(t, t.TempDir(), nil, nil, nil, nil)
	svy, err := mgr.CreateSurvey("Plan", "floor plan base layer", "wlan0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	if err := mgr.StartSurvey(svy.ID); err != nil {
		t.Fatalf("StartSurvey: %v", err)
	}
	floor := svy.GetActiveFloor()
	if floor == nil {
		t.Fatal("no active floor")
	}
	floor.FloorPlan = &survey.FloorPlan{ImageData: planData, Width: w, Height: h, ScaleM: 0.05}

	now := time.Now().UTC()
	for i, pt := range [][2]int{{10, 10}, {40, 30}, {70, 50}} {
		nets := []*wifi.ScannedNetwork{{
			SSID: "ap", BSSID: "00:00:00:00:00:01",
			Signal: -40 - i*10, Channel: 36, Frequency: 5180, LastSeen: now,
		}}
		if err := mgr.AddSample(svy.ID, pt[0], pt[1], &survey.PassiveSample{Networks: nets}); err != nil {
			t.Fatalf("AddSample %d: %v", i, err)
		}
	}

	res, err := mgr.GenerateHeatmap(svy.ID, survey.DefaultHeatmapConfig())
	if err != nil {
		t.Fatalf("GenerateHeatmap: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(res.Image))
	if err != nil {
		t.Fatalf("decode heatmap: %v", err)
	}
	if got := img.Bounds().Dx(); got != w {
		t.Fatalf("heatmap width = %d, want the plan's %d", got, w)
	}
	return img
}

// planWhite is the stand-in plan colour shared with the report tests.
var planWhite = color.NRGBA{R: 255, G: 255, B: 255, A: 255}

func TestHeatmapDrawsOnTheFloorPlan(t *testing.T) {
	const w, h = 80, 60
	// Sampled away from the survey points, where the plan carries the picture
	// and the sample markers do not.
	probeX, probeY := 5, 55

	blue := renderWithPlan(t, solidPlanDataURI(t, color.NRGBA{R: 0, G: 0, B: 255, A: 255}, w, h), w, h)
	white := renderWithPlan(t, solidPlanDataURI(t, color.NRGBA{R: 255, G: 255, B: 255, A: 255}, w, h), w, h)
	none := renderWithPlan(t, "", w, h)

	blueAt := blue.At(probeX, probeY)
	whiteAt := white.At(probeX, probeY)
	noneAt := none.At(probeX, probeY)

	if blueAt == whiteAt {
		t.Errorf("the same heat over a blue plan and a white plan rendered identically (%v) — the plan is not being drawn", blueAt)
	}
	if blueAt == noneAt {
		t.Errorf("a blue plan rendered the same as no plan at all (%v)", blueAt)
	}
	// Heat is translucent, so the plan tints the result without replacing it:
	// over a blue plan the pixel must carry more blue than it does over none.
	_, _, bBlue, _ := blueAt.RGBA()
	_, _, bNone, _ := noneAt.RGBA()
	if bBlue <= bNone {
		t.Errorf("blue channel over a blue plan = %d, over no plan = %d — want the plan showing through the heat", bBlue, bNone)
	}
}

func TestHeatmapSurvivesAnUndecodablePlan(t *testing.T) {
	// A plan we cannot read is not worth failing the measurements over.
	const w, h = 40, 30
	img := renderWithPlan(t, "data:image/png;base64,bm90LWFuLWltYWdl", w, h)
	if img.Bounds().Dy() != h {
		t.Fatalf("height = %d, want %d", img.Bounds().Dy(), h)
	}
}
