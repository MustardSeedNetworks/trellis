// SPDX-License-Identifier: BUSL-1.1

package api

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"

	"connectrpc.com/connect"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	surveyv1 "github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1"
)

// ListFloors returns every floor of a survey, lowest level first.
func (h *SurveyServiceHandler) ListFloors(
	_ context.Context,
	req *connect.Request[surveyv1.ListFloorsRequest],
) (*connect.Response[surveyv1.ListFloorsResponse], error) {
	surveyID := req.Msg.GetSurveyId()
	if surveyID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("survey_id is required"))
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

	return connect.NewResponse(&surveyv1.ListFloorsResponse{Floors: floors}), nil
}

// GetFloor returns one floor of a survey by ID.
func (h *SurveyServiceHandler) GetFloor(
	_ context.Context,
	req *connect.Request[surveyv1.GetFloorRequest],
) (*connect.Response[surveyv1.GetFloorResponse], error) {
	surveyID, floorID := req.Msg.GetSurveyId(), req.Msg.GetFloorId()
	if surveyID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("survey_id is required"))
	}
	if floorID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("floor_id is required"))
	}

	svy, err := h.manager.GetSurvey(surveyID)
	if err != nil {
		return nil, notFoundOrInternal(err)
	}
	floor, err := h.manager.GetFloor(surveyID, floorID)
	if err != nil {
		return nil, notFoundOrInternal(err)
	}

	return connect.NewResponse(&surveyv1.GetFloorResponse{
		Floor: toFloor(floor, svy.GetActiveFloor()),
	}), nil
}

// toFloor maps a domain floor onto the wire, marking it active by identity
// against the floor the survey collects onto.
func toFloor(floor *survey.Floor, active *survey.Floor) *surveyv1.Floor {
	out := &surveyv1.Floor{
		Id:           floor.ID,
		Name:         floor.Name,
		Level:        int32Of(floor.Level),
		SampleCount:  int32Of(len(floor.Samples)),
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
	req *connect.Request[surveyv1.SetFloorPlanRequest],
) (*connect.Response[surveyv1.SetFloorPlanResponse], error) {
	svy, floor, err := h.floorFor(req.Msg.GetSurveyId(), req.Msg.GetFloorId())
	if err != nil {
		return nil, err
	}
	if err := h.manager.SetFloorPlan(svy.ID, floor.ID, req.Msg.GetImage()); err != nil {
		return nil, planError(err)
	}
	return connect.NewResponse(&surveyv1.SetFloorPlanResponse{
		Floor: h.floorAfterChange(svy.ID, floor.ID),
	}), nil
}

// CalibrateFloorPlan sets how many metres a pixel of the plan is.
func (h *SurveyServiceHandler) CalibrateFloorPlan(
	_ context.Context,
	req *connect.Request[surveyv1.CalibrateFloorPlanRequest],
) (*connect.Response[surveyv1.CalibrateFloorPlanResponse], error) {
	svy, floor, err := h.floorFor(req.Msg.GetSurveyId(), req.Msg.GetFloorId())
	if err != nil {
		return nil, err
	}
	if err := h.manager.CalibrateFloorPlan(svy.ID, floor.ID,
		survey.Position{X: int(req.Msg.GetX1()), Y: int(req.Msg.GetY1())},
		survey.Position{X: int(req.Msg.GetX2()), Y: int(req.Msg.GetY2())},
		req.Msg.GetMetres(),
	); err != nil {
		return nil, planError(err)
	}
	return connect.NewResponse(&surveyv1.CalibrateFloorPlanResponse{
		Floor: h.floorAfterChange(svy.ID, floor.ID),
	}), nil
}

// GetFloorPlanImage returns a floor's plan image, or nothing when it has none.
//
// An empty reply rather than an error: "this floor has no plan" is an ordinary
// answer, and a client asking so it can draw one has to be able to tell that
// from a failure.
func (h *SurveyServiceHandler) GetFloorPlanImage(
	_ context.Context,
	req *connect.Request[surveyv1.GetFloorPlanImageRequest],
) (*connect.Response[surveyv1.GetFloorPlanImageResponse], error) {
	_, floor, err := h.floorFor(req.Msg.GetSurveyId(), req.Msg.GetFloorId())
	if err != nil {
		return nil, err
	}
	if floor.FloorPlan == nil {
		return connect.NewResponse(&surveyv1.GetFloorPlanImageResponse{}), nil
	}

	image, err := base64.StdEncoding.DecodeString(floor.FloorPlan.ImageData)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal,
			fmt.Errorf("decode stored floor plan: %w", err))
	}
	return connect.NewResponse(&surveyv1.GetFloorPlanImageResponse{
		Image:  image,
		Width:  int32Of(floor.FloorPlan.Width),
		Height: int32Of(floor.FloorPlan.Height),
	}), nil
}

// floorFor resolves the survey and floor a request named, with an empty floor
// meaning the active one.
func (h *SurveyServiceHandler) floorFor(
	surveyID, floorID string,
) (*survey.Survey, *survey.Floor, error) {
	if surveyID == "" {
		return nil, nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("survey_id is required"))
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
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, survey.ErrNoFloorPlan):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	default:
		return connect.NewError(connect.CodeInvalidArgument, err)
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
				errors.New("survey has no floors"))
		}
		return floor, nil
	}

	floor := svy.GetFloorByID(floorID)
	if floor == nil {
		return nil, connect.NewError(connect.CodeNotFound,
			errors.New("floor not found: "+floorID))
	}
	return floor, nil
}
