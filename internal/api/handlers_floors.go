// SPDX-License-Identifier: BUSL-1.1

package api

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"

	"connectrpc.com/connect/v2"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	surveyv1 "github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1"
)

// ListFloors returns every floor of a survey, lowest level first.
func (h *SurveyServiceHandler) ListFloors(
	_ context.Context,
	req *surveyv1.ListFloorsRequest,
) (*surveyv1.ListFloorsResponse, error) {
	surveyID := req.GetSurveyId()
	if surveyID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "survey_id is required")
	}

	svy, err := h.manager.GetSurvey(surveyID)
	if err != nil {
		return nil, notFoundOrInternal(err)
	}

	// The active floor is resolved once here rather than compared per floor:
	// an empty ActiveFloorID means the first floor is active, and asking each
	// floor whether its ID equals "" would mark none of them.
	active := svy.GetActiveFloor()

	floors := make([]*surveyv1.Floor, 0, len(svy.Floors))
	for _, floor := range svy.Floors {
		floors = append(floors, toFloor(floor, active))
	}
	// Storey order, which is what a picker lists and is not the order floors
	// were added in — an imported basement arrives after the ground floor.
	sort.SliceStable(floors, func(i, j int) bool {
		return floors[i].GetLevel() < floors[j].GetLevel()
	})

	return &surveyv1.ListFloorsResponse{Floors: floors}, nil
}

// GetFloor returns one floor of a survey by ID.
func (h *SurveyServiceHandler) GetFloor(
	_ context.Context,
	req *surveyv1.GetFloorRequest,
) (*surveyv1.GetFloorResponse, error) {
	surveyID, floorID := req.GetSurveyId(), req.GetFloorId()
	if surveyID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "survey_id is required")
	}
	if floorID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "floor_id is required")
	}

	svy, err := h.manager.GetSurvey(surveyID)
	if err != nil {
		return nil, notFoundOrInternal(err)
	}
	floor, err := h.manager.GetFloor(surveyID, floorID)
	if err != nil {
		return nil, notFoundOrInternal(err)
	}

	return &surveyv1.GetFloorResponse{
		Floor: toFloor(floor, svy.GetActiveFloor()),
	}, nil
}

// CreateFloor adds a storey to a survey.
//
// The new floor is not made active: a survey is laid out floor by floor before
// it is walked, and switching the walk on every add would move it away from the
// floor being measured.
func (h *SurveyServiceHandler) CreateFloor(
	_ context.Context,
	req *surveyv1.CreateFloorRequest,
) (*surveyv1.CreateFloorResponse, error) {
	surveyID, name := req.GetSurveyId(), req.GetName()
	if surveyID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "survey_id is required")
	}

	// AddFloor refuses a blank name itself, so there is one rule about what a
	// floor may be called rather than two that can drift apart.
	floor, err := h.manager.AddFloor(surveyID, name, int(req.GetLevel()))
	if err != nil {
		return nil, floorError(err)
	}

	return &surveyv1.CreateFloorResponse{
		Floor: h.floorAfterChange(surveyID, floor.ID),
	}, nil
}

// SetActiveFloor moves the walk onto another floor of the same survey.
func (h *SurveyServiceHandler) SetActiveFloor(
	_ context.Context,
	req *surveyv1.SetActiveFloorRequest,
) (*surveyv1.SetActiveFloorResponse, error) {
	surveyID, floorID := req.GetSurveyId(), req.GetFloorId()
	if surveyID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "survey_id is required")
	}
	if floorID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "floor_id is required")
	}

	if err := h.manager.SetActiveFloor(surveyID, floorID); err != nil {
		return nil, floorError(err)
	}

	return &surveyv1.SetActiveFloorResponse{
		Floor: h.floorAfterChange(surveyID, floorID),
	}, nil
}

