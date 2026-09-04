// SPDX-License-Identifier: BUSL-1.1

package survey_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
)

// planImage encodes a plan of the given size, so a test can assert the stored
// dimensions came from the image rather than from what the caller claimed.
func planImage(t *testing.T, width, height int, asJPEG bool) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := range width {
		img.Set(x, height/2, color.RGBA{R: 200, G: 200, B: 200, A: 255})
	}
	var buf bytes.Buffer
	var err error
	if asJPEG {
		err = jpeg.Encode(&buf, img, nil)
	} else {
		err = png.Encode(&buf, img)
	}
	if err != nil {
		t.Fatalf("encode plan: %v", err)
	}
	return buf.Bytes()
}

// planSurvey returns a manager and a survey with one floor to hang a plan on.
func planSurvey(t *testing.T) (*survey.Manager, string, string) {
	t.Helper()

	mgr := mustManager(t, t.TempDir(), &countingScanner{}, nil, nil, nil)
	s, err := mgr.CreateSurvey("plan", "", "en0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	if len(s.Floors) == 0 {
		t.Fatal("a new survey has no floor to hang a plan on")
	}
	return mgr, s.ID, s.Floors[0].ID
}

func TestSetFloorPlanReadsTheImageRatherThanTrustingTheCaller(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		jpeg bool
	}{
		{"png", false},
		{"jpeg", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mgr, id, floorID := planSurvey(t)
			if err := mgr.SetFloorPlan(id, floorID, planImage(t, 640, 480, tc.jpeg)); err != nil {
				t.Fatalf("SetFloorPlan: %v", err)
			}

			s, err := mgr.GetSurvey(id)
			if err != nil {
				t.Fatalf("GetSurvey: %v", err)
			}
			plan := s.GetFloorByID(floorID).FloorPlan
			if plan == nil {
				t.Fatal("no plan stored")
			}
			// Read from the image, not asserted by the uploader: every stored
			// point's position is in this pixel space, so a claimed size that
			// did not match would put the whole walk in the wrong place.
			if plan.Width != 640 || plan.Height != 480 {
				t.Errorf("plan is %dx%d, want 640x480", plan.Width, plan.Height)
			}
			if plan.ImageData == "" {
				t.Error("the plan carries no image")
			}
		})
	}
}

func TestSetFloorPlanRefusesWhatItCannotDraw(t *testing.T) {
	t.Parallel()

	mgr, id, floorID := planSurvey(t)

	// A file that is not an image would be stored, served, and fail to render
	// in the browser — a plan that silently is not there.
	if err := mgr.SetFloorPlan(id, floorID, []byte("this is not a floor plan")); err == nil {
		t.Fatal("want an error for a file that is not an image")
	}
	if err := mgr.SetFloorPlan(id, floorID, nil); err == nil {
		t.Fatal("want an error for an empty upload")
	}
	if err := mgr.SetFloorPlan(id, "no-such-floor", planImage(t, 10, 10, false)); !errors.Is(err, survey.ErrFloorNotFound) {
		t.Fatalf("unknown floor = %v, want ErrFloorNotFound", err)
	}
}

func TestCalibrateFloorPlanTurnsTwoPointsIntoAScale(t *testing.T) {
	t.Parallel()

	mgr, id, floorID := planSurvey(t)
	if err := mgr.SetFloorPlan(id, floorID, planImage(t, 800, 600, false)); err != nil {
		t.Fatalf("SetFloorPlan: %v", err)
	}

	// A 300-pixel line the operator says is 30 metres: a tenth of a metre per
	// pixel.
	if err := mgr.CalibrateFloorPlan(id, floorID, survey.Position{X: 100, Y: 100},
		survey.Position{X: 400, Y: 100}, 30); err != nil {
		t.Fatalf("CalibrateFloorPlan: %v", err)
	}

	s, _ := mgr.GetSurvey(id)
	if got := s.GetFloorByID(floorID).FloorPlan.ScaleM; got != 0.1 {
		t.Errorf("scale = %v m/px, want 0.1", got)
	}
}

