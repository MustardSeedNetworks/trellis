// SPDX-License-Identifier: BUSL-1.1

package survey_test

import (
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
)

func TestImportAirMagnetStoresAWalkableSurvey(t *testing.T) {
	m := mustManager(t, t.TempDir(), nil, nil, nil, nil)

	svy, err := m.ImportAirMagnet("indoor walk", utf16le(t, svdPassiveExport(t)))
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	if svy.Status != survey.StatusCompleted {
		t.Errorf("status = %s, want completed — an import is a finished walk", svy.Status)
	}
	floor := svy.GetActiveFloor()
	if floor == nil {
		t.Fatal("no active floor")
	}
	if len(floor.Samples) != 2 {
		t.Fatalf("samples = %d, want 2", len(floor.Samples))
	}
	if floor.FloorPlan != nil {
		t.Error("an .svd carries no plan image, so the floor must not claim one")
	}

	// The measurements have to survive the store, not just the parse: a
	// reload is what the report and the heatmap read.
	reloaded, err := m.GetSurvey(svy.ID)
	if err != nil {
		t.Fatal(err)
	}
	first := reloaded.GetActiveFloor().Samples[0]
	if first.X != 246 || first.Y != 43 {
		t.Errorf("first sample at (%d,%d), want (246,43)", first.X, first.Y)
	}
	passive, ok := first.SampleData.(*survey.PassiveSample)
	if !ok || len(passive.Networks) != 2 {
		t.Fatalf("first sample data = %#v", first.SampleData)
	}
	if got := passive.Networks[0].SSID; got != "cisco-3500" {
		t.Errorf("strongest network = %q, want cisco-3500", got)
	}
}
