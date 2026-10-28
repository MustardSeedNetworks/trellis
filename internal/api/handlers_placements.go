// SPDX-License-Identifier: BUSL-1.1

package api

import (
	"context"
	"errors"

	"connectrpc.com/connect/v2"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	surveyv1 "github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1"
)

// ListAPPlacements returns the APs placed on a floor.
func (h *SurveyServiceHandler) ListAPPlacements(
	_ context.Context,
	req *surveyv1.ListAPPlacementsRequest,
) (*surveyv1.ListAPPlacementsResponse, error) {
	_, floor, err := h.floorFor(req.GetSurveyId(), req.GetFloorId())
	if err != nil {
		return nil, err
	}
	return &surveyv1.ListAPPlacementsResponse{
		Placements: toAPPlacements(floor.APLocations),
	}, nil
}

// CreateAPPlacement places an AP on a floor by hand.
func (h *SurveyServiceHandler) CreateAPPlacement(
	_ context.Context,
	req *surveyv1.CreateAPPlacementRequest,
) (*surveyv1.CreateAPPlacementResponse, error) {
	svy, floor, err := h.floorFor(req.GetSurveyId(), req.GetFloorId())
	if err != nil {
		return nil, err
	}
	ap, err := h.manager.AddAPPlacement(svy.ID, floor.ID, fromAPPlacementEdit(req.GetPlacement()))
	if err != nil {
		return nil, placementError(err)
	}
	return &surveyv1.CreateAPPlacementResponse{Placement: toAPPlacement(ap)}, nil
}

// UpdateAPPlacement moves a placed AP or changes its label, power or band.
func (h *SurveyServiceHandler) UpdateAPPlacement(
	_ context.Context,
	req *surveyv1.UpdateAPPlacementRequest,
) (*surveyv1.UpdateAPPlacementResponse, error) {
	placementID := req.GetPlacementId()
	if placementID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "placement_id is required")
	}
	svy, floor, err := h.floorFor(req.GetSurveyId(), req.GetFloorId())
	if err != nil {
		return nil, err
	}
	ap, err := h.manager.UpdateAPPlacement(svy.ID, floor.ID, placementID,
		fromAPPlacementEdit(req.GetPlacement()))
	if err != nil {
		return nil, placementError(err)
	}
	return &surveyv1.UpdateAPPlacementResponse{Placement: toAPPlacement(ap)}, nil
}

// DeleteAPPlacement removes a placed AP and returns the floor's remaining ones.
func (h *SurveyServiceHandler) DeleteAPPlacement(
	_ context.Context,
	req *surveyv1.DeleteAPPlacementRequest,
) (*surveyv1.DeleteAPPlacementResponse, error) {
	placementID := req.GetPlacementId()
	if placementID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "placement_id is required")
	}
	svy, floor, err := h.floorFor(req.GetSurveyId(), req.GetFloorId())
	if err != nil {
		return nil, err
	}
	remaining, err := h.manager.DeleteAPPlacement(svy.ID, floor.ID, placementID)
	if err != nil {
		return nil, placementError(err)
	}
	return &surveyv1.DeleteAPPlacementResponse{
		Placements: toAPPlacements(remaining),
	}, nil
}

// placementError maps a placement failure onto a code that says whose problem
// it is. A floor with no plan has nowhere to put an AP, which is a fact about
// the floor; a position off the plan, an unknown band or an impossible power is
// the request's.
func placementError(err error) error {
	switch {
	case errors.Is(err, survey.ErrSurveyNotFound),
		errors.Is(err, survey.ErrFloorNotFound),
		errors.Is(err, survey.ErrPlacementNotFound):
		return connect.NewError(connect.CodeNotFound, err.Error()).WithCause(err)
	case errors.Is(err, survey.ErrNoFloorPlan):
		return connect.NewError(connect.CodeFailedPrecondition, err.Error()).WithCause(err)
	case errors.Is(err, survey.ErrPlacementOffPlan),
		errors.Is(err, survey.ErrUnknownBand),
		errors.Is(err, survey.ErrTxPowerRange):
		return connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	default:
		return connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
}

func fromAPPlacementEdit(in *surveyv1.APPlacementEdit) survey.APPlacementEdit {
	return survey.APPlacementEdit{
		X:          int(in.GetX()),
		Y:          int(in.GetY()),
		Label:      in.GetLabel(),
		TxPowerDBm: in.TxPowerDbm,
		Band:       in.GetBand(),
	}
}

func toAPPlacements(in []survey.APLocation) []*surveyv1.APPlacement {
	out := make([]*surveyv1.APPlacement, 0, len(in))
	for _, ap := range in {
		out = append(out, toAPPlacement(ap))
	}
	return out
}

func toAPPlacement(ap survey.APLocation) *surveyv1.APPlacement {
	return &surveyv1.APPlacement{
		Id:         ap.ID,
		X:          int32Of(ap.X),
		Y:          int32Of(ap.Y),
		Label:      ap.Label,
		Bssid:      ap.BSSID,
		TxPowerDbm: ap.TxPowerDBm,
		Band:       ap.Band,
		Imported:   ap.Imported,
	}
}
