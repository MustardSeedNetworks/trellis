// SPDX-License-Identifier: BUSL-1.1

package api_test

import (
	"context"
	"fmt"
	"math"
	"testing"

	"connectrpc.com/connect/v2"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	surveyv1 "github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1"
	"github.com/MustardSeedNetworks/trellis/internal/api"
)

// plannedSurvey stores a survey whose one floor has a w×h px plan at scaleM
// metres per pixel and the given APs placed on it, and no walk.
func plannedSurvey(tb testing.TB, w, h int, scaleM float64, aps []survey.APLocation) (*api.SurveyServiceHandler, string) {
	tb.Helper()

	mgr := mustManager(tb, tb.TempDir(), nil, nil, nil, nil)
	svy, err := mgr.CreateSurvey("planned", "", "", survey.TypePassive)
	if err != nil {
		tb.Fatalf("CreateSurvey: %v", err)
	}
	if err := mgr.UpdateFloorPlan(svy.ID, &survey.FloorPlan{Width: w, Height: h, ScaleM: scaleM}); err != nil {
		tb.Fatalf("UpdateFloorPlan: %v", err)
	}
	stored, err := mgr.GetSurvey(svy.ID)
	if err != nil {
		tb.Fatalf("GetSurvey: %v", err)
	}
	if err := mgr.UpdateImportedData(svy.ID, survey.ImportedDataUpdate{
		FloorID: stored.GetActiveFloor().ID, APLocations: aps,
	}); err != nil {
		tb.Fatalf("UpdateImportedData: %v", err)
	}
	return api.NewSurveyServiceHandler(mgr), svy.ID
}

func TestGetHeatmapServesAPrediction(t *testing.T) {
	t.Parallel()

	handler, id := plannedSurvey(t, 400, 300, 0.05, []survey.APLocation{
		{ID: "ap1", X: 105, Y: 105, Band: "5 GHz", BSSID: "aa:bb:cc:00:00:01"},
	})
	got, err := handler.GetHeatmap(context.Background(), &surveyv1.GetHeatmapRequest{
		SurveyId: id, Metric: "predicted_rssi", Band: "5 GHz",
	})
	if err != nil {
		t.Fatalf("GetHeatmap(predicted_rssi): %v", err)
	}
	msg := got
	if msg.GetMetric() != "predicted_rssi" {
		t.Errorf("metric = %q, want predicted_rssi", msg.GetMetric())
	}
	if msg.GetGridCols() != 40 || msg.GetGridRows() != 30 || len(msg.GetGrid()) != 1200 {
		t.Errorf("grid %d×%d (%d values), want 40×30", msg.GetGridCols(), msg.GetGridRows(), len(msg.GetGrid()))
	}
	// The AP sits on the centre of cell (10, 10): 10 dBm − FSPL(5500, 1 m).
	if v := msg.GetGrid()[10*40+10]; v < -37.36 || v > -37.16 {
		t.Errorf("cell under the AP = %.3f dBm, want -37.257", v)
	}
	if len(msg.GetPng()) == 0 || len(msg.GetLegend()) == 0 {
		t.Error("a prediction came back without an image or a legend")
	}
	// Nobody walked the floor, so the AP is drawn uncalibrated and says why.
	cals := msg.GetCalibrations()
	if len(cals) != 1 || cals[0].GetPlacementId() != "ap1" || cals[0].GetCalibrated() ||
		cals[0].GetFallback() == "" || cals[0].GetExponent() != 3 || cals[0].MeanAbsErrorDb != nil {
		t.Errorf("calibrations = %v, want ap1 drawn on the defaults with the reason", cals)
	}
}

func TestGetHeatmapRefusesAPredictionTheFloorCannotCarry(t *testing.T) {
	t.Parallel()

	placed := []survey.APLocation{{ID: "ap1", X: 105, Y: 105, Band: "5 GHz"}}
	for _, tc := range []struct {
		name   string
		scaleM float64
		aps    []survey.APLocation
		band   string
		want   connect.Code
	}{
		{"no scale", 0, placed, "5 GHz", connect.CodeFailedPrecondition},
		{"no placements", 0.05, nil, "5 GHz", connect.CodeFailedPrecondition},
		{"no band", 0.05, placed, "", connect.CodeInvalidArgument},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			handler, id := plannedSurvey(t, 400, 300, tc.scaleM, tc.aps)
			_, err := handler.GetHeatmap(context.Background(), &surveyv1.GetHeatmapRequest{
				SurveyId: id, Metric: "predicted_rssi", Band: tc.band,
			})
			if connect.CodeOf(err) != tc.want {
				t.Errorf("err = %v (%v), want %v", err, connect.CodeOf(err), tc.want)
			}
		})
	}
}

// A delta needs a walk to subtract: a planned floor nobody walked is a fact
// about the survey, not a bad request.
func TestGetHeatmapRefusesADeltaWithNothingWalked(t *testing.T) {
	t.Parallel()

	handler, id := plannedSurvey(t, 400, 300, 0.05, []survey.APLocation{
		{ID: "ap1", X: 105, Y: 105, Band: "5 GHz", BSSID: "aa:bb:cc:00:00:01"},
	})
	_, err := handler.GetHeatmap(context.Background(), &surveyv1.GetHeatmapRequest{
		SurveyId: id, Metric: "predicted_delta", Band: "5 GHz",
	})
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("err = %v (%v), want FailedPrecondition", err, connect.CodeOf(err))
	}
}

// BenchmarkGetHeatmapPredicted serves a prediction through the handler for the
// PRD's reference floor: 5,000 m² (a 4000×2000 px plan at 0.025 m/px, so the
// 10 px cells are 0.25 m) with 20 APs. The model's own share is
// core/rf's BenchmarkPredictGrid; this adds painting and PNG encoding.
func BenchmarkGetHeatmapPredicted(b *testing.B) {
	const w, h = 4000, 2000
	aps := make([]survey.APLocation, 20)
	for i := range aps {
		// An additive-recurrence sequence: deterministic, and spread evenly.
		_, fx := math.Modf(float64(i+1) * 0.6180339887)
		_, fy := math.Modf(float64(i+1) * 0.7548776662)
		aps[i] = survey.APLocation{
			ID: fmt.Sprintf("ap%d", i), X: int(fx * w), Y: int(fy * h), Band: "5 GHz",
		}
	}
	handler, id := plannedSurvey(b, w, h, 0.025, aps)
	req := &surveyv1.GetHeatmapRequest{SurveyId: id, Metric: "predicted_rssi", Band: "5 GHz"}
	for b.Loop() {
		if _, err := handler.GetHeatmap(context.Background(), req); err != nil {
			b.Fatalf("GetHeatmap: %v", err)
		}
	}
}
