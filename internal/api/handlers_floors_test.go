// SPDX-License-Identifier: BUSL-1.1

package api_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
	surveyv1 "github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1"
	"github.com/MustardSeedNetworks/trellis/internal/api"
)

// twoFloorSurvey returns a handler over a survey with a strong ground floor and
// a weak upper floor, each carrying its own measurements. The two differ by
// enough that a reply about the wrong floor is visible in the numbers.
func twoFloorSurvey(t *testing.T) (*api.SurveyServiceHandler, *survey.Manager, string, string, string) {
	t.Helper()

	mgr := mustManager(t, t.TempDir(), nil, nil, nil, nil)
	svy, err := mgr.CreateSurvey("Everett HQ", "", "en0", survey.TypePassive)
	if err != nil {
		t.Fatalf("CreateSurvey: %v", err)
	}
	if err := mgr.StartSurvey(svy.ID); err != nil {
		t.Fatalf("StartSurvey: %v", err)
	}

	// CreateSurvey opens with "Floor 1"; the basement is added after it and is
	// the lower storey, which is what makes list order worth asserting.
	ground := svy.Floors[0].ID
	basement, err := mgr.AddFloor(svy.ID, "Basement", -1)
	if err != nil {
		t.Fatalf("AddFloor: %v", err)
	}

	addSample := func(floorID string, x, y, signal, snr int) {
		t.Helper()
		sample := &survey.PassiveSample{
			Networks: []*wifi.ScannedNetwork{{
				SSID: "corp", BSSID: "aa:bb:cc:00:00:01",
				Signal: signal, SNR: snr, Channel: 6, Frequency: 2437,
			}},
		}
		if err := mgr.AddSampleToFloor(svy.ID, floorID, x, y, sample); err != nil {
			t.Fatalf("AddSampleToFloor(%s): %v", floorID, err)
		}
	}
	addSample(ground, 10, 10, -50, 40)
	addSample(ground, 90, 90, -55, 35)
	addSample(basement.ID, 10, 10, -88, 6)
	addSample(basement.ID, 90, 90, -86, 7)
	addSample(basement.ID, 50, 50, -90, 5)

	return api.NewSurveyServiceHandler(mgr), mgr, svy.ID, ground, basement.ID
}

func TestListFloorsOrdersByStoreyAndMarksTheActiveOne(t *testing.T) {
	t.Parallel()

	handler, _, surveyID, ground, basement := twoFloorSurvey(t)

	resp, err := handler.ListFloors(context.Background(),
		connect.NewRequest(&surveyv1.ListFloorsRequest{SurveyId: surveyID}))
	if err != nil {
		t.Fatalf("ListFloors: %v", err)
	}

	floors := resp.Msg.GetFloors()
	if len(floors) != 2 {
		t.Fatalf("floors = %d, want 2", len(floors))
	}
	// Basement is level -1 and was added second: storey order, not insertion
	// order.
	if floors[0].GetId() != basement {
		t.Errorf("floors[0] = %q, want the basement (level -1) first", floors[0].GetName())
	}
	if floors[0].GetLevel() != -1 {
		t.Errorf("basement level = %d, want -1", floors[0].GetLevel())
	}
	if floors[0].GetSampleCount() != 3 || floors[1].GetSampleCount() != 2 {
		t.Errorf("sample counts = %d and %d, want 3 (basement) and 2 (ground)",
			floors[0].GetSampleCount(), floors[1].GetSampleCount())
	}
	// The survey collects onto Floor 1, which is what an empty ActiveFloorID
	// means. Exactly one floor may say so.
	if floors[0].GetIsActive() {
		t.Error("basement reported active; the survey collects onto Floor 1")
	}
	if !floors[1].GetIsActive() {
		t.Error("no floor reported active, so a picker cannot say which is being walked")
	}
	if floors[1].GetId() != ground {
		t.Errorf("floors[1] = %q, want the ground floor", floors[1].GetId())
	}
	if floors[0].GetHasFloorPlan() {
		t.Error("HasFloorPlan = true for a floor walked without a plan")
	}
}

