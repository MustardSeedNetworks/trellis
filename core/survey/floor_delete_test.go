// SPDX-License-Identifier: BUSL-1.1

package survey_test

import (
	"errors"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// passive is one reading, enough to make a floor hold a measurement.
func passive(signal int) *survey.PassiveSample {
	return &survey.PassiveSample{
		Networks: []*wifi.ScannedNetwork{
			{SSID: "msn", BSSID: "aa:bb:cc:dd:ee:ff", Signal: signal, Channel: 36, SNR: 30},
		},
	}
}

// A floor holds measurements, and deleting it has to take them with it — both
// out of the survey a caller reads back and out of the database. The store
// rewrites a survey's floors as a whole, so a point left behind would be a
// row with no floor rather than a visible sample; a reopened manager is what
// proves it is gone.
func TestDeletingAFloorTakesItsMeasurements(t *testing.T) {
	dir := t.TempDir()
	m := mustManager(t, dir, nil, nil, nil, nil)

	s, err := m.CreateSurvey("Two storeys", "", "en0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	if err := m.StartSurvey(s.ID); err != nil {
		t.Fatalf("StartSurvey: %v", err)
	}
	upstairs, err := m.AddFloor(s.ID, "First", 1)
	if err != nil {
		t.Fatalf("AddFloor: %v", err)
	}
	if err := m.AddSampleToFloor(s.ID, s.Floors[0].ID, 10, 10, passive(-55)); err != nil {
		t.Fatalf("AddSampleToFloor ground: %v", err)
	}
	if err := m.AddSampleToFloor(s.ID, upstairs.ID, 20, 20, passive(-70)); err != nil {
		t.Fatalf("AddSampleToFloor first: %v", err)
	}

	if err := m.DeleteFloor(s.ID, upstairs.ID); err != nil {
		t.Fatalf("DeleteFloor: %v", err)
	}

	reopened := mustManager(t, dir, nil, nil, nil, nil)
	if err := reopened.LoadSurveys(); err != nil {
		t.Fatalf("LoadSurveys: %v", err)
	}
	stored, err := reopened.GetSurvey(s.ID)
	if err != nil {
		t.Fatalf("GetSurvey after reopen: %v", err)
	}
	if got := len(stored.Floors); got != 1 {
		t.Fatalf("floors after delete = %d, want 1", got)
	}
	if got := len(stored.GetAllMeasuredSamples()); got != 1 {
		t.Errorf("measurements after delete = %d, want 1: the deleted floor's reading survived", got)
	}
}

// The last floor cannot be deleted, and the refusal has to be classifiable:
// the API layer maps it onto a code, and matching on message text is what a
// sentinel exists to avoid.
func TestDeletingTheLastFloorIsRefusedWithATypedError(t *testing.T) {
	m := mustManager(t, t.TempDir(), nil, nil, nil, nil)

	s, err := m.CreateSurvey("One storey", "", "en0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}

	err = m.DeleteFloor(s.ID, s.Floors[0].ID)
	if !errors.Is(err, survey.ErrLastFloor) {
		t.Errorf("DeleteFloor of the last floor = %v, want ErrLastFloor", err)
	}
}

// A floor that does not exist is not found, whatever the survey's floor count.
// Checking the count first answers "cannot delete the last floor" about a
// floor the caller never named.
func TestDeletingAnUnknownFloorOfAOneFloorSurveyIsNotFound(t *testing.T) {
	m := mustManager(t, t.TempDir(), nil, nil, nil, nil)

	s, err := m.CreateSurvey("One storey", "", "en0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}

	err = m.DeleteFloor(s.ID, "no-such-floor")
	if !errors.Is(err, survey.ErrFloorNotFound) {
		t.Errorf("DeleteFloor of an unknown floor = %v, want ErrFloorNotFound", err)
	}
}

// A rename must not silently accept a blank name: the rail lists floors by
// name and a blank row is one nothing can pick out.
func TestRenamingAFloorToNothingIsRefused(t *testing.T) {
	m := mustManager(t, t.TempDir(), nil, nil, nil, nil)

	s, err := m.CreateSurvey("One storey", "", "en0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}

	if err := m.UpdateFloor(s.ID, s.Floors[0].ID, "  ", 0); !errors.Is(err, survey.ErrFloorNameEmpty) {
		t.Errorf("UpdateFloor with a blank name = %v, want ErrFloorNameEmpty", err)
	}
	floor, err := m.GetFloor(s.ID, s.Floors[0].ID)
	if err != nil {
		t.Fatalf("GetFloor: %v", err)
	}
	if floor.Name == "  " {
		t.Error("the blank name was stored anyway")
	}
}
