// SPDX-License-Identifier: BUSL-1.1

package survey_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/rf"
	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

const calBSSID = "aa:bb:cc:00:00:01"

// known is the model the synthetic walk is generated from: an exponent no
// default would guess and a reference loss 4 dB above free space, the way a
// real floor sits away from the uncalibrated model.
var known = rf.Model{TxPowerDBm: 17, RefLossDB: rf.FreeSpaceLossDB(5500) + 4, Exponent: 2.7}

// calibratedFloor is a 400×300 px plan at 0.1 m/px with one 5 GHz AP on a
// centre cell, walked on a 20 px grid. Each point hears the AP at what known
// predicts there, rounded to the whole dBm a scan reports, plus whatever
// heard() adds.
func calibratedFloor(heard func(x, y int, rssi int) []*wifi.ScannedNetwork) *survey.Floor {
	power := known.TxPowerDBm
	floor := &survey.Floor{
		ID:        "f1",
		FloorPlan: &survey.FloorPlan{Width: 400, Height: 300, ScaleM: 0.1},
		APLocations: []survey.APLocation{
			{ID: "ap1", Label: "AP-1", BSSID: calBSSID, X: 205, Y: 155, Band: "5 GHz", TxPowerDBm: &power},
		},
	}
	for y := 10; y < 300; y += 20 {
		for x := 10; x < 400; x += 20 {
			d := math.Hypot(float64(x-205), float64(y-155)) * 0.1
			rssi := int(math.Round(known.PredictRSSI(d)))
			sample := &survey.PassiveSample{Networks: heard(x, y, rssi)}
			sample.CalculateAggregations()
			floor.Samples = append(floor.Samples, &survey.SamplePoint{X: x, Y: y, Timestamp: time.Now(), SampleData: sample})
		}
	}
	return floor
}

func on5GHz(_, _ int, rssi int) []*wifi.ScannedNetwork {
	return []*wifi.ScannedNetwork{{SSID: "lab", BSSID: calBSSID, Signal: rssi, Channel: 36, Frequency: 5180}}
}

func TestPredictionCalibratesToTheFloorsOwnWalk(t *testing.T) {
	result, err := survey.GenerateFloorHeatmap(calibratedFloor(on5GHz), predictedConfig("5 GHz"))
	if err != nil {
		t.Fatalf("GenerateFloorHeatmap: %v", err)
	}
	if len(result.Calibrations) != 1 {
		t.Fatalf("calibrations = %+v, want one for the one AP", result.Calibrations)
	}
	c := result.Calibrations[0]
	if !c.Calibrated || c.Clamped || c.Fallback != "" {
		t.Fatalf("calibration = %+v, want a trusted fit", c)
	}
	if math.Abs(c.Exponent-known.Exponent) > 0.05 {
		t.Errorf("fitted exponent = %.3f, want %.2f ± 0.05", c.Exponent, known.Exponent)
	}
	if c.PlacementID != "ap1" || c.Label != "AP-1" || c.BSSID != calBSSID || c.Pairs != 15*20 {
		t.Errorf("calibration names %q %q %q over %d pairs, want ap1 AP-1 %s over 300",
			c.PlacementID, c.Label, c.BSSID, c.Pairs, calBSSID)
	}
	// Rounding to whole dBm is the only error left in the walk.
	if c.MeanAbsErrorDB == nil || *c.MeanAbsErrorDB > 0.5 {
		t.Errorf("residual = %v, want under the 0.5 dB rounding", c.MeanAbsErrorDB)
	}
	// The map is drawn with the fit: 10 m east of the AP is cell (15, 30).
	if got, want := result.Grid[15][30], known.PredictRSSI(10); math.Abs(got-want) > 0.3 {
		t.Errorf("cell 10 m east = %.2f dBm, want %.2f from the known model", got, want)
	}
}

