// SPDX-License-Identifier: BUSL-1.1

package survey_test

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
)

// reopen loads dir's store into a fresh manager and returns the survey from
// it: what a daemon restart would show.
func reopen(t *testing.T, dir, id string) *survey.Survey {
	t.Helper()
	m := mustManager(t, dir, nil, nil, nil, nil)
	if err := m.LoadSurveys(); err != nil {
		t.Fatalf("LoadSurveys: %v", err)
	}
	s, err := m.GetSurvey(id)
	if err != nil {
		t.Fatalf("GetSurvey after reopen: %v", err)
	}
	return s
}

// An imported placement has to survive a restart. Until #821 none did: the
// importers set them, and no table held them.
func TestImportedPlacementsSurviveARestart(t *testing.T) {
	dir := t.TempDir()
	mgr := mustManager(t, dir, nil, nil, nil, nil)

	amp := buildPlacementAMP(t, map[string]any{
		"fileName":          "DIA Main Hall",
		"floorPlanFilename": "DIAMainHall.jpg",
		"floorPlanScalePpf": 6.673,
		"apLocations": []map[string]any{
			{"x": 918.0, "y": 3227.4, "label": "RuckusWi:58b633-c652d9"},
		},
	})
	imported, err := mgr.ImportAirMapper("DIA Main Hall", amp)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	stored := reopen(t, dir, imported.ID)
	if len(stored.Floors) != 1 {
		t.Fatalf("floors = %d, want 1", len(stored.Floors))
	}
	floor := stored.Floors[0]
	if floor.ID != imported.Floors[0].ID {
		t.Fatalf("floor %s after reopen, want %s", floor.ID, imported.Floors[0].ID)
	}
	if len(floor.APLocations) != 1 {
		t.Fatalf("AP placements after reopen = %d, want 1", len(floor.APLocations))
	}
	if got, want := floor.APLocations[0], imported.Floors[0].APLocations[0]; got.ID != want.ID ||
		got.X != want.X || got.Y != want.Y || got.BSSID != want.BSSID || !got.Imported {
		t.Errorf("placement after reopen = %+v, want %+v", got, want)
	}
	if floor.APLocations[0].TxPowerDBm != nil {
		t.Errorf("power = %v from an archive that states none", *floor.APLocations[0].TxPowerDBm)
	}
}

// AirMagnet records each radio's channel and power, so its placements carry a
// band and a transmit power, and both survive the store.
func TestAirMagnetPlacementsKeepPowerAndBand(t *testing.T) {
	dir := t.TempDir()
	mgr := mustManager(t, dir, nil, nil, nil, nil)

	imported, err := mgr.ImportAirMagnet("QA floor", utf16le(t, svdWithSections(t)))
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	aps := reopen(t, dir, imported.ID).Floors[0].APLocations
	if len(aps) != 2 {
		t.Fatalf("AP placements after reopen = %d, want 2", len(aps))
	}
	for _, ap := range aps {
		// The fixture's radios are on channels 149 and 157 at 100 mW.
		if ap.Band != "5 GHz" {
			t.Errorf("%s band = %q, want 5 GHz", ap.BSSID, ap.Band)
		}
		if ap.TxPowerDBm == nil || *ap.TxPowerDBm != 20 {
			t.Errorf("%s power = %v, want 20 dBm", ap.BSSID, ap.TxPowerDBm)
		}
	}
}

