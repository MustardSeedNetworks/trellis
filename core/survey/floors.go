package survey

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// UpdateFloorPlan updates the floor plan for the active floor (or specified floor).
func (m *Manager) UpdateFloorPlan(id string, floorPlan *FloorPlan) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	survey, exists := m.surveys[id]
	if !exists {
		return fmt.Errorf("%w: %s", ErrSurveyNotFound, id)
	}

	// Find the active floor
	floor := survey.GetActiveFloor()
	if floor == nil {
		return fmt.Errorf("no active floor set for survey: %s", id)
	}

	floor.FloorPlan = floorPlan
	floor.UpdatedAt = time.Now()
	survey.UpdatedAt = time.Now()

	return m.persistSurvey(survey)
}

// UpdateFloorPlanByFloorID updates the floor plan for a specific floor.
func (m *Manager) UpdateFloorPlanByFloorID(surveyID, floorID string, floorPlan *FloorPlan) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	survey, exists := m.surveys[surveyID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrSurveyNotFound, surveyID)
	}

	floor := survey.GetFloorByID(floorID)
	if floor == nil {
		return fmt.Errorf("%w: %s", ErrFloorNotFound, floorID)
	}

	floor.FloorPlan = floorPlan
	floor.UpdatedAt = time.Now()
	survey.UpdatedAt = time.Now()

	return m.persistSurvey(survey)
}

// AddFloor adds a new floor to a survey.
func (m *Manager) AddFloor(surveyID, name string, level int) (*Floor, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrFloorNameEmpty
	}

	survey, exists := m.surveys[surveyID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrSurveyNotFound, surveyID)
	}

	now := time.Now()
	floor := &Floor{
		ID:        uuid.New().String(),
		Name:      name,
		Level:     level,
		Samples:   make([]*SamplePoint, 0),
		CreatedAt: now,
		UpdatedAt: now,
	}

	survey.Floors = append(survey.Floors, floor)
	survey.UpdatedAt = now

	if err := m.persistSurvey(survey); err != nil {
		return nil, err
	}

	return floor, nil
}

// UpdateFloor updates floor metadata (name, level).
//
// Both fields are written on every call: proto3 cannot tell a level of 0 from
// an absent one, and a ground floor is level 0, so a caller sends the pair it
// wants the floor to end up with rather than a patch.
func (m *Manager) UpdateFloor(surveyID, floorID, name string, level int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name = strings.TrimSpace(name)
	if name == "" {
		return ErrFloorNameEmpty
	}

	survey, exists := m.surveys[surveyID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrSurveyNotFound, surveyID)
	}

	floor := survey.GetFloorByID(floorID)
	if floor == nil {
		return fmt.Errorf("%w: %s", ErrFloorNotFound, floorID)
	}

	floor.Name = name
	floor.Level = level
	floor.UpdatedAt = time.Now()
	survey.UpdatedAt = time.Now()

	return m.persistSurvey(survey)
}

// DeleteFloor removes a floor from a survey.
func (m *Manager) DeleteFloor(surveyID, floorID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	survey, exists := m.surveys[surveyID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrSurveyNotFound, surveyID)
	}

	// Whether the floor exists is settled before whether it is the last one:
	// asking the count first answers "cannot delete the last floor" about a
	// floor the caller never named.
	index := -1
	for i, floor := range survey.Floors {
		if floor.ID == floorID {
			index = i
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("%w: %s", ErrFloorNotFound, floorID)
	}
	if len(survey.Floors) <= 1 {
		return ErrLastFloor
	}

	// The floor's measurements go with it: persistSurvey rewrites the survey's
	// floors, and survey_points cascades on floor_id.
	survey.Floors = append(survey.Floors[:index], survey.Floors[index+1:]...)

	// If we deleted the active floor, switch to the first remaining floor
	if survey.ActiveFloorID == floorID {
		survey.ActiveFloorID = survey.Floors[0].ID
	}

	survey.UpdatedAt = time.Now()
	return m.persistSurvey(survey)
}

// SetActiveFloor sets the active floor for data collection.
func (m *Manager) SetActiveFloor(surveyID, floorID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	survey, exists := m.surveys[surveyID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrSurveyNotFound, surveyID)
	}

	// Verify floor exists
	floor := survey.GetFloorByID(floorID)
	if floor == nil {
		return fmt.Errorf("%w: %s", ErrFloorNotFound, floorID)
	}

	survey.ActiveFloorID = floorID
	survey.UpdatedAt = time.Now()

	return m.persistSurvey(survey)
}

// GetFloors returns all floors for a survey.
func (m *Manager) GetFloors(surveyID string) ([]*Floor, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	survey, exists := m.surveys[surveyID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrSurveyNotFound, surveyID)
	}

	return survey.Floors, nil
}

// GetFloor returns a specific floor.
func (m *Manager) GetFloor(surveyID, floorID string) (*Floor, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	survey, exists := m.surveys[surveyID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrSurveyNotFound, surveyID)
	}

	floor := survey.GetFloorByID(floorID)
	if floor == nil {
		return nil, fmt.Errorf("%w: %s", ErrFloorNotFound, floorID)
	}

	return floor, nil
}
