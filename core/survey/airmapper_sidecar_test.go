// SPDX-License-Identifier: BUSL-1.1

package survey_test

// One archive in the reference corpus — Link-Live_TouchTelSurvey2020_2021-01-15
// — carries someone else's `.serial`: the sidecar of the Large Retail Example
// survey, declaring 64 points, a Walmart floor plan and that plan's
// pixels-per-foot, packed alongside a 39-point TouchTel walk and its own plan
// image. Trusting it applies another building's scale, propagation radius and
// AP placements to this floor.
//
// The sidecar states the plan it belongs to: floorPlanWidthPx and
// floorPlanHeightPx. The archive carries the plan itself. When the two
// disagree the sidecar is not describing this archive, and the archive is read
// as the serial-less archives of #335 are — measurements and plan, no
// calibration — rather than with a stranger's numbers.

import (
	"strings"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/survey"
)

func TestSerialForADifferentPlanIsRefused(t *testing.T) {
	// buildPlacementAMP writes a 40x30 plan.
	data := buildPlacementAMP(t, map[string]any{
		"fileName":          "Large Retail Example",
		"floorPlanWidthPx":  1936,
		"floorPlanHeightPx": 1112,
		"floorPlanScalePpf": 1.9000883102416992,
		"surveyPointCount":  64,
		"propagation":       30.0,
		"propagationUnit":   "ft",
		"apLocations": []map[string]any{
			{"x": 10.0, "y": 10.0, "label": "BelkinIn:58ef68-09f907"},
		},
	})

	file, err := survey.ParseAirMapperFile(data)
	if err != nil {
		t.Fatalf("ParseAirMapperFile: %v", err)
	}
	result, err := file.ToImportResult()
	if err != nil {
		t.Fatalf("ToImportResult: %v", err)
	}

	if result.Calibration.ScaleM != 0 {
		t.Errorf("scale %g m/px came from a sidecar describing a 1936x1112 plan, "+
			"but this archive's plan is 40x30", result.Calibration.ScaleM)
	}
	if len(result.APLocations) != 0 {
		t.Errorf("%d AP placement(s) taken from a sidecar that describes another plan",
			len(result.APLocations))
	}
	if result.SurveyPointCount != 0 {
		t.Errorf("survey point count %d taken from a sidecar that describes another plan",
			result.SurveyPointCount)
	}
	if !hasWarningAbout(result.Warnings, "does not describe this archive") {
		t.Errorf("no warning naming the mismatch; warnings were %q", result.Warnings)
	}
}

func TestSerialForTheArchivesOwnPlanIsUsed(t *testing.T) {
	data := buildPlacementAMP(t, map[string]any{
		"fileName":          "DIA Main Hall",
		"floorPlanWidthPx":  40,
		"floorPlanHeightPx": 30,
		"floorPlanScalePpf": 2.0,
		"surveyPointCount":  36,
		"apLocations": []map[string]any{
			{"x": 10.0, "y": 10.0, "label": "BelkinIn:58ef68-09f907"},
		},
	})

	file, err := survey.ParseAirMapperFile(data)
	if err != nil {
		t.Fatalf("ParseAirMapperFile: %v", err)
	}
	result, err := file.ToImportResult()
	if err != nil {
		t.Fatalf("ToImportResult: %v", err)
	}
	if result.Calibration.ScaleM == 0 {
		t.Error("a sidecar that matches the archive's own plan was refused")
	}
	if len(result.APLocations) != 1 {
		t.Errorf("got %d placements, want 1", len(result.APLocations))
	}
	if result.SurveyPointCount != 36 {
		t.Errorf("got survey point count %d, want 36", result.SurveyPointCount)
	}
}

// A sidecar that states no plan dimensions cannot be checked this way, and is
// trusted as before: the check exists to catch a sidecar that names a
// different plan, not to demand a field.
func TestSerialWithNoDeclaredPlanSizeIsStillUsed(t *testing.T) {
	data := buildPlacementAMP(t, map[string]any{
		"fileName":          "No dimensions",
		"floorPlanScalePpf": 2.0,
		"surveyPointCount":  12,
	})

	file, err := survey.ParseAirMapperFile(data)
	if err != nil {
		t.Fatalf("ParseAirMapperFile: %v", err)
	}
	result, err := file.ToImportResult()
	if err != nil {
		t.Fatalf("ToImportResult: %v", err)
	}
	if result.Calibration.ScaleM == 0 {
		t.Error("a sidecar with no declared plan size was refused")
	}
}

func hasWarningAbout(warnings []string, substr string) bool {
	for _, w := range warnings {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}
