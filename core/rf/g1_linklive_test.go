// SPDX-License-Identifier: BUSL-1.1

package rf_test

// g1_linklive_test.go is the re-scoped Gate G1 run (docs/11-GATE-G1-RESULT.md,
// "The owner's decision, 2026-09-08"): the model scored against the AP
// placements in Link-Live's own decode of each survey, so the gate no longer
// rests on this repo's reader of the AirMapper archive.
//
// Everything comes from the oracle directory scripts/linklive-fetch-oracle.sh
// writes: heatmap.json states each survey's scale, and each processed record
// carries the walk, what was heard at every position and where the operator
// placed the APs, all in the floor plan's own pixel space. The data is the
// vendor's and is not committed.
//
//	TRELLIS_LL_ORACLE=~/data/linklive-oracle go test ./core/rf/ -run G1LinkLive -v

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/rf"
	"github.com/MustardSeedNetworks/trellis/internal/linkliveoracle"
)

// assumedTxPowersDBm brackets what an AP plausibly transmits. A Link-Live
// placement records no power, so the uncalibrated number is reported across
// this range rather than at one guess.
var assumedTxPowersDBm = []float64{10, 15, 20, 23}

func TestG1LinkLivePathLoss(t *testing.T) {
	dir, ok := linkliveoracle.Dir()
	if !ok {
		t.Skipf("set %s to the directory scripts/linklive-fetch-oracle.sh wrote", linkliveoracle.Env)
	}
	surveys, err := linkliveoracle.Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	var placed []placedAP
	for _, s := range surveys {
		metresPerPixel := s.MetresPerPixel()
		if metresPerPixel == 0 {
			t.Logf("%s %s: no scale stated; not scored", s.ID, s.FileName)
			continue
		}

		// Score only the APs the surveyor placed; each radio is scored on its
		// own.
		placements := map[string]linkliveoracle.Placement{}
		for _, p := range s.APLocations {
			if p.SurveyorNamed() {
				placements[p.Label] = p
			}
		}

		samples := map[string][]rf.Sample{}
		carrier := map[string]float64{}
		for _, pt := range s.Points {
			for _, o := range pt.APs {
				ap, ok := placements[o.APName]
				if !ok || o.Signal == nil {
					continue
				}
				key := o.APName + " " + linkliveoracle.NormalizeBSSID(o.BSSID)
				samples[key] = append(samples[key], rf.Sample{
					DistanceM: math.Hypot(pt.X-ap.X, pt.Y-ap.Y) * metresPerPixel,
					RSSIDBm:   float64(*o.Signal),
				})
				if mhz := o.CarrierMHz(); mhz > 0 {
					carrier[key] = float64(mhz)
				}
			}
		}

		scored := 0
		for _, v := range samples {
			if len(v) >= minPairs {
				scored++
			}
		}
		t.Logf("%s %s (%s): %d points, %d placements, %d surveyor-named, %d radios joined, %d with >=%d pairs",
			s.ID, s.FileName, s.Mode, len(s.Points), len(s.APLocations), len(placements),
			len(samples), scored, minPairs)

		for key, v := range samples {
			placed = append(placed, placedAP{
				name:       fmt.Sprintf("%s %s", s.ID[:8], key),
				samples:    v,
				txPowerDBm: defaultTxPowerDBm,
				carrierMHz: carrier[key],
			})
		}
	}

	results := scorePlacedAPs(t, placed)
	reportG1(t, results, "censored readings (Link-Live omits a missing reading)")
	reportBySurvey(t, results)

	for _, power := range assumedTxPowersDBm {
		var sum float64
		n := 0
		for _, p := range placed {
			if len(p.samples) < minPairs {
				continue
			}
			sum += rf.Evaluate(uncalibrated(power, p.carrierMHz), p.samples).MeanAbsDB
			n++
		}
		t.Logf("assumed transmit power %.0f dBm: mean pre-calibration MAE=%.2f dB over %d placed APs",
			power, sum/float64(n), n)
	}
}

// reportBySurvey prints each survey's mean, so a reader can see which floors
// carry the overall number.
func reportBySurvey(t *testing.T, results []floorResult) {
	t.Helper()
	type acc struct {
		n         int
		pre, post float64
	}
	by := map[string]*acc{}
	for _, r := range results {
		id, _, _ := strings.Cut(r.name, " ")
		a := by[id]
		if a == nil {
			a = &acc{}
			by[id] = a
		}
		a.n++
		a.pre += r.preCal.MeanAbsDB
		a.post += r.postCal.MeanAbsDB
	}
	ids := make([]string, 0, len(by))
	for id := range by {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		a := by[id]
		t.Logf("survey %s: %d placed APs, mean pre-calibration MAE=%.2f dB, mean calibrated MAE=%.2f dB",
			id, a.n, a.pre/float64(a.n), a.post/float64(a.n))
	}
}

func uncalibrated(txPowerDBm, carrier float64) rf.Model {
	if carrier == 0 {
		carrier = 2437
	}
	return rf.Model{
		TxPowerDBm: txPowerDBm,
		RefLossDB:  rf.FreeSpaceLossDB(carrier),
		Exponent:   defaultExponent,
	}
}
