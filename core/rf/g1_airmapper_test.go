// SPDX-License-Identifier: BUSL-1.1

package rf_test

// g1_airmapper_test.go is Gate G1 re-measured on the ground truth the gate
// specified and did not get (#362).
//
// The first run (docs/11-GATE-G1-RESULT.md, #359) scored the model against
// AirMagnet demo projects, because the AirMapper reference corpus looked as
// though it carried no AP placements at all. That was a parser bug, found by
// the Link-Live cross-product oracle and fixed in #361: three archives carry
// 216 placements between them. An AirMapper archive is the better ground
// truth on two counts the AirMagnet exports could not offer — it states its
// own metres-per-pixel scale, so no unit has to be assumed, and the placements
// and the walk are in the same pixel space by construction.
//
//	TRELLIS_AMP_CORPUS=~/AirMapper/Survey go test ./core/rf/ -run G1AirMapper -v
//
// Like the AirMagnet measurement, what this asserts on every run is narrower
// than the gate: that the corpus still joins, and that calibration never makes
// a floor worse. The gate's verdict is the number in the log, recorded in the
// result doc by hand.

import (
	"io/fs"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/rf"
	"github.com/MustardSeedNetworks/trellis/core/survey"
)

func TestG1PathLossAgainstAirMapperFloors(t *testing.T) {
	root, names := ampCorpus(t)

	var placed []placedAP
	for _, name := range names {
		data, err := fs.ReadFile(root, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		file, err := survey.ParseAirMapperFile(data)
		if err != nil {
			continue // an archive this parser refuses carries no placements either
		}
		imported, err := file.ToImportResult()
		if err != nil {
			t.Fatalf("%s: ToImportResult: %v", name, err)
		}
		if len(imported.APLocations) == 0 {
			continue
		}
		// No scale, no distances. Assuming one would put invented metres on
		// every pair and the fit would absorb the error into the reference
		// loss, which is exactly how the AirMagnet run lost its uncalibrated
		// number.
		if imported.Calibration.ScaleM <= 0 {
			t.Logf("%s: %d placements but no scale in the archive; not scored",
				name, len(imported.APLocations))
			continue
		}
		points, err := survey.ParseSurveyResult(file.SurveyResult)
		if err != nil {
			t.Fatalf("%s: ParseSurveyResult: %v", name, err)
		}

		locations := map[string]survey.APLocationData{}
		for _, ap := range imported.APLocations {
			// A placement labelled with an AP name rather than an address
			// groups several radios; joining it to one of them would score the
			// wrong radio's readings against that position.
			if ap.BSSID == "" {
				continue
			}
			locations[normalizeBSSID(ap.BSSID)] = ap
		}

		samples := map[string][]rf.Sample{}
		carrier := map[string]float64{}
		for _, p := range points {
			for _, n := range p.Networks {
				key := normalizeBSSID(n.BSSID)
				ap, ok := locations[key]
				if !ok || n.Signal == 0 {
					continue
				}
				dx := (float64(p.X) - ap.X) * imported.Calibration.ScaleM
				dy := (float64(p.Y) - ap.Y) * imported.Calibration.ScaleM
				samples[key] = append(samples[key], rf.Sample{
					DistanceM: math.Hypot(dx, dy),
					RSSIDBm:   float64(n.Signal),
				})
				if n.Frequency > 0 {
					carrier[key] = float64(n.Frequency)
				}
			}
		}

		scored := 0
		for _, s := range samples {
			if len(s) >= minPairs {
				scored++
			}
		}
		// A walk short enough that no AP reaches minPairs contributes nothing,
		// and saying so here is the difference between "not measured" and
		// "measured well". The DIA Main Hall walk is 36 positions: its 98
		// placements join, and not one of them reaches 30 pairs.
		t.Logf("%s: %d placements, %d addressed, %d joined a measurement, %d with >=%d pairs",
			name, len(imported.APLocations), len(locations), len(samples), scored, minPairs)

		for key, s := range samples {
			placed = append(placed, placedAP{
				// AirMapper placements carry no transmit power, so every AP is
				// scored at the same assumed power as an uncalibrated planner
				// would assume.
				name:       filepath.Base(name) + " " + key,
				samples:    s,
				txPowerDBm: defaultTxPowerDBm,
				carrierMHz: carrier[key],
			})
		}
	}

	results := scorePlacedAPs(t, placed)
	reportG1(t, results, "censored readings (AirMapper writes no sentinel floor)")
}

// normalizeBSSID puts an address in one shape. Placements carry it lower-case
// with colons, measurements upper-case, and AirMapper's own labels use dashes.
func normalizeBSSID(s string) string {
	return strings.ToUpper(strings.ReplaceAll(s, "-", ":"))
}
