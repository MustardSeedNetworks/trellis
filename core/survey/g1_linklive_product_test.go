// SPDX-License-Identifier: BUSL-1.1

package survey_test

// g1_linklive_product_test.go runs Gate G1's Link-Live scoring through the
// product: each oracle survey becomes a floor with the surveyor's placements on
// it, and the predicted map calibrates every placed radio from that floor's own
// walk exactly as GetHeatmap does. It records what T-P5 measures against the
// predictive phase's kill criterion (docs/11-GATE-G1-RESULT.md).
//
//	TRELLIS_LL_ORACLE=~/data/linklive-oracle go test ./core/survey/ -run G1LinkLiveProduct -v

import (
	"math"
	"sort"
	"testing"
	"time"

	"github.com/MustardSeedNetworks/trellis/core/internal/linkliveoracle"
	"github.com/MustardSeedNetworks/trellis/core/rf"
	"github.com/MustardSeedNetworks/trellis/core/survey"
	"github.com/MustardSeedNetworks/trellis/core/wifi"
)

// killCriterionDB is the predictive phase's stop line: a calibrated mean
// error above it through the product path stops the phase after T-P5.
const killCriterionDB = 6.0

var predictionBands = []string{"2.4 GHz", "5 GHz", "6 GHz"}

func bandOf(mhz int) string {
	switch {
	case mhz >= 2400 && mhz < 2500:
		return "2.4 GHz"
	case mhz >= 5000 && mhz < 5925:
		return "5 GHz"
	case mhz >= 5925:
		return "6 GHz"
	}
	return ""
}

// oracleFloor builds the floor the product would hold for a Link-Live survey:
// the walk as passive samples, and one placement per radio of each
// surveyor-named AP, at the AP's position and carrying that radio's BSSID and
// band. A placement in this product is one radio, so a three-radio AP is three
// placements at one spot, which is also how Gate G1 scored them.
func oracleFloor(s linkliveoracle.Survey) *survey.Floor {
	named := map[string]linkliveoracle.Placement{}
	for _, p := range s.APLocations {
		if p.SurveyorNamed() {
			named[p.Label] = p
		}
	}
	floor := &survey.Floor{
		ID:        s.ID,
		FloorPlan: &survey.FloorPlan{Width: s.WidthPx, Height: s.HeightPx, ScaleM: s.MetresPerPixel()},
	}
	placed := map[string]bool{}
	for _, pt := range s.Points {
		sample := &survey.PassiveSample{}
		for _, o := range pt.APs {
			mhz := o.CarrierMHz()
			if o.Signal == nil || bandOf(mhz) == "" {
				continue
			}
			bssid := linkliveoracle.NormalizeBSSID(o.BSSID)
			sample.Networks = append(sample.Networks, &wifi.ScannedNetwork{BSSID: bssid, Signal: *o.Signal, Frequency: mhz})
			if ap, ok := named[o.APName]; ok && !placed[bssid] {
				placed[bssid] = true
				floor.APLocations = append(floor.APLocations, survey.APLocation{
					ID: o.APName + " " + bssid, Label: o.APName, BSSID: bssid,
					X: int(math.Round(ap.X)), Y: int(math.Round(ap.Y)), Band: bandOf(mhz),
				})
			}
		}
		sample.CalculateAggregations()
		floor.Samples = append(floor.Samples, &survey.SamplePoint{
			X: int(math.Round(pt.X)), Y: int(math.Round(pt.Y)), Timestamp: time.Now(), SampleData: sample,
		})
	}
	return floor
}

func TestG1LinkLiveProductPath(t *testing.T) {
	dir, ok := linkliveoracle.Dir()
	if !ok {
		t.Skipf("set %s to the directory scripts/linklive-fetch-oracle.sh wrote", linkliveoracle.Env)
	}
	surveys, err := linkliveoracle.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(surveys, func(i, j int) bool { return surveys[i].ID < surveys[j].ID })

	// drawn is every radio with enough readings to fit, the set Gate G1
	// scored, at the error of the model the map was actually drawn with.
	// unclamped is the subset whose fitted exponent was physical.
	var drawn, unclamped []float64
	for _, s := range surveys {
		if s.MetresPerPixel() == 0 || s.WidthPx <= 0 || s.HeightPx <= 0 {
			t.Logf("%s %s: no scale or plan size stated; not scored", s.ID[:8], s.FileName)
			continue
		}
		floor := oracleFloor(s)
		var sDrawn, sUnclamped []float64
		for _, band := range predictionBands {
			result, err := survey.GenerateFloorHeatmap(floor, predictedConfig(band))
			if err != nil {
				continue // no surveyor-named radio on this band
			}
			for _, c := range result.Calibrations {
				if c.Pairs < rf.MinFitPairs || c.MeanAbsErrorDB == nil {
					continue
				}
				sDrawn = append(sDrawn, *c.MeanAbsErrorDB)
				if !c.Clamped {
					sUnclamped = append(sUnclamped, *c.MeanAbsErrorDB)
				}
			}
		}
		if len(sDrawn) == 0 {
			continue
		}
		t.Logf("survey %s: %3d radios, %3d clamped: drawn-map MAE=%5.2f dB, unclamped MAE=%5.2f dB",
			s.ID[:8], len(sDrawn), len(sDrawn)-len(sUnclamped), mean(sDrawn), mean(sUnclamped))
		drawn = append(drawn, sDrawn...)
		unclamped = append(unclamped, sUnclamped...)
	}
	if len(drawn) == 0 {
		t.Fatal("no radio on any survey had enough readings to score")
	}
	t.Logf("all: %d radios, %d clamped: drawn-map MAE=%.2f dB, unclamped MAE=%.2f dB; kill criterion %.0f dB",
		len(drawn), len(drawn)-len(unclamped), mean(drawn), mean(unclamped), killCriterionDB)
	if m := mean(drawn); m > killCriterionDB {
		t.Errorf("calibrated MAE %.2f dB is above the %.0f dB kill criterion: stop the predictive phase after T-P5", m, killCriterionDB)
	}
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return math.NaN()
	}
	var sum float64
	for _, x := range v {
		sum += x
	}
	return sum / float64(len(v))
}
