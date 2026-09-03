// SPDX-License-Identifier: BUSL-1.1

// Package api implements the connectrpc handlers that expose Trellis's
// measured-survey engine (core/survey). It is a thin translation layer:
// validate the request, call the engine, map the result onto the proto
// reply. No survey logic lives here.
package api

import (
	"context"
	"errors"
	"fmt"
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

// GetHeatmap renders a measured-signal heatmap for the floor a request names,
// or the survey's active floor when it names none.
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

	floor, err := floorOf(svy, req.Msg.GetFloorId())
	if err != nil {
		return nil, err
	}

	result, err := survey.GenerateFloorHeatmap(floor, config)
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
		Metric:      result.Type,
		Legend:      legendStops(result.Scale),
		Grid:        flattenGrid(result.Grid),
		GridCols:    int32Of(gridCols(result.Grid)),
		GridRows:    int32Of(len(result.Grid)),
		CellSize:    int32Of(result.CellSize),
	}), nil
}

// flattenGrid lays the interpolated field out row-major for the wire.
// float32 rather than float64: these are dBm and dB to a fraction of a
// decibel, and halving the payload matters more than digits no radio measures.
func flattenGrid(grid [][]float64) []float32 {
	if len(grid) == 0 {
		return nil
	}

	flat := make([]float32, 0, len(grid)*len(grid[0]))
	for _, row := range grid {
		for _, value := range row {
			flat = append(flat, float32(value))
		}
	}
	return flat
}

// gridCols reports the row width of a grid whose rows are all the same length
// by construction (InterpolateGrid builds them from one cols count).
func gridCols(grid [][]float64) int {
	if len(grid) == 0 {
		return 0
	}
	return len(grid[0])
}

// legendStops maps the colour scale that painted the image onto the reply, so
// the legend a client draws and the gradient it labels are the same scale.
func legendStops(scale survey.ColorScale) []*surveyv1.LegendStop {
	stops := make([]*surveyv1.LegendStop, 0, len(scale.Stops))
	for _, stop := range scale.Stops {
		stops = append(stops, &surveyv1.LegendStop{
			Value: stop.Value,
			Color: fmt.Sprintf("#%02x%02x%02x", stop.Color.R, stop.Color.G, stop.Color.B),
		})
	}
	return stops
}

// GetCoverage runs dead-zone detection over one floor's measured samples.
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

	// Coverage is scored per floor, like the heatmap beside it: a whole-survey
	// score would answer a question about the building while the picture on
	// screen shows one storey.
	floor, err := floorOf(svy, req.Msg.GetFloorId())
	if err != nil {
		return nil, err
	}

	analysis, err := survey.DetectFloorDeadZones(svy.ID, floor, threshold, nil)
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

	pdf, err := survey.NewReportGenerator(svy, reportOptions(req.Msg.GetOptions())).Generate()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&surveyv1.GenerateReportResponse{Pdf: pdf}), nil
}

// reportOptions maps the request's section choices onto the generator's.
//
// A request that sends no options gets the engine's defaults, which is what
// every report got before the options could cross the wire at all. Once a
// caller does send them, they are taken literally: proto3 cannot distinguish
// an unset bool from false, so a partially-filled options message would
// otherwise silently re-enable sections the operator turned off.
func reportOptions(opts *surveyv1.ReportOptions) survey.ReportOptions {
	if opts == nil {
		return survey.DefaultReportOptions()
	}
	return survey.ReportOptions{
		IncludeHeatmaps:         opts.GetIncludeHeatmaps(),
		IncludeRawData:          opts.GetIncludeRawData(),
		IncludeRecommendations:  opts.GetIncludeRecommendations(),
		IncludeExecutiveSummary: opts.GetIncludeExecutiveSummary(),
		CompanyName:             opts.GetCompanyName(),
	}
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

// notFoundOrInternal maps the manager's not-found errors onto
// connect.CodeNotFound via the exported survey sentinels; anything else
// surfaces as CodeInternal.
func notFoundOrInternal(err error) error {
	if errors.Is(err, survey.ErrSurveyNotFound) || errors.Is(err, survey.ErrFloorNotFound) {
		return connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewError(connect.CodeInternal, err)
}
