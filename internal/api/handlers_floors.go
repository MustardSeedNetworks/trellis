// SPDX-License-Identifier: BUSL-1.1

package api

import (
	"context"
	"errors"
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
	return &surveyv1.Floor{
		Id:           floor.ID,
		Name:         floor.Name,
		Level:        int32Of(floor.Level),
		SampleCount:  int32Of(len(floor.Samples)),
		HasFloorPlan: floor.FloorPlan != nil,
		IsActive:     active != nil && active.ID == floor.ID,
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
