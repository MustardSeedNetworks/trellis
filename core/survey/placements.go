// SPDX-License-Identifier: BUSL-1.1

package survey

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrPlacementNotFound is returned when a floor has no AP placement with
	// the given ID.
	ErrPlacementNotFound = errors.New("AP placement not found")
	// ErrPlacementOffPlan is returned when a placement's position is not a
	// pixel of the floor's plan. A prediction drawn from an AP the operator
	// cannot see on the map is coverage nobody can account for.
	ErrPlacementOffPlan = errors.New("AP placement is outside the floor plan")
	// ErrTxPowerRange is returned for a transmit power no Wi-Fi radio is
	// allowed to emit.
	ErrTxPowerRange = fmt.Errorf("transmit power must be between %v and %v dBm", minTxPowerDBm, maxTxPowerDBm)
)

// The range a placed AP's stated power may take. 36 dBm is the highest EIRP
// any regulatory domain permits a Wi-Fi AP (FCC 4 W point-to-multipoint);
// below -10 dBm no enterprise radio can be configured. A value outside it is
// a typo, and the predicted map would carry it into every cell.
const (
	minTxPowerDBm = -10
	maxTxPowerDBm = 36
)

// APPlacementEdit is what an operator sets on a placed AP: where it sits, what
// it is called, and the power and band the prediction uses.
type APPlacementEdit struct {
	X, Y  int
	Label string
	// TxPowerDBm nil means unstated; the prediction then assumes
	// rf.DefaultTxPowerDBm, and saying so is the operator's choice.
	TxPowerDBm *float64
	// Band is "2.4 GHz", "5 GHz", "6 GHz", or empty for an AP whose band is
	// not stated, which the prediction draws on every band.
	Band string
}

// validate checks the edit against the plan its position is a pixel of.
func (e APPlacementEdit) validate(plan *FloorPlan) error {
	if plan == nil {
		return ErrNoFloorPlan
	}
	if e.X < 0 || e.Y < 0 || e.X >= plan.Width || e.Y >= plan.Height {
		return fmt.Errorf("%w: (%d, %d) on a %dx%d plan",
			ErrPlacementOffPlan, e.X, e.Y, plan.Width, plan.Height)
	}
	if e.Band != "" {
		if _, ok := bandCentreMHz[e.Band]; !ok {
			return fmt.Errorf("%w, not %q", ErrUnknownBand, e.Band)
		}
	}
	if p := e.TxPowerDBm; p != nil && (math.IsNaN(*p) || *p < minTxPowerDBm || *p > maxTxPowerDBm) {
		return fmt.Errorf("%w, not %v", ErrTxPowerRange, *p)
	}
	return nil
}

func (e APPlacementEdit) applyTo(ap *APLocation) {
	ap.X, ap.Y = e.X, e.Y
	ap.Label = e.Label
	ap.TxPowerDBm = e.TxPowerDBm
	ap.Band = e.Band
}

// AddAPPlacement places an AP on a floor by hand and returns it with its new
// ID.
func (m *Manager) AddAPPlacement(surveyID, floorID string, edit APPlacementEdit) (APLocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, floor, err := m.floorOf(surveyID, floorID)
	if err != nil {
		return APLocation{}, err
	}
	if err := edit.validate(floor.FloorPlan); err != nil {
		return APLocation{}, err
	}

	ap := APLocation{ID: uuid.NewString()}
	edit.applyTo(&ap)
	floor.APLocations = append(floor.APLocations, ap)
	return ap, m.touchFloor(s, floor)
}

// UpdateAPPlacement moves a placed AP or changes its label, power or band.
// What the operator does not edit — the BSSID, vendor and notes an importer
// recorded, and whether it was imported — is kept: an imported AP dragged to
// where it really hangs is still that AP.
func (m *Manager) UpdateAPPlacement(
	surveyID, floorID, placementID string,
	edit APPlacementEdit,
) (APLocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, floor, err := m.floorOf(surveyID, floorID)
	if err != nil {
		return APLocation{}, err
	}
	i := placementIndex(floor, placementID)
	if i < 0 {
		return APLocation{}, fmt.Errorf("%w: %s", ErrPlacementNotFound, placementID)
	}
	if err := edit.validate(floor.FloorPlan); err != nil {
		return APLocation{}, err
	}

	edit.applyTo(&floor.APLocations[i])
	return floor.APLocations[i], m.touchFloor(s, floor)
}

// DeleteAPPlacement removes a placed AP from a floor and returns the floor's
// remaining placements.
func (m *Manager) DeleteAPPlacement(surveyID, floorID, placementID string) ([]APLocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, floor, err := m.floorOf(surveyID, floorID)
	if err != nil {
		return nil, err
	}
	i := placementIndex(floor, placementID)
	if i < 0 {
		return nil, fmt.Errorf("%w: %s", ErrPlacementNotFound, placementID)
	}

	floor.APLocations = slices.Delete(floor.APLocations, i, i+1)
	return slices.Clone(floor.APLocations), m.touchFloor(s, floor)
}

func placementIndex(floor *Floor, placementID string) int {
	return slices.IndexFunc(floor.APLocations, func(ap APLocation) bool {
		return ap.ID == placementID
	})
}

// touchFloor stamps a floor and its survey as changed and stores the survey.
// The caller holds m.mu.
func (m *Manager) touchFloor(s *Survey, floor *Floor) error {
	floor.UpdatedAt = time.Now()
	s.UpdatedAt = floor.UpdatedAt
	return m.persistSurvey(s)
}
