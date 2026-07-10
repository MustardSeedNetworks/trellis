// SPDX-License-Identifier: BUSL-1.1

// Package api implements the connectrpc handlers that expose Trellis's
// measured-survey engine (core/survey). It is a thin translation layer:
// validate the request, call the engine, map the result onto the proto
// reply. No survey logic lives here.
package api

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	surveyv1 "github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1"
	"github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1/surveyv1connect"
)

// defaultDeadZoneThresholdDBm is used when a GetCoverage request leaves
// threshold_dbm unset (zero value).
const defaultDeadZoneThresholdDBm = -75

// SurveyServiceHandler implements surveyv1connect.SurveyServiceHandler by
// wrapping a *survey.Manager. It holds no state of its own.
type SurveyServiceHandler struct {
	manager *survey.Manager
}

var _ surveyv1connect.SurveyServiceHandler = (*SurveyServiceHandler)(nil)

// NewSurveyServiceHandler builds a handler over the given survey manager.
func NewSurveyServiceHandler(manager *survey.Manager) *SurveyServiceHandler {
	return &SurveyServiceHandler{manager: manager}
}

// ImportAirMapper imports an AirMapper (.amp) archive into a new stored
// survey.
func (h *SurveyServiceHandler) ImportAirMapper(
	_ context.Context,
	req *connect.Request[surveyv1.ImportAirMapperRequest],
) (*connect.Response[surveyv1.ImportAirMapperResponse], error) {
	name := req.Msg.GetName()
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	data := req.Msg.GetAmpData()
	if len(data) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("amp_data is required"))
	}

	svy, err := h.manager.ImportAirMapper(name, data)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	return connect.NewResponse(&surveyv1.ImportAirMapperResponse{
		Survey: toSurveySummary(svy),
	}), nil
}

// ListSurveys returns summaries of every stored survey.
func (h *SurveyServiceHandler) ListSurveys(
	_ context.Context,
	_ *connect.Request[surveyv1.ListSurveysRequest],
) (*connect.Response[surveyv1.ListSurveysResponse], error) {
	surveys := h.manager.ListSurveys()
	summaries := make([]*surveyv1.SurveySummary, 0, len(surveys))
	for _, svy := range surveys {
		summaries = append(summaries, toSurveySummary(svy))
	}

	return connect.NewResponse(&surveyv1.ListSurveysResponse{Surveys: summaries}), nil
}

// GetSurvey returns a single stored survey by ID.
func (h *SurveyServiceHandler) GetSurvey(
	_ context.Context,
	req *connect.Request[surveyv1.GetSurveyRequest],
) (*connect.Response[surveyv1.GetSurveyResponse], error) {
	id := req.Msg.GetId()
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	svy, err := h.manager.GetSurvey(id)
	if err != nil {
		return nil, notFoundOrInternal(err)
	}

	return connect.NewResponse(&surveyv1.GetSurveyResponse{Survey: toSurveySummary(svy)}), nil
}

// DeleteSurvey removes a stored survey.
func (h *SurveyServiceHandler) DeleteSurvey(
	_ context.Context,
	req *connect.Request[surveyv1.DeleteSurveyRequest],
) (*connect.Response[surveyv1.DeleteSurveyResponse], error) {
	id := req.Msg.GetId()
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	if err := h.manager.DeleteSurvey(id); err != nil {
		return nil, notFoundOrInternal(err)
	}

	return connect.NewResponse(&surveyv1.DeleteSurveyResponse{}), nil
}

// GetHeatmap renders a measured-signal heatmap for a survey's active floor.
func (h *SurveyServiceHandler) GetHeatmap(
	_ context.Context,
	req *connect.Request[surveyv1.GetHeatmapRequest],
) (*connect.Response[surveyv1.GetHeatmapResponse], error) {
	surveyID := req.Msg.GetSurveyId()
	if surveyID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("survey_id is required"))
	}

	svy, err := h.manager.GetSurvey(surveyID)
	if err != nil {
		return nil, notFoundOrInternal(err)
	}

	config := survey.DefaultHeatmapConfig()
	if strings.EqualFold(req.Msg.GetMetric(), "snr") {
		config.Type = survey.HeatmapSNR
	} else {
		config.Type = survey.HeatmapRSSI
	}

	result, err := survey.GenerateHeatmap(svy, config)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	return connect.NewResponse(&surveyv1.GetHeatmapResponse{
		Png:         result.Image,
		Width:       int32(result.Width),
		Height:      int32(result.Height),
		Min:         result.Stats.Min,
		Max:         result.Stats.Max,
		SampleCount: int32(result.SampleCount),
	}), nil
}

// GetCoverage runs dead-zone detection over a survey's measured samples.
func (h *SurveyServiceHandler) GetCoverage(
	_ context.Context,
	req *connect.Request[surveyv1.GetCoverageRequest],
) (*connect.Response[surveyv1.GetCoverageResponse], error) {
	surveyID := req.Msg.GetSurveyId()
	if surveyID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("survey_id is required"))
	}

	svy, err := h.manager.GetSurvey(surveyID)
	if err != nil {
		return nil, notFoundOrInternal(err)
	}

	threshold := int(req.Msg.GetThresholdDbm())
	if threshold == 0 {
		threshold = defaultDeadZoneThresholdDBm
	}

	analysis, err := survey.DetectDeadZones(svy, threshold, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	return connect.NewResponse(&surveyv1.GetCoverageResponse{
		CoverageScore:   analysis.CoverageScore,
		DeadZoneCount:   int32(len(analysis.DeadZones)),
		Recommendations: analysis.Recommendations,
	}), nil
}

// GenerateReport renders a PDF report for a survey.
func (h *SurveyServiceHandler) GenerateReport(
	_ context.Context,
	req *connect.Request[surveyv1.GenerateReportRequest],
) (*connect.Response[surveyv1.GenerateReportResponse], error) {
	surveyID := req.Msg.GetSurveyId()
	if surveyID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("survey_id is required"))
	}

	svy, err := h.manager.GetSurvey(surveyID)
	if err != nil {
		return nil, notFoundOrInternal(err)
	}

	pdf, err := survey.NewReportGenerator(svy, survey.DefaultReportOptions()).Generate()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&surveyv1.GenerateReportResponse{Pdf: pdf}), nil
}

// toSurveySummary maps a core survey.Survey onto the proto SurveySummary.
func toSurveySummary(svy *survey.Survey) *surveyv1.SurveySummary {
	hasFloorPlan := false
	if floor := svy.GetActiveFloor(); floor != nil {
		hasFloorPlan = floor.FloorPlan != nil
	}

	return &surveyv1.SurveySummary{
		Id:           svy.ID,
		Name:         svy.Name,
		Status:       string(svy.Status),
		FloorCount:   int32(len(svy.Floors)),
		SampleCount:  int32(len(svy.GetAllSamples())),
		HasFloorPlan: hasFloorPlan,
	}
}

// notFoundOrInternal maps the manager's untyped "not found" errors onto
// connect.CodeNotFound; anything else surfaces as CodeInternal. core/survey
// does not export a sentinel not-found error, so this matches on the message
// the manager consistently uses.
func notFoundOrInternal(err error) error {
	if strings.Contains(err.Error(), "not found") {
		return connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewError(connect.CodeInternal, err)
}
