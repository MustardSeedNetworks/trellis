// SPDX-License-Identifier: BUSL-1.1

package survey_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
)

// buildAMP synthesizes a minimal but valid AirMapper (.amp) archive: a JSON
// `.serial` metadata file plus a real PNG floor plan of the given size. This is
// what a user's exported AirMapper survey looks like on disk (a zip), so the
// import path is exercised end-to-end, not mocked.
func buildAMP(t *testing.T, w, h int, scalePpf float64, limits []survey.InsitesLimit) []byte {
	t.Helper()

	// Real PNG so image.DecodeConfig recovers the true dimensions.
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{255, 255, 255, 255})
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	serial := survey.SerialMetadata{
		FileName:          "floorplan.png",
		FloorPlanScalePpf: scalePpf, // pixels per foot → drives Calibration.ScaleM
		Propagation:       15,
		PropagationUnit:   "feet",
		SurveyPointCount:  0,
		InsitesLimits:     limits,
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

// TestImportAirMapperRoundTrip proves the full import→store→reopen flow: a real
// .amp archive is imported into a survey (floor plan dimensions recovered from
// the PNG, calibration from the .serial), persisted to disk, and reloaded by a
// fresh Manager with everything intact. The imported survey is then usable by
// the heatmap engine (its dimensions come from the recovered floor plan).
func TestImportAirMapperRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Import needs no live capture — all seams are nil.
	mgr := survey.NewManager(dir, nil, nil, nil, nil)

	const (
		w, h     = 1600, 1200
		scalePpf = 20.0 // 20 px/ft → 0.3048/20 m/px
	)
	amp := buildAMP(t, w, h, scalePpf, []survey.InsitesLimit{
		{Option: "rssi", Name: "Signal", Limit: -70, Suffix: "dBm", Enabled: true, Mode: "passive"},
	})

	svy, err := mgr.ImportAirMapper("Everett HQ", amp)
	if err != nil {
		t.Fatalf("ImportAirMapper: %v", err)
	}

	// --- Survey shape from the import --------------------------------------
	if svy.Name != "Everett HQ" {
		t.Errorf("Name = %q, want Everett HQ", svy.Name)
	}
	floor := svy.GetActiveFloor()
	if floor == nil || floor.FloorPlan == nil {
		t.Fatal("imported survey has no active floor plan")
	}
	if floor.FloorPlan.Width != w || floor.FloorPlan.Height != h {
		t.Errorf("floor plan dims = %dx%d, want %dx%d (recovered from PNG)",
			floor.FloorPlan.Width, floor.FloorPlan.Height, w, h)
	}
	// 1/20 px per foot × 0.3048 m/ft = 0.01524 m/px.
	if got, want := floor.FloorPlan.ScaleM, 0.3048/scalePpf; abs(got-want) > 1e-9 {
		t.Errorf("ScaleM = %.6f, want %.6f (from FloorPlanScalePpf)", got, want)
	}
	if floor.FloorPlan.ImageData == "" {
		t.Error("floor plan image data URL not populated")
	}
	if len(svy.PassFailCriteria) != 1 || svy.PassFailCriteria[0].Option != "rssi" {
		t.Errorf("pass/fail criteria not imported: %+v", svy.PassFailCriteria)
	}
	if len(svy.PassFailCriteria) == 1 && !svy.PassFailCriteria[0].Imported {
		t.Error("imported criterion should be marked Imported")
	}

	// --- Persistence round-trip: a fresh Manager reloads it ----------------
	reopened := survey.NewManager(dir, nil, nil, nil, nil)
	if err := reopened.LoadSurveys(); err != nil {
		t.Fatalf("LoadSurveys: %v", err)
	}
	got, err := reopened.GetSurvey(svy.ID)
	if err != nil {
		t.Fatalf("reopened GetSurvey(%s): %v", svy.ID, err)
	}
	if got.Name != "Everett HQ" {
		t.Errorf("reopened Name = %q, want Everett HQ", got.Name)
	}
	rf := got.GetActiveFloor()
	if rf == nil || rf.FloorPlan == nil || rf.FloorPlan.Width != w || rf.FloorPlan.Height != h {
		t.Errorf("reopened floor plan not intact: %+v", rf)
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
