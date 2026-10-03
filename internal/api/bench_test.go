// SPDX-License-Identifier: BUSL-1.1

package api_test

// The PRD's scale target is 100k+ survey points. These benchmarks measure the
// two reads a surveyor makes of a finished floor -- the heatmap and the
// coverage verdict -- at 1k, 10k and 100k points, through the handler the UI
// calls. docs/13-PERFORMANCE.md records the results against the budgets.
//
// The floor is the same synthetic shape core/survey's import benchmark builds:
// a 2000×1500 px plan, walked on an even grid, ten of forty APs heard at
// every point.

import (
	"context"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
	surveyv1 "github.com/MustardSeedNetworks/trellis/gen/trellis/survey/v1"
	"github.com/MustardSeedNetworks/trellis/internal/api"
)

const (
	benchPlanW     = 2000
	benchPlanH     = 1500
	benchAPs       = 40
	benchHeardPerP = 10
)

// benchFloor stores a completed survey of n passive points and returns its ID.
func benchFloor(b *testing.B, mgr *survey.Manager, n int) string {
	b.Helper()

	svy, err := mgr.CreateSurvey("bench", "", "", survey.TypePassive)
	if err != nil {
		b.Fatalf("CreateSurvey: %v", err)
	}
	if err := mgr.UpdateFloorPlan(svy.ID, &survey.FloorPlan{
		Width: benchPlanW, Height: benchPlanH, ScaleM: 0.05,
	}); err != nil {
		b.Fatalf("UpdateFloorPlan: %v", err)
	}
	if err := mgr.StartSurvey(svy.ID); err != nil {
		b.Fatalf("StartSurvey: %v", err)
	}

	cols := 1
	for cols*cols*benchPlanH < n*benchPlanW {
		cols++
	}
	rows := (n + cols - 1) / cols
	for i := range n {
		nets := make([]*wifi.ScannedNetwork, 0, benchHeardPerP)
		for j := range benchHeardPerP {
			ap := (i + j*7) % benchAPs
			signal := -40 - (i+ap*13)%50
			nets = append(nets, &wifi.ScannedNetwork{
				SSID: "corp", BSSID: fmt.Sprintf("aa:bb:cc:00:00:%02x", ap),
				Channel: []int{1, 6, 11, 36, 149}[ap%5], Signal: signal, NoiseFloor: -95, SNR: signal + 95,
			})
		}
		sample := &survey.PassiveSample{Networks: nets}
		sample.CalculateAggregations()
		x, y := (i%cols)*benchPlanW/cols+1, (i/cols)*benchPlanH/rows+1
		if err := mgr.AddSample(svy.ID, x, y, sample); err != nil {
			b.Fatalf("AddSample %d: %v", i, err)
		}
	}
	if err := mgr.CompleteSurvey(svy.ID); err != nil {
		b.Fatalf("CompleteSurvey: %v", err)
	}
	return svy.ID
}

// One floor per size serves both reads: building the 100k floor is the slow
// part, and it is setup, not what is measured.
func BenchmarkFloorReads(b *testing.B) {
	ctx := context.Background()
	for _, n := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("points=%d", n), func(b *testing.B) {
			mgr := mustManager(b, b.TempDir(), nil, nil, nil, nil)
			id := benchFloor(b, mgr, n)
			handler := api.NewSurveyServiceHandler(mgr)

			b.Run("GetHeatmap", func(b *testing.B) {
				req := connect.NewRequest(&surveyv1.GetHeatmapRequest{SurveyId: id, Metric: "rssi"})
				for b.Loop() {
					if _, err := handler.GetHeatmap(ctx, req); err != nil {
						b.Fatalf("GetHeatmap: %v", err)
					}
				}
			})
			b.Run("GetCoverage", func(b *testing.B) {
				req := connect.NewRequest(&surveyv1.GetCoverageRequest{SurveyId: id, Metric: "rssi"})
				for b.Loop() {
					if _, err := handler.GetCoverage(ctx, req); err != nil {
						b.Fatalf("GetCoverage: %v", err)
					}
				}
			})
		})
	}
}