// UpdateFloor renames a floor and sets its storey number.
func (h *SurveyServiceHandler) UpdateFloor(
	_ context.Context,
	req *surveyv1.UpdateFloorRequest,
) (*surveyv1.UpdateFloorResponse, error) {
	surveyID, floorID := req.GetSurveyId(), req.GetFloorId()
	if surveyID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "survey_id is required")
	}
	if floorID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "floor_id is required")
	}

	if err := h.manager.UpdateFloor(surveyID, floorID,
		req.GetName(), int(req.GetLevel())); err != nil {
		return nil, floorError(err)
	}

	return &surveyv1.UpdateFloorResponse{
		Floor: h.floorAfterChange(surveyID, floorID),
	}, nil
}

// DeleteFloor removes a storey and the measurements walked on it.
//
// The reply is the survey's remaining floors rather than an acknowledgement:
// deleting the floor being walked moves the walk, so the caller cannot derive
// the new state from the request it sent.
func (h *SurveyServiceHandler) DeleteFloor(
	ctx context.Context,
	req *surveyv1.DeleteFloorRequest,
) (*surveyv1.DeleteFloorResponse, error) {
	surveyID, floorID := req.GetSurveyId(), req.GetFloorId()
	if surveyID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "survey_id is required")
	}
	if floorID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "floor_id is required")
	}

	if err := h.manager.DeleteFloor(surveyID, floorID); err != nil {
		return nil, floorError(err)
	}

	listed, err := h.ListFloors(ctx,
		&surveyv1.ListFloorsRequest{SurveyId: surveyID})
	if err != nil {
		return nil, err
	}
	return &surveyv1.DeleteFloorResponse{
		Floors: listed.GetFloors(),
	}, nil
}

// floorError maps a floor failure onto a code that says whose problem it is.
// A blank name is the request's fault; the last floor of a survey is a true
// statement about the survey, which is what FailedPrecondition is for.
func floorError(err error) error {
	switch {
	case errors.Is(err, survey.ErrFloorNameEmpty):
		return connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	case errors.Is(err, survey.ErrLastFloor):
		return connect.NewError(connect.CodeFailedPrecondition, err.Error()).WithCause(err)
	default:
		return notFoundOrInternal(err)
	}
}

// toFloor maps a domain floor onto the wire, marking it active by identity
// against the floor the survey collects onto.
func toFloor(floor *survey.Floor, active *survey.Floor) *surveyv1.Floor {
	out := &surveyv1.Floor{
		Id:    floor.ID,
		Name:  floor.Name,
		Level: int32Of(floor.Level),
		// Measurements, not points — see the survey summary (ADR-0009).
		SampleCount:  int32Of(len(floor.MeasuredSamples())),
		HasFloorPlan: floor.FloorPlan != nil,
		IsActive:     active != nil && active.ID == floor.ID,
	}
	if floor.FloorPlan != nil {
		out.PlanWidth = int32Of(floor.FloorPlan.Width)
		out.PlanHeight = int32Of(floor.FloorPlan.Height)
		out.ScaleM = floor.FloorPlan.ScaleM
	}
	return out
}

// SetFloorPlan stores an image as a floor's plan.
func (h *SurveyServiceHandler) SetFloorPlan(
	_ context.Context,
	req *surveyv1.SetFloorPlanRequest,
) (*surveyv1.SetFloorPlanResponse, error) {
	svy, floor, err := h.floorFor(req.GetSurveyId(), req.GetFloorId())
	if err != nil {
		return nil, err
	}
	if err := h.manager.SetFloorPlan(svy.ID, floor.ID, req.GetImage()); err != nil {
		return nil, planError(err)
	}
	return &surveyv1.SetFloorPlanResponse{
		Floor: h.floorAfterChange(svy.ID, floor.ID),
	}, nil
}

// CalibrateFloorPlan sets how many metres a pixel of the plan is.
func (h *SurveyServiceHandler) CalibrateFloorPlan(
	_ context.Context,
	req *surveyv1.CalibrateFloorPlanRequest,
) (*surveyv1.CalibrateFloorPlanResponse, error) {
	svy, floor, err := h.floorFor(req.GetSurveyId(), req.GetFloorId())
	if err != nil {
		return nil, err
	}
	if err := h.manager.CalibrateFloorPlan(svy.ID, floor.ID,
		survey.Position{X: int(req.GetX1()), Y: int(req.GetY1())},
		survey.Position{X: int(req.GetX2()), Y: int(req.GetY2())},
		req.GetMetres(),
	); err != nil {
		return nil, planError(err)
	}
	return &surveyv1.CalibrateFloorPlanResponse{
		Floor: h.floorAfterChange(svy.ID, floor.ID),
	}, nil
}

