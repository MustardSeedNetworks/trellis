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

	// The measurements. Until this landed, an import produced a floor plan
	// with nothing on it: the parser read the .serial metadata and the image
	// and dropped every observation, which for the reference corpus meant
	// discarding 113,551 of them across twelve files.
	if err := m.importMeasurements(svy.ID, ampFile.SurveyResult); err != nil {
		return nil, err
	}

	return m.GetSurvey(svy.ID)
}

// importMeasurements decodes the .SurveyResult member and records each walk
// position with everything observed from it.
//
// An archive with no measurement member is not an error: AirMapper writes one
// only once a walk has produced data, so a plan-only export is a legitimate
// thing to import. The survey simply arrives empty.
func (m *Manager) importMeasurements(surveyID string, payload []byte) error {
	if len(payload) == 0 {
		return nil
	}
	points, err := ParseSurveyResult(payload)
	if err != nil {
		return fmt.Errorf("parse survey measurements: %w", err)
	}
	if len(points) == 0 {
		return nil
	}

	// AddSample requires a survey in progress; an import is a completed walk,
	// so it is moved through that state rather than around it — the status
	// machine stays the single description of what a survey is doing.
	if err := m.StartSurvey(surveyID); err != nil {
		return fmt.Errorf("open survey for import: %w", err)
	}
	for _, p := range points {
		sample := &PassiveSample{Networks: p.Networks}
		sample.CalculateAggregations()
		if err := m.AddSample(surveyID, p.X, p.Y, sample); err != nil {
			return fmt.Errorf("record imported point (%d,%d): %w", p.X, p.Y, err)
		}
	}
	if err := m.CompleteSurvey(surveyID); err != nil {
		return fmt.Errorf("close imported survey: %w", err)
	}
	return nil
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
