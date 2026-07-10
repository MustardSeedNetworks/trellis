// SPDX-License-Identifier: BUSL-1.1

package survey

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // register JPEG decoder for DecodeConfig
	_ "image/png"  // register PNG decoder for DecodeConfig
	"math"

	"github.com/google/uuid"
)

// ImportAirMapper parses an AirMapper (.amp) archive, builds a survey from its
// floor plan and imported placements, persists it, and returns it.
//
// This is a single core operation on purpose. In Seed the import was
// orchestrated by the frontend across three API round-trips (parse, then
// UpdateFloorPlan, then UpdateImportedData); pulling it into one testable core
// method means the import logic lives where it can be verified, and Trellis's
// API/UI call one thing instead of re-implementing the choreography.
func (m *Manager) ImportAirMapper(name string, data []byte) (*Survey, error) {
	ampFile, err := ParseAirMapperFile(data)
	if err != nil {
		return nil, fmt.Errorf("parse AirMapper file: %w", err)
	}
	result, err := ampFile.ToImportResult()
	if err != nil {
		return nil, fmt.Errorf("process AirMapper file: %w", err)
	}
	if len(ampFile.FloorPlan) == 0 {
		return nil, errors.New("AirMapper file has no floor plan image")
	}

	// Floor-plan pixel dimensions come from the image itself; the calibration
	// only carries meters-per-pixel. DecodeConfig reads just the header.
	cfg, _, err := image.DecodeConfig(bytes.NewReader(ampFile.FloorPlan))
	if err != nil {
		return nil, fmt.Errorf("decode floor plan image: %w", err)
	}

	svy, err := m.CreateSurvey(name, "Imported from AirMapper", "", TypePassive)
	if err != nil {
		return nil, err
	}

	if err := m.UpdateFloorPlan(svy.ID, &FloorPlan{
		ImageData: result.FloorPlanImage, // base64 data URL, for display
		Width:     cfg.Width,
		Height:    cfg.Height,
		ScaleM:    result.Calibration.ScaleM,
	}); err != nil {
		return nil, err
	}

	if err := m.UpdateImportedData(svy.ID, ImportedDataUpdate{
		APLocations:      importedAPLocations(result.APLocations),
		ClientLocations:  importedClientLocations(result.ClientLocations),
		PassFailCriteria: importedPassFailCriteria(result.PassFailCriteria),
	}); err != nil {
		return nil, err
	}

	return m.GetSurvey(svy.ID)
}

// importedAPLocations maps AirMapper AP placements onto the survey domain type,
// assigning stable IDs and marking them imported. Pixel coordinates are rounded
// from the AirMapper float positions.
func importedAPLocations(in []APLocationData) []APLocation {
	if len(in) == 0 {
		return nil
	}
	out := make([]APLocation, 0, len(in))
	for _, ap := range in {
		out = append(out, APLocation{
			ID:       uuid.New().String(),
			X:        roundToInt(ap.X),
			Y:        roundToInt(ap.Y),
			Label:    ap.Label,
			BSSID:    ap.BSSID,
			Imported: true,
		})
	}
	return out
}

// importedClientLocations maps AirMapper client placements onto the survey type.
func importedClientLocations(in []ClientLocationData) []ClientLocation {
	if len(in) == 0 {
		return nil
	}
	out := make([]ClientLocation, 0, len(in))
	for _, c := range in {
		out = append(out, ClientLocation{
			ID:       uuid.New().String(),
			X:        roundToInt(c.X),
			Y:        roundToInt(c.Y),
			Label:    c.Label,
			MAC:      c.MAC,
			Imported: true,
		})
	}
	return out
}

// importedPassFailCriteria maps AirMapper insites limits onto the survey's
// pass/fail criteria.
func importedPassFailCriteria(in []InsitesLimit) []PassFailCriterion {
	if len(in) == 0 {
		return nil
	}
	out := make([]PassFailCriterion, 0, len(in))
	for _, l := range in {
		out = append(out, PassFailCriterion{
			Option:   l.Option,
			Name:     l.Name,
			Limit:    l.Limit,
			Suffix:   l.Suffix,
			Enabled:  l.Enabled,
			Mode:     l.Mode,
			APCount:  l.AP,
			Imported: true,
		})
	}
	return out
}

func roundToInt(v float64) int {
	return int(math.Round(v))
}