// Placements belong to a floor. Each floor of a two-storey survey keeps its
// own, and deleting a floor deletes its placements rather than leaving rows
// that name a floor that no longer exists.
func TestPlacementsStayOnTheirFloor(t *testing.T) {
	dir := t.TempDir()
	mgr := mustManager(t, dir, nil, nil, nil, nil)

	s, err := mgr.CreateSurvey("Two storeys", "", "en0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	ground := s.Floors[0]
	upstairs, err := mgr.AddFloor(s.ID, "First", 2)
	if err != nil {
		t.Fatalf("AddFloor: %v", err)
	}
	power := 17.0
	for _, place := range []struct {
		floor string
		ap    survey.APLocation
		phone survey.ClientLocation
	}{
		{ground.ID,
			survey.APLocation{ID: "ap-ground", X: 10, Y: 20, BSSID: "aa:aa:aa:aa:aa:01"},
			survey.ClientLocation{ID: "phone-ground", X: 5, Y: 6, MAC: "cc:cc:cc:cc:cc:01"}},
		{upstairs.ID,
			survey.APLocation{ID: "ap-first", X: 30, Y: 40, BSSID: "aa:aa:aa:aa:aa:02",
				TxPowerDBm: &power, Band: "6 GHz"},
			survey.ClientLocation{ID: "phone-first", X: 7, Y: 8, MAC: "cc:cc:cc:cc:cc:02"}},
	} {
		if err := mgr.UpdateImportedData(s.ID, survey.ImportedDataUpdate{
			FloorID:         place.floor,
			APLocations:     []survey.APLocation{place.ap},
			ClientLocations: []survey.ClientLocation{place.phone},
		}); err != nil {
			t.Fatalf("UpdateImportedData %s: %v", place.floor, err)
		}
	}

	stored := reopen(t, dir, s.ID)
	want := map[string]struct{ ap, phone string }{
		ground.ID:   {"ap-ground", "phone-ground"},
		upstairs.ID: {"ap-first", "phone-first"},
	}
	for _, f := range stored.Floors {
		if len(f.APLocations) != 1 || f.APLocations[0].ID != want[f.ID].ap {
			t.Errorf("floor %s APs = %+v, want only %s", f.Name, f.APLocations, want[f.ID].ap)
		}
		if len(f.ClientLocations) != 1 || f.ClientLocations[0].ID != want[f.ID].phone {
			t.Errorf("floor %s clients = %+v, want only %s", f.Name, f.ClientLocations, want[f.ID].phone)
		}
	}
	if first := stored.GetFloorByID(upstairs.ID); first.APLocations[0].TxPowerDBm == nil ||
		*first.APLocations[0].TxPowerDBm != power || first.APLocations[0].Band != "6 GHz" {
		t.Errorf("first-floor AP = %+v, want 17 dBm on 6 GHz", first.APLocations[0])
	}

	if err := mgr.DeleteFloor(s.ID, upstairs.ID); err != nil {
		t.Fatalf("DeleteFloor: %v", err)
	}
	if got := placementRows(t, dir); got != 2 {
		t.Errorf("placement rows after deleting a floor = %d, want 2 (the ground floor's)", got)
	}
}

// A placement is a pixel on one plan, so an update that names no floor, or a
// floor this survey does not have, is refused rather than guessed at.
func TestPlacementsNeedAFloor(t *testing.T) {
	mgr := mustManager(t, t.TempDir(), nil, nil, nil, nil)
	s, err := mgr.CreateSurvey("One storey", "", "en0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	for _, floorID := range []string{"", "no-such-floor"} {
		err := mgr.UpdateImportedData(s.ID, survey.ImportedDataUpdate{
			FloorID:     floorID,
			APLocations: []survey.APLocation{{ID: "ap", X: 1, Y: 1}},
		})
		if !errors.Is(err, survey.ErrFloorNotFound) {
			t.Errorf("floor %q: err = %v, want ErrFloorNotFound", floorID, err)
		}
	}
}

// placementRows counts every stored placement, attached to a floor or not, by
// reading the database the manager wrote.
func placementRows(t *testing.T, dir string) int {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(dir, "surveys.db")+"?mode=ro")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = db.Close() }()
	var n int
	if err := db.QueryRowContext(t.Context(), `
		SELECT (SELECT COUNT(*) FROM ap_placements) + (SELECT COUNT(*) FROM client_placements)`).
		Scan(&n); err != nil {
		t.Fatalf("count placements: %v", err)
	}
	return n
}

// Dragging an imported AP to where it really hangs moves it; it is still the
// AP the importer recorded, and the BSSID is what calibration joins on.
func TestUpdateAPPlacementKeepsWhatTheImporterRecorded(t *testing.T) {
	mgr := mustManager(t, t.TempDir(), nil, nil, nil, nil)
	s, err := mgr.CreateSurvey("Imported", "", "en0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	floorID := s.Floors[0].ID
	if err := mgr.SetFloorPlan(s.ID, floorID, planPNG(t, 200, 100)); err != nil {
		t.Fatalf("SetFloorPlan: %v", err)
	}
	imported := survey.APLocation{
		ID: "ap-1", X: 5, Y: 5, Label: "AP-1", BSSID: "aa:bb:cc:00:00:01",
		Vendor: "Ruckus", Notes: "ceiling", Imported: true,
	}
	if err := mgr.UpdateImportedData(s.ID, survey.ImportedDataUpdate{
		FloorID: floorID, APLocations: []survey.APLocation{imported},
	}); err != nil {
		t.Fatalf("UpdateImportedData: %v", err)
	}

	power := 12.0
	got, err := mgr.UpdateAPPlacement(s.ID, floorID, "ap-1", survey.APPlacementEdit{
		X: 150, Y: 80, Label: "AP-1 lobby", TxPowerDBm: &power, Band: "5 GHz",
	})
	if err != nil {
		t.Fatalf("UpdateAPPlacement: %v", err)
	}
	if got.X != 150 || got.Y != 80 || got.Label != "AP-1 lobby" || *got.TxPowerDBm != 12 || got.Band != "5 GHz" {
		t.Errorf("edited fields = %+v, want the edit applied", got)
	}
	if got.BSSID != imported.BSSID || got.Vendor != imported.Vendor ||
		got.Notes != imported.Notes || !got.Imported {
		t.Errorf("importer fields = %+v, want BSSID, vendor, notes and imported kept", got)
	}
	if _, err := mgr.UpdateAPPlacement(s.ID, floorID, "ap-2", survey.APPlacementEdit{}); !errors.Is(err, survey.ErrPlacementNotFound) {
		t.Errorf("updating an unknown placement = %v, want ErrPlacementNotFound", err)
	}
}
