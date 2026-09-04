// SPDX-License-Identifier: BUSL-1.1

package survey

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"math"
	"time"

	// Registered for their decoders: SetFloorPlan reads a plan's real
	// dimensions out of the image rather than trusting what an uploader says
	// they are.
	_ "image/jpeg"
	_ "image/png"
)

// ErrNoFloorPlan means the floor has no plan, so there is nothing for a
// calibration's two points to be points on.
var ErrNoFloorPlan = errors.New("survey: floor has no plan")

// maxFloorPlanBytes bounds one upload. A building plan exported from a CAD tool
// runs to a few megabytes; a hundred times that is not a plan, and the image is
// held in memory to decode and stored base64-encoded, so the cap is what keeps
// one bad upload from being the daemon's memory ceiling.
const maxFloorPlanBytes = 32 << 20

// SetFloorPlan stores an image as a floor's plan.
//
// The dimensions come from decoding the image, not from the caller. Every
// stored point's position is in this pixel space — a walk, an import, a
// heatmap — so a claimed size that did not match the file would put the whole
// survey in the wrong place, silently and only visibly once someone looked at
// a map.
//
// The scale is not touched. A new plan of the same building at a different
// resolution needs recalibrating, and quietly carrying the old metres-per-pixel
// across would report distances that are wrong by exactly the ratio nobody
// noticed.
func (m *Manager) SetFloorPlan(surveyID, floorID string, imageData []byte) error {
	if len(imageData) == 0 {
		return errors.New("survey: floor plan is empty")
	}
	if len(imageData) > maxFloorPlanBytes {
		return fmt.Errorf("survey: floor plan is %d bytes, over the %d-byte limit",
			len(imageData), maxFloorPlanBytes)
	}

	config, format, err := image.DecodeConfig(bytes.NewReader(imageData))
	if err != nil {
		// Stored anyway, it would be served to a browser that renders nothing:
		// a plan that silently is not there.
		return fmt.Errorf("survey: floor plan is not a PNG or JPEG image: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 {
		return fmt.Errorf("survey: %s floor plan has no dimensions", format)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	s, floor, err := m.floorOf(surveyID, floorID)
	if err != nil {
		return err
	}

	scale := 0.0
	if floor.FloorPlan != nil {
		scale = floor.FloorPlan.ScaleM
	}
	if floor.FloorPlan != nil && (floor.FloorPlan.Width != config.Width ||
		floor.FloorPlan.Height != config.Height) {
		// A different-sized plan is a different pixel space, and the old scale
		// does not describe it.
		scale = 0
	}

	floor.FloorPlan = &FloorPlan{
		ImageData: base64.StdEncoding.EncodeToString(imageData),
		Width:     config.Width,
		Height:    config.Height,
		ScaleM:    scale,
	}
	floor.UpdatedAt = time.Now()
	s.UpdatedAt = floor.UpdatedAt
	return m.persistSurvey(s)
}

// CalibrateFloorPlan sets how many metres a pixel of the plan is, from two
// points the operator marked and the real distance between them.
//
// Two points and a tape measure is the only calibration that needs nothing but
// the operator: a plan carries no scale of its own, and one exported at an
// arbitrary resolution has no relationship to the building until somebody says
// what one of its lines is.
//
// The distance is the straight line between the points, diagonals included: an
// operator marking a wall that runs across the plan means the length of the
// wall, not of its shadow on an axis.
func (m *Manager) CalibrateFloorPlan(
	surveyID, floorID string,
	from, to Position,
	metres float64,
) error {
	if metres <= 0 {
		return fmt.Errorf("survey: calibration distance %v must be positive", metres)
	}

	pixels := math.Hypot(float64(to.X-from.X), float64(to.Y-from.Y))
	if pixels == 0 {
		return errors.New("survey: calibration needs two different points")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	s, floor, err := m.floorOf(surveyID, floorID)
	if err != nil {
		return err
	}
	if floor.FloorPlan == nil {
		return fmt.Errorf("%w: %s", ErrNoFloorPlan, floorID)
	}

	floor.FloorPlan.ScaleM = metres / pixels
	floor.UpdatedAt = time.Now()
	s.UpdatedAt = floor.UpdatedAt
	return m.persistSurvey(s)
}

// floorOf resolves a survey and one of its floors. The caller holds m.mu.
func (m *Manager) floorOf(surveyID, floorID string) (*Survey, *Floor, error) {
	s, exists := m.surveys[surveyID]
	if !exists {
		return nil, nil, fmt.Errorf("%w: %s", ErrSurveyNotFound, surveyID)
	}
	floor := s.GetFloorByID(floorID)
	if floor == nil {
		return nil, nil, fmt.Errorf("%w: %s", ErrFloorNotFound, floorID)
	}
	return s, floor, nil
}