func TestGetFloorAnswersAboutTheFloorAsked(t *testing.T) {
	t.Parallel()

	handler, _, surveyID, _, basement := twoFloorSurvey(t)

	resp, err := handler.GetFloor(context.Background(),
		connect.NewRequest(&surveyv1.GetFloorRequest{SurveyId: surveyID, FloorId: basement}))
	if err != nil {
		t.Fatalf("GetFloor: %v", err)
	}
	if got := resp.Msg.GetFloor(); got.GetName() != "Basement" || got.GetSampleCount() != 3 {
		t.Errorf("floor = %q with %d samples, want Basement with 3",
			got.GetName(), got.GetSampleCount())
	}

	_, err = handler.GetFloor(context.Background(),
		connect.NewRequest(&surveyv1.GetFloorRequest{SurveyId: surveyID, FloorId: "no-such-floor"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("GetFloor(unknown) = %v, want NotFound", connect.CodeOf(err))
	}

	_, err = handler.GetFloor(context.Background(),
		connect.NewRequest(&surveyv1.GetFloorRequest{SurveyId: surveyID}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("GetFloor(no floor_id) = %v, want InvalidArgument", connect.CodeOf(err))
	}
}

// The reason floor_id exists: without it both floors are answered with the
// same picture and the same score.
func TestHeatmapAndCoverageAreScopedToTheNamedFloor(t *testing.T) {
	t.Parallel()

	handler, _, surveyID, ground, basement := twoFloorSurvey(t)
	ctx := context.Background()

	groundMap, err := handler.GetHeatmap(ctx, connect.NewRequest(&surveyv1.GetHeatmapRequest{
		SurveyId: surveyID, FloorId: ground,
	}))
	if err != nil {
		t.Fatalf("GetHeatmap(ground): %v", err)
	}
	basementMap, err := handler.GetHeatmap(ctx, connect.NewRequest(&surveyv1.GetHeatmapRequest{
		SurveyId: surveyID, FloorId: basement,
	}))
	if err != nil {
		t.Fatalf("GetHeatmap(basement): %v", err)
	}

	if groundMap.Msg.GetSampleCount() != 2 || basementMap.Msg.GetSampleCount() != 3 {
		t.Errorf("sample counts = %d and %d, want 2 (ground) and 3 (basement)",
			groundMap.Msg.GetSampleCount(), basementMap.Msg.GetSampleCount())
	}
	if groundMap.Msg.GetMin() < -60 {
		t.Errorf("ground floor min = %.1f dBm, want its own (~-55), not the basement's",
			groundMap.Msg.GetMin())
	}
	if basementMap.Msg.GetMax() > -70 {
		t.Errorf("basement max = %.1f dBm, want its own (~-86)", basementMap.Msg.GetMax())
	}

	groundCoverage, err := handler.GetCoverage(ctx, connect.NewRequest(&surveyv1.GetCoverageRequest{
		SurveyId: surveyID, ThresholdDbm: -75, FloorId: ground,
	}))
	if err != nil {
		t.Fatalf("GetCoverage(ground): %v", err)
	}
	basementCoverage, err := handler.GetCoverage(ctx, connect.NewRequest(&surveyv1.GetCoverageRequest{
		SurveyId: surveyID, ThresholdDbm: -75, FloorId: basement,
	}))
	if err != nil {
		t.Fatalf("GetCoverage(basement): %v", err)
	}

	if groundCoverage.Msg.GetCoverageScore() != 100 || groundCoverage.Msg.GetDeadZoneCount() != 0 {
		t.Errorf("ground coverage = %.1f%% with %d dead zones, want 100%% and none",
			groundCoverage.Msg.GetCoverageScore(), groundCoverage.Msg.GetDeadZoneCount())
	}
	if basementCoverage.Msg.GetCoverageScore() != 0 || basementCoverage.Msg.GetDeadZoneCount() == 0 {
		t.Errorf("basement coverage = %.1f%% with %d dead zones, want 0%% and at least one",
			basementCoverage.Msg.GetCoverageScore(), basementCoverage.Msg.GetDeadZoneCount())
	}
}

// An unnamed floor is the survey's active one, which is what every
// single-floor caller relies on.
func TestHeatmapWithoutAFloorUsesTheActiveOne(t *testing.T) {
	t.Parallel()

	handler, mgr, surveyID, _, basement := twoFloorSurvey(t)
	ctx := context.Background()

	active, err := handler.GetHeatmap(ctx,
		connect.NewRequest(&surveyv1.GetHeatmapRequest{SurveyId: surveyID}))
	if err != nil {
		t.Fatalf("GetHeatmap(no floor): %v", err)
	}
	if active.Msg.GetSampleCount() != 2 {
		t.Errorf("sample count = %d, want the active floor's 2", active.Msg.GetSampleCount())
	}

	if err := mgr.SetActiveFloor(surveyID, basement); err != nil {
		t.Fatalf("SetActiveFloor: %v", err)
	}
	moved, err := handler.GetHeatmap(ctx,
		connect.NewRequest(&surveyv1.GetHeatmapRequest{SurveyId: surveyID}))
	if err != nil {
		t.Fatalf("GetHeatmap after SetActiveFloor: %v", err)
	}
	if moved.Msg.GetSampleCount() != 3 {
		t.Errorf("sample count = %d, want the basement's 3 once it is active",
			moved.Msg.GetSampleCount())
	}
}

func TestHeatmapAndCoverageRejectAnUnknownFloor(t *testing.T) {
	t.Parallel()

	handler, _, surveyID, _, _ := twoFloorSurvey(t)
	ctx := context.Background()

	_, err := handler.GetHeatmap(ctx, connect.NewRequest(&surveyv1.GetHeatmapRequest{
		SurveyId: surveyID, FloorId: "no-such-floor",
	}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("GetHeatmap(unknown floor) = %v, want NotFound rather than the active floor",
			connect.CodeOf(err))
	}

	_, err = handler.GetCoverage(ctx, connect.NewRequest(&surveyv1.GetCoverageRequest{
		SurveyId: surveyID, FloorId: "no-such-floor",
	}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("GetCoverage(unknown floor) = %v, want NotFound", connect.CodeOf(err))
	}
}

func TestListFloorsRequiresASurvey(t *testing.T) {
	t.Parallel()

	handler := api.NewSurveyServiceHandler(mustManager(t, t.TempDir(), nil, nil, nil, nil))
	ctx := context.Background()

	_, err := handler.ListFloors(ctx, connect.NewRequest(&surveyv1.ListFloorsRequest{}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("ListFloors(no survey_id) = %v, want InvalidArgument", connect.CodeOf(err))
	}

	_, err = handler.ListFloors(ctx,
		connect.NewRequest(&surveyv1.ListFloorsRequest{SurveyId: "no-such-survey"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("ListFloors(unknown survey) = %v, want NotFound", connect.CodeOf(err))
	}
}