func TestCalibrateFloorPlanMeasuresTheDiagonal(t *testing.T) {
	t.Parallel()

	mgr, id, floorID := planSurvey(t)
	if err := mgr.SetFloorPlan(id, floorID, planImage(t, 800, 600, false)); err != nil {
		t.Fatalf("SetFloorPlan: %v", err)
	}

	// 3-4-5: a 500-pixel line drawn as 300 across and 400 down. An operator
	// marking a diagonal wall gets the length of the wall, not of its shadow.
	if err := mgr.CalibrateFloorPlan(id, floorID, survey.Position{X: 0, Y: 0},
		survey.Position{X: 300, Y: 400}, 25); err != nil {
		t.Fatalf("CalibrateFloorPlan: %v", err)
	}

	s, _ := mgr.GetSurvey(id)
	if got := s.GetFloorByID(floorID).FloorPlan.ScaleM; got != 0.05 {
		t.Errorf("scale = %v m/px, want 0.05", got)
	}
}

func TestCalibrateFloorPlanRefusesAScaleItCannotCompute(t *testing.T) {
	t.Parallel()

	mgr, id, floorID := planSurvey(t)
	if err := mgr.SetFloorPlan(id, floorID, planImage(t, 800, 600, false)); err != nil {
		t.Fatalf("SetFloorPlan: %v", err)
	}

	for _, tc := range []struct {
		name     string
		from, to survey.Position
		metres   float64
	}{
		// Every one of these would divide by zero or produce a scale that makes
		// every distance on the plan nonsense.
		{"the same point twice", survey.Position{X: 5, Y: 5}, survey.Position{X: 5, Y: 5}, 10},
		{"no distance", survey.Position{X: 0, Y: 0}, survey.Position{X: 100, Y: 0}, 0},
		{"a negative distance", survey.Position{X: 0, Y: 0}, survey.Position{X: 100, Y: 0}, -5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := mgr.CalibrateFloorPlan(id, floorID, tc.from, tc.to, tc.metres); err == nil {
				t.Errorf("%s: want an error", tc.name)
			}
		})
	}
}

func TestCalibrationNeedsAPlanToCalibrate(t *testing.T) {
	t.Parallel()

	mgr, id, floorID := planSurvey(t)
	// The pixels a calibration is expressed in are the plan's. Without one
	// there is nothing for the two points to be points on.
	if err := mgr.CalibrateFloorPlan(id, floorID, survey.Position{X: 0, Y: 0},
		survey.Position{X: 100, Y: 0}, 10); !errors.Is(err, survey.ErrNoFloorPlan) {
		t.Fatalf("without a plan = %v, want ErrNoFloorPlan", err)
	}
}

func TestAFloorPlanSurvivesAReload(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mgr := mustManager(t, dir, &countingScanner{}, nil, nil, nil)
	s, _ := mgr.CreateSurvey("plan", "", "en0", survey.TypePassive)
	floorID := s.Floors[0].ID
	if err := mgr.SetFloorPlan(s.ID, floorID, planImage(t, 320, 240, false)); err != nil {
		t.Fatalf("SetFloorPlan: %v", err)
	}
	if err := mgr.CalibrateFloorPlan(s.ID, floorID, survey.Position{X: 0, Y: 0},
		survey.Position{X: 200, Y: 0}, 10); err != nil {
		t.Fatalf("CalibrateFloorPlan: %v", err)
	}
	if err := mgr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened := mustManager(t, dir, nil, nil, nil, nil)
	if err := reopened.LoadSurveys(); err != nil {
		t.Fatalf("LoadSurveys: %v", err)
	}
	after, _ := reopened.GetSurvey(s.ID)
	plan := after.GetFloorByID(floorID).FloorPlan
	if plan == nil {
		t.Fatal("the plan did not survive the reload")
	}
	if plan.Width != 320 || plan.ScaleM != 0.05 {
		t.Errorf("plan after reload = %dx%d at %v m/px, want 320x240 at 0.05",
			plan.Width, plan.Height, plan.ScaleM)
	}
}

var _ = context.Background
