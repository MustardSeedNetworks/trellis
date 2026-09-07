// SPDX-License-Identifier: BUSL-1.1

package survey_test

// AirMapper writes its AP placements at the top level of the `.serial`
// sidecar, under `apLocations`. The parser read them from a nested
// `locations.aps` member that no archive in the reference corpus carries, so
// every placement was dropped in silence: 216 of them across 48 archives, and
// the ground truth Gate G1 went looking for and reported as absent.
//
// The oracle is NetAlly's own decode of the same archives, served by Link-Live
// (docs/12-CROSS-PRODUCT-ORACLE.md) — for the DIA Main Hall survey it lists 39
// grouped AP placements where Trellis imported none.

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

// buildPlacementAMP writes an archive whose `.serial` carries placements in
// AirMapper's real shape: floats for the position, and a label that is either
// the vendor-prefixed address AirMapper writes for a single BSS
// ("BelkinIn:58ef68-09f907") or a bare AP name when placements are grouped.
func buildPlacementAMP(t *testing.T, serial map[string]any) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 30))
	img.Set(0, 0, color.RGBA{255, 255, 255, 255})
	var plan bytes.Buffer
	if err := png.Encode(&plan, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	body, err := json.Marshal(serial)
	if err != nil {
		t.Fatalf("marshal serial: %v", err)
	}

	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	write := func(name string, data []byte) {
		f, createErr := zw.Create(name)
		if createErr != nil {
			t.Fatalf("zip create %s: %v", name, createErr)
		}
		if _, writeErr := f.Write(data); writeErr != nil {
			t.Fatalf("zip write %s: %v", name, writeErr)
		}
	}
	write("1920020.serial", body)
	write("DIAMainHall.jpg", plan.Bytes())
	if closeErr := zw.Close(); closeErr != nil {
		t.Fatalf("zip close: %v", closeErr)
	}
	return archive.Bytes()
}

func TestParseAirMapperReadsTopLevelAPPlacements(t *testing.T) {
	amp := buildPlacementAMP(t, map[string]any{
		"fileName":          "DIA Main Hall",
		"floorPlanFilename": "DIAMainHall.jpg",
		"floorPlanScalePpf": 6.673,
		"surveyPointCount":  36,
		"apLocations": []map[string]any{
			{"x": 918.0, "y": 3227.4047781973954, "label": "BelkinIn:58ef68-09f907"},
			{"x": 1328.9358342094545, "y": 3844.3482054031406, "label": "HT_LV5_W_Plaza_"},
		},
	})

	file, err := survey.ParseAirMapperFile(amp)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := file.ToImportResult()
	if err != nil {
		t.Fatalf("to import result: %v", err)
	}

	if len(result.APLocations) != 2 {
		t.Fatalf("AP placements = %d, want 2 — the archive states them under apLocations", len(result.APLocations))
	}

	addressed := result.APLocations[0]
	if addressed.BSSID != "58:ef:68:09:f9:07" {
		t.Errorf("BSSID = %q, want 58:ef:68:09:f9:07 read out of the label", addressed.BSSID)
	}
	if addressed.Vendor != "BelkinIn" {
		t.Errorf("vendor = %q, want BelkinIn", addressed.Vendor)
	}
	if addressed.X != 918 || addressed.Y != 3227.4047781973954 {
		t.Errorf("position = (%v,%v), want the archive's floats unrounded", addressed.X, addressed.Y)
	}

	// A grouped placement is named, not addressed. Inventing a BSSID for it
	// would join measurements to the wrong radio.
	named := result.APLocations[1]
	if named.BSSID != "" {
		t.Errorf("BSSID = %q for a placement labelled with an AP name, want none", named.BSSID)
	}
	if named.Label != "HT_LV5_W_Plaza_" {
		t.Errorf("label = %q, want the name kept verbatim", named.Label)
	}
}

func TestImportAirMapperLandsPlacementsOnTheSurvey(t *testing.T) {
	mgr := mustManager(t, t.TempDir(), nil, nil, nil, nil)

	amp := buildPlacementAMP(t, map[string]any{
		"fileName":          "DIA Main Hall",
		"floorPlanFilename": "DIAMainHall.jpg",
		"floorPlanScalePpf": 6.673,
		"apLocations": []map[string]any{
			{"x": 918.0, "y": 3227.4, "label": "RuckusWi:58b633-c652d9"},
		},
	})

	svy, err := mgr.ImportAirMapper("DIA Main Hall", amp)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(svy.APLocations) != 1 {
		t.Fatalf("stored AP placements = %d, want 1", len(svy.APLocations))
	}
	if got := svy.APLocations[0].BSSID; got != "58:b6:33:c6:52:d9" {
		t.Errorf("stored BSSID = %q, want 58:b6:33:c6:52:d9", got)
	}
	if !svy.APLocations[0].Imported {
		t.Error("placement is not marked imported")
	}
}