func TestPredictionFallsBackWhenTheFitCannotBeTrusted(t *testing.T) {
	for _, tc := range []struct {
		name   string
		floor  func() *survey.Floor
		pairs  int
		reason string
	}{
		{"no BSSID", func() *survey.Floor {
			f := calibratedFloor(on5GHz)
			f.APLocations[0].BSSID = ""
			return f
		}, 0, "no BSSID"},
		{"too few readings", func() *survey.Floor {
			f := calibratedFloor(on5GHz)
			f.Samples = f.Samples[:rf.MinFitPairs-1]
			return f
		}, rf.MinFitPairs - 1, "a fit needs at least 30"},
		{"heard only on another band", func() *survey.Floor {
			return calibratedFloor(func(_, _ int, rssi int) []*wifi.ScannedNetwork {
				return []*wifi.ScannedNetwork{{BSSID: calBSSID, Signal: rssi, Channel: 6, Frequency: 2437}}
			})
		}, 0, "0 readings on 5 GHz"},
		{"every reading at one distance", func() *survey.Floor {
			f := calibratedFloor(on5GHz)
			for _, sp := range f.Samples {
				sp.X, sp.Y = 250, 150
			}
			return f
		}, 15 * 20, "same distance"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			floor := tc.floor()
			result, err := survey.GenerateFloorHeatmap(floor, predictedConfig("5 GHz"))
			if err != nil {
				t.Fatalf("GenerateFloorHeatmap: %v", err)
			}
			c := result.Calibrations[0]
			if c.Calibrated || !strings.Contains(c.Fallback, tc.reason) || c.Pairs != tc.pairs {
				t.Errorf("calibration = %+v, want a fallback naming %q over %d pairs", c, tc.reason, tc.pairs)
			}
			if c.Exponent != rf.DefaultExponent {
				t.Errorf("drawn exponent = %v, want the default %v", c.Exponent, rf.DefaultExponent)
			}
			// Drawn uncalibrated: 10 m east is 17 dBm − FSPL(5500, 1 m) − 30.
			if got, want := result.Grid[15][30], 17-rf.FreeSpaceLossDB(5500)-30; math.Abs(got-want) > 0.1 {
				t.Errorf("cell 10 m east = %.2f dBm, want the uncalibrated %.2f", got, want)
			}
		})
	}
}

func TestClampedFitIsDrawnAtTheBound(t *testing.T) {
	// The signal rises with distance: no physical exponent explains it.
	var pairs []rf.Sample
	floor := calibratedFloor(func(x, y int, _ int) []*wifi.ScannedNetwork {
		d := math.Hypot(float64(x-205), float64(y-155)) * 0.1
		pairs = append(pairs, rf.Sample{DistanceM: d, RSSIDBm: float64(int(-80 + d))})
		return []*wifi.ScannedNetwork{{BSSID: calBSSID, Signal: int(-80 + d), Channel: 36, Frequency: 5180}}
	})
	result, err := survey.GenerateFloorHeatmap(floor, predictedConfig("5 GHz"))
	if err != nil {
		t.Fatalf("GenerateFloorHeatmap: %v", err)
	}
	c := result.Calibrations[0]
	if !c.Clamped || c.FittedExponent == nil || *c.FittedExponent >= 0 {
		t.Errorf("calibration = %+v, want clamped with the negative exponent the walk fits", c)
	}
	if !c.Calibrated || c.Fallback != "" || c.Exponent != rf.MinExponent {
		t.Errorf("calibration = %+v, want drawn calibrated at the bound exponent %v", c, rf.MinExponent)
	}

	// Drawn with the fit's own reference loss at the bound, not the defaults.
	fit, err := rf.Fit(known.TxPowerDBm, pairs)
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if got, want := result.Grid[15][30], fit.Model.PredictRSSI(10); math.Abs(got-want) > 0.3 {
		t.Errorf("cell 10 m east = %.2f dBm, want %.2f from the clamped fit", got, want)
	}
}

func TestMeasuredMapCarriesNoCalibrations(t *testing.T) {
	result, err := survey.GenerateFloorHeatmap(calibratedFloor(on5GHz), survey.DefaultHeatmapConfig())
	if err != nil {
		t.Fatalf("GenerateFloorHeatmap: %v", err)
	}
	if len(result.Calibrations) != 0 {
		t.Errorf("measured map carries %d calibrations", len(result.Calibrations))
	}
}
