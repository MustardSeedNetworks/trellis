// SPDX-License-Identifier: BUSL-1.1

package survey_test

// airmapper_linklive_test.go covers the archive shape a customer most often
// has: the one Link-Live hands back, which carries the survey and the plan and
// no .serial member at all. Eight of the 58 archives in the reference corpus
// are this shape, and every one of them was refused outright.

import (
	"archive/zip"
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// buildLinkLiveAMP writes the three members a Link-Live export has, and no
// fourth: no .serial, so no placements, no pass/fail criteria and no scale.
func buildLinkLiveAMP(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{255, 255, 255, 255})
	var plan bytes.Buffer
	if err := png.Encode(&plan, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	write := func(name string, data []byte) {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := f.Write(data); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	write("EVTOffice.jpg", plan.Bytes())
	write("EVTOffice.acsx", []byte("<acsx/>"))
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return archive.Bytes()
}

func TestImportAirMapperWithoutSerialMetadata(t *testing.T) {
	mgr := mustManager(t, t.TempDir(), nil, nil, nil, nil)

	svy, err := mgr.ImportAirMapper("EVT office", buildLinkLiveAMP(t, 800, 600))
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	floor := svy.GetActiveFloor()
	if floor == nil || floor.FloorPlan == nil {
		t.Fatal("no floor plan — the archive carries one")
	}
	if floor.FloorPlan.Width != 800 || floor.FloorPlan.Height != 600 {
		t.Errorf("plan is %dx%d, want 800x600", floor.FloorPlan.Width, floor.FloorPlan.Height)
	}

	// The scale is the point. Without .serial the archive says nothing about
	// what a pixel is worth, and an invented default would put metres on every
	// dead-zone radius in the report — a number nobody measured. Unknown has to
	// stay unknown until an operator calibrates the plan.
	if floor.FloorPlan.ScaleM != 0 {
		t.Errorf("scale = %v m/px from an archive that carries no calibration", floor.FloorPlan.ScaleM)
	}
	if len(svy.APLocations) != 0 {
		t.Errorf("AP locations = %d from an archive with no placements", len(svy.APLocations))
	}
}
