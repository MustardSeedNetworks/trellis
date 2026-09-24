// SPDX-License-Identifier: BUSL-1.1

package survey_test

// A floor walked without a noise figure has samples and no SNR. The heatmap
// and the dead-zone analysis both say so with ErrMetricUnmeasured, which a
// caller can tell apart from a broken request; a floor with no samples at all
// is a different fact and must not borrow the sentinel (trellis#607).

import (
	"errors"
	"testing"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

func floorlessFloor() *survey.Floor {
	samples := make([]*survey.SamplePoint, 0, 3)
	for i := range 3 {
		samples = append(samples, &survey.SamplePoint{
			X: 40 + i*50, Y: 30 + i*30, Timestamp: time.Now(),
			SampleData: &survey.PassiveSample{
				Networks: []*wifi.ScannedNetwork{{Signal: -58 - i*4}},
			},
		})
	}
	return &survey.Floor{ID: "f", Name: "Floor 1", Samples: samples}
}

func TestUnmeasuredMetricIsDistinguishable(t *testing.T) {
	t.Parallel()

	floor := floorlessFloor()

	config := survey.DefaultHeatmapConfig()
	config.Type = survey.HeatmapSNR
	if _, err := survey.GenerateFloorHeatmap(floor, config); !errors.Is(err, survey.ErrMetricUnmeasured) {
		t.Errorf("GenerateFloorHeatmap(snr) = %v, want ErrMetricUnmeasured", err)
	}

	_, err := survey.DetectFloorDeadZones("s", floor, survey.HeatmapSNR,
		survey.DefaultThresholdFor(survey.HeatmapSNR), nil)
	if !errors.Is(err, survey.ErrMetricUnmeasured) {
		t.Errorf("DetectFloorDeadZones(snr) = %v, want ErrMetricUnmeasured", err)
	}

	whole := &survey.Survey{ID: "s", FloorPlan: &survey.FloorPlan{Width: 200, Height: 200}, Samples: floor.Samples}
	if _, err := survey.GenerateHeatmap(whole, config); !errors.Is(err, survey.ErrMetricUnmeasured) {
		t.Errorf("GenerateHeatmap(snr) = %v, want ErrMetricUnmeasured", err)
	}

	// Signal was measured, so the RSSI layer of the same floor still draws.
	if _, err := survey.GenerateFloorHeatmap(floor, survey.DefaultHeatmapConfig()); err != nil {
		t.Errorf("GenerateFloorHeatmap(rssi) on the same floor: %v", err)
	}
}

func TestEmptyFloorIsNotAnUnmeasuredMetric(t *testing.T) {
	t.Parallel()

	empty := &survey.Floor{ID: "e", FloorPlan: &survey.FloorPlan{Width: 200, Height: 200}}
	config := survey.DefaultHeatmapConfig()
	config.Type = survey.HeatmapSNR
	_, err := survey.GenerateFloorHeatmap(empty, config)
	if err == nil {
		t.Fatal("GenerateFloorHeatmap on a floor with no samples: want an error, got none")
	}
	if errors.Is(err, survey.ErrMetricUnmeasured) {
		t.Errorf("an unwalked floor reported as an unmeasured metric: %v", err)
	}
}
