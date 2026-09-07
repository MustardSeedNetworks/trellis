// SPDX-License-Identifier: BUSL-1.1

package survey_test

// floorplan_replace_test.go pins what happens to measured points when the plan
// underneath them is replaced.
//
// A sample's position is a pixel coordinate on the plan it was taken against.
// Swap in an image of different dimensions and every stored point means
// somewhere else in the building, while the map still looks right — the worst
// shape a survey defect can take. Reprojecting them would be a guess: a
// replacement plan is rarely a pure scale of the old one.

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

func planPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{255, 255, 255, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func surveyWithPlanAndSamples(t *testing.T, w, h, samples int) (*survey.Manager, string, string) {
	t.Helper()
	mgr := mustManager(t, t.TempDir(), nil, nil, nil, nil)
	svy, err := mgr.CreateSurvey("Replace", "plan replacement", "wlan0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	floor := svy.GetActiveFloor()
	if floor == nil {
		t.Fatal("no active floor")
	}
	if err := mgr.SetFloorPlan(svy.ID, floor.ID, planPNG(t, w, h)); err != nil {
		t.Fatalf("SetFloorPlan: %v", err)
	}
	if err := mgr.CalibrateFloorPlan(svy.ID, floor.ID,
		survey.Position{X: 0, Y: 0}, survey.Position{X: 100, Y: 0}, 10); err != nil {
		t.Fatalf("CalibrateFloorPlan: %v", err)
	}
	if err := mgr.StartSurvey(svy.ID); err != nil {
		t.Fatalf("StartSurvey: %v", err)
	}
	for i := range samples {
		if err := mgr.AddSample(svy.ID, 10+i*10, 20+i*10, &survey.PassiveSample{
			Networks: []*wifi.ScannedNetwork{{
				SSID: "ap", BSSID: "00:00:00:00:00:01", Signal: -50 - i,
				Channel: 36, Frequency: 5180,
			}},
		}); err != nil {
			t.Fatalf("AddSample %d: %v", i, err)
		}
	}
	return mgr, svy.ID, floor.ID
}

func TestReplacingAPlanUnderSamplesIsRefused(t *testing.T) {
	mgr, surveyID, floorID := surveyWithPlanAndSamples(t, 800, 600, 3)

	err := mgr.SetFloorPlan(surveyID, floorID, planPNG(t, 1024, 768))
	if !errors.Is(err, survey.ErrPlanWouldStrandSamples) {
		t.Fatalf("SetFloorPlan with a different size = %v, want ErrPlanWouldStrandSamples", err)
	}
	// The operator needs to know how much is at stake, not just that it was
	// refused: three points is a decision, three hundred is a different one.
	if got := err.Error(); !bytes.Contains([]byte(got), []byte("3")) {
		t.Errorf("error %q does not say how many samples are on the floor", got)
	}

	svy, err := mgr.GetSurvey(surveyID)
	if err != nil {
		t.Fatal(err)
	}
	plan := svy.GetActiveFloor().FloorPlan
	if plan.Width != 800 || plan.Height != 600 {
		t.Errorf("plan is now %dx%d — the refusal did not hold", plan.Width, plan.Height)
	}
	if plan.ScaleM == 0 {
		t.Error("the scale was cleared by a replacement that did not happen")
	}
}

func TestReplacingAPlanWithTheSameSizeKeepsItsScale(t *testing.T) {
	mgr, surveyID, floorID := surveyWithPlanAndSamples(t, 800, 600, 3)
	before, err := mgr.GetSurvey(surveyID)
	if err != nil {
		t.Fatal(err)
	}
	scale := before.GetActiveFloor().FloorPlan.ScaleM

	// A redrawn plan at the same resolution describes the same pixel space, so
	// the points still mean what they meant and the scale still holds.
	if err := mgr.SetFloorPlan(surveyID, floorID, planPNG(t, 800, 600)); err != nil {
		t.Fatalf("same-size replacement refused: %v", err)
	}
	after, err := mgr.GetSurvey(surveyID)
	if err != nil {
		t.Fatal(err)
	}
	if got := after.GetActiveFloor().FloorPlan.ScaleM; got != scale {
		t.Errorf("scale = %v after a same-size replacement, want %v", got, scale)
	}
}

func TestReplacingAPlanOnAnEmptyFloorIsAllowed(t *testing.T) {
	// Nothing is stranded when nothing has been measured, and this is the
	// ordinary case: upload a plan, look at it, upload a better one.
	mgr, surveyID, floorID := surveyWithPlanAndSamples(t, 800, 600, 0)

	if err := mgr.SetFloorPlan(surveyID, floorID, planPNG(t, 1024, 768)); err != nil {
		t.Fatalf("replacement on an empty floor refused: %v", err)
	}
	svy, err := mgr.GetSurvey(surveyID)
	if err != nil {
		t.Fatal(err)
	}
	plan := svy.GetActiveFloor().FloorPlan
	if plan.Width != 1024 || plan.Height != 768 {
		t.Errorf("plan is %dx%d, want the replacement", plan.Width, plan.Height)
	}
	if plan.ScaleM != 0 {
		t.Errorf("scale = %v — a new pixel space has no scale until it is calibrated", plan.ScaleM)
	}
}
