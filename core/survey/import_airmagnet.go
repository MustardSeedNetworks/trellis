// SPDX-License-Identifier: BUSL-1.1

package survey

import (
	"fmt"
	"math"

	"github.com/google/uuid"
)

// ImportAirMagnet reads an AirMagnet Survey .svd export and stores it as a
// survey, so a customer's existing AirMagnet work analyses and reports here the
// same way a walked survey does.
//
// Unlike an AirMapper archive there is no floor plan to import: AirMagnet keeps
// the plan as a separate file in the project directory (a .jpg or .dwg beside
// the .svd) and the export carries only its extent. The survey therefore
// arrives without a plan and the heatmap is bounded by the measurements
// themselves, which is what the analysis path already does for a walk with no
// plan uploaded yet.
func (m *Manager) ImportAirMagnet(name string, data []byte) (*Survey, error) {
	file, err := ParseAirMagnetSVD(data)
	if err != nil {
		return nil, fmt.Errorf("parse AirMagnet file: %w", err)
	}

	svy, err := m.CreateSurvey(name, airMagnetDescription(file), "", TypePassive)
	if err != nil {
		return nil, err
	}

	if aps := airMagnetAPLocations(file.APs); len(aps) > 0 {
		if err := m.UpdateImportedData(svy.ID, ImportedDataUpdate{
			FloorID:     svy.ActiveFloorID,
			APLocations: aps,
		}); err != nil {
			return nil, err
		}
	}

	if err := m.importAirMagnetPoints(svy.ID, file.Points); err != nil {
		return nil, err
	}
	return m.GetSurvey(svy.ID)
}

// importAirMagnetPoints records each walk position with everything heard from
// it. Like the AirMapper importer it moves the survey through the in-progress
// state rather than around it, so the status machine stays the one description
// of what a survey is doing.
func (m *Manager) importAirMagnetPoints(surveyID string, points []AirMagnetPoint) error {
	if len(points) == 0 {
		// The vendor ships header-only exports; importing one as an empty
		// survey is more useful than refusing a file AirMagnet itself wrote.
		return nil
	}
	if err := m.StartSurvey(surveyID); err != nil {
		return fmt.Errorf("open survey for import: %w", err)
	}
	samples := make([]*SamplePoint, 0, len(points))
	for _, p := range points {
		samples = append(samples, newSamplePoint(p.X, p.Y, &PassiveSample{Networks: p.Networks}))
	}
	if err := m.recordImported(surveyID, samples); err != nil {
		return err
	}
	return m.CompleteSurvey(surveyID)
}

// airMagnetDescription says where the survey came from and what the file
// declared about it, including the extent that has no plan to go with it.
func airMagnetDescription(f *AirMagnetFile) string {
	description := fmt.Sprintf("Imported from AirMagnet Survey %s (%s)",
		f.versionForMessage(), f.Type)
	if f.Width > 0 && f.Height > 0 {
		description += fmt.Sprintf(", declared extent %dx%d, no floor plan in the export",
			f.Width, f.Height)
	}
	return description
}

// airMagnetAPLocations maps the file's AP configuration section onto the
// domain type, the same shape the AirMapper importer produces. AirMagnet also
// records each radio's channel and power, which an AirMapper archive does not.
func airMagnetAPLocations(in []AirMagnetAP) []APLocation {
	if len(in) == 0 {
		return nil
	}
	out := make([]APLocation, 0, len(in))
	for _, ap := range in {
		label := ap.Name
		if label == "" {
			label = ap.SSID
		}
		loc := APLocation{
			ID:       uuid.New().String(),
			X:        ap.X,
			Y:        ap.Y,
			Label:    label,
			BSSID:    ap.BSSID,
			Imported: true,
		}
		if ap.PowerMW > 0 {
			dBm := 10 * math.Log10(float64(ap.PowerMW))
			loc.TxPowerDBm = &dBm
		}
		if ap.Channel > 0 {
			loc.Band = bandLabel(frequencyForChannel(ap.Channel), ap.Channel)
		}
		out = append(out, loc)
	}
	return out
}