// GetFloorPlanImage returns a floor's plan image, or nothing when it has none.
//
// An empty reply rather than an error: "this floor has no plan" is an ordinary
// answer, and a client asking so it can draw one has to be able to tell that
// from a failure.
func (h *SurveyServiceHandler) GetFloorPlanImage(
	_ context.Context,
	req *surveyv1.GetFloorPlanImageRequest,
) (*surveyv1.GetFloorPlanImageResponse, error) {
	_, floor, err := h.floorFor(req.GetSurveyId(), req.GetFloorId())
	if err != nil {
		return nil, err
	}
	if floor.FloorPlan == nil {
		return &surveyv1.GetFloorPlanImageResponse{}, nil
	}

	image, err := base64.StdEncoding.DecodeString(floor.FloorPlan.ImageData)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal,
			fmt.Errorf("decode stored floor plan: %w", err).Error())
	}
	return &surveyv1.GetFloorPlanImageResponse{
		Image:  image,
		Width:  int32Of(floor.FloorPlan.Width),
		Height: int32Of(floor.FloorPlan.Height),
	}, nil
}

// floorFor resolves the survey and floor a request named, with an empty floor
// meaning the active one.
func (h *SurveyServiceHandler) floorFor(
	surveyID, floorID string,
) (*survey.Survey, *survey.Floor, error) {
	if surveyID == "" {
		return nil, nil, connect.NewError(connect.CodeInvalidArgument,
			"survey_id is required")
	}
	svy, err := h.manager.GetSurvey(surveyID)
	if err != nil {
		return nil, nil, notFoundOrInternal(err)
	}
	floor, err := floorOf(svy, floorID)
	if err != nil {
		return nil, nil, err
	}
	return svy, floor, nil
}

// floorAfterChange re-reads the floor so the reply describes what was stored
// rather than what was asked for.
func (h *SurveyServiceHandler) floorAfterChange(surveyID, floorID string) *surveyv1.Floor {
	svy, err := h.manager.GetSurvey(surveyID)
	if err != nil {
		return nil
	}
	return toFloor(svy.GetFloorByID(floorID), svy.GetActiveFloor())
}

// planError maps a floor-plan failure onto a code that says whose problem it
// is. A file that is not an image, and a calibration whose two points are the
// same, are both the request's fault; the caller can fix either.
func planError(err error) error {
	switch {
	case errors.Is(err, survey.ErrSurveyNotFound), errors.Is(err, survey.ErrFloorNotFound):
		return connect.NewError(connect.CodeNotFound, err.Error()).WithCause(err)
	case errors.Is(err, survey.ErrNoFloorPlan),
		errors.Is(err, survey.ErrPlanWouldStrandSamples):
		// Both are true statements about the survey's current state rather
		// than faults in the request, and the message carries what the
		// operator needs to decide what to do next.
		return connect.NewError(connect.CodeFailedPrecondition, err.Error()).WithCause(err)
	default:
		return connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	}
}

// floorOf resolves the floor a request named, or the active floor when it
// named none. A named floor that does not exist is an error rather than a
// silent fall back to the active one, which would answer about a floor the
// caller did not ask for.
func floorOf(svy *survey.Survey, floorID string) (*survey.Floor, error) {
	if floorID == "" {
		floor := svy.GetActiveFloor()
		if floor == nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				"survey has no floors")
		}
		return floor, nil
	}

	floor := svy.GetFloorByID(floorID)
	if floor == nil {
		return nil, connect.NewError(connect.CodeNotFound,
			"floor not found: "+floorID)
	}
	return floor, nil
}
