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
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/rf"
)

const llOracleEnv = "TRELLIS_LL_ORACLE"

// feetToMetres converts the pixels-per-foot scale Link-Live states.
const feetToMetres = 0.3048

// llHeatmapEntry is Link-Live's metadata for one survey.
type llHeatmapEntry struct {
	ID       string  `json:"_id"`
	FileName string  `json:"fileName"`
	ScalePpf float64 `json:"floorPlanScalePpf"`
	Mode     string  `json:"surveyMode"`
}

type llSurvey struct {
	Points      []llPoint     `json:"points"`
	APLocations []llPlacement `json:"apLocations"`
}

type llPlacement struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Label string  `json:"label"`
}

type llPoint struct {
	X   float64      `json:"x"`
	Y   float64      `json:"y"`
	APs []llObserved `json:"aps"`
}

// llObserved is one BSS heard at a position. A record with no signal is a
// position Link-Live holds no reading for, and is not a measurement.
type llObserved struct {
	BSSID   string          `json:"bssid"`
	Signal  *int            `json:"signal"`
	Band    string          `json:"band"`
	Channel json.RawMessage `json:"channelPrimary"`
}

// assumedTxPowersDBm brackets what an AP plausibly transmits. A Link-Live
// placement records no power, so the uncalibrated number is reported across
// this range rather than at one guess.
var assumedTxPowersDBm = []float64{10, 15, 20, 23}

func TestG1LinkLivePathLoss(t *testing.T) {
	dir := os.Getenv(llOracleEnv)
	if dir == "" {
		t.Skipf("set %s to the directory scripts/linklive-fetch-oracle.sh wrote", llOracleEnv)
	}
	if strings.HasPrefix(dir, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, dir[2:])
		}
	}

	// Rooted at the directory the operator named, so a survey id read from
	// heatmap.json cannot reach outside it.
	root := os.DirFS(dir)
	var entries []llHeatmapEntry
	readJSON(t, root, "heatmap.json", &entries)

	var placed []placedAP
	surveys := 0
	for _, e := range entries {
		name := e.ID + ".json.gz"
		if _, err := fs.Stat(root, name); err != nil {
			continue // heatmap.json lists every survey in the org; only fetched ones are scored
		}
		surveys++
		var s llSurvey
		readJSON(t, root, name, &s)
		if e.ScalePpf <= 0 {
			t.Logf("%s %s: no scale stated; not scored", e.ID, e.FileName)
			continue
		}
		metresPerPixel := feetToMetres / e.ScalePpf

		// A label folds the address in ("RuckusWi:58b633-c652d9"). One that
		// names an AP instead groups its radios, and joining it to one of them
		// would score the wrong radio.
		locations := map[string]llPlacement{}
		for _, p := range s.APLocations {
			if key, ok := placementAddress(p.Label); ok {
				locations[key] = p
			}
		}

		samples := map[string][]rf.Sample{}
		carrier := map[string]float64{}
		for _, pt := range s.Points {
			for _, o := range pt.APs {
				key := normalizeLLBSSID(o.BSSID)
				ap, ok := locations[key]
				if !ok || o.Signal == nil {
					continue
				}
				samples[key] = append(samples[key], rf.Sample{
					DistanceM: math.Hypot(pt.X-ap.X, pt.Y-ap.Y) * metresPerPixel,
					RSSIDBm:   float64(*o.Signal),
				})
				if mhz := carrierMHz(o.Band, o.Channel); mhz > 0 {
					carrier[key] = mhz
				}
			}
		}

		scored := 0
		for _, v := range samples {
			if len(v) >= minPairs {
				scored++
			}
		}
		t.Logf("%s %s (%s): %d points, %d placements, %d addressed, %d joined, %d with >=%d pairs",
			e.ID, e.FileName, e.Mode, len(s.Points), len(s.APLocations), len(locations),
			len(samples), scored, minPairs)

		for key, v := range samples {
			placed = append(placed, placedAP{
				name:       fmt.Sprintf("%s %s", e.ID[:8], key),
				samples:    v,
				txPowerDBm: defaultTxPowerDBm,
				carrierMHz: carrier[key],
			})
		}
	}
	if surveys == 0 {
		t.Fatalf("no processed survey under %s matches heatmap.json", dir)
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

// placementAddress returns the BSS address a placement label carries, if any.
func placementAddress(label string) (string, bool) {
	_, address, found := strings.Cut(label, ":")
	if !found || !strings.Contains(address, "-") {
		return "", false
	}
	return normalizeLLBSSID(label), true
}

// normalizeLLBSSID reduces Link-Live's "Vendor:aabbcc-ddeeff" to bare hex.
func normalizeLLBSSID(s string) string {
	if _, address, found := strings.Cut(s, ":"); found && strings.Contains(address, "-") {
		s = address
	}
	return strings.ToLower(strings.NewReplacer(":", "", "-", "").Replace(s))
}

// carrierMHz turns Link-Live's band label and primary channel into a carrier.
// The channel is a number, or a string such as "97E" for a 6 GHz channel.
func carrierMHz(band string, raw json.RawMessage) float64 {
	text := strings.TrimSuffix(strings.Trim(string(raw), `"`), "E")
	ch, err := strconv.Atoi(text)
	if err != nil || ch <= 0 {
		return 0
	}
	switch {
	case strings.HasPrefix(band, "2.4"):
		if ch == 14 {
			return 2484
		}
		return float64(2407 + 5*ch)
	case strings.HasPrefix(band, "5"):
		return float64(5000 + 5*ch)
	case strings.HasPrefix(band, "6"):
		return float64(5950 + 5*ch)
	}
	return 0
}

// readJSON decodes a file the fetch script wrote, gzipped or not. Older
// records are a bare array of points rather than an object.
func readJSON(t *testing.T, root fs.FS, path string, into any) {
	t.Helper()
	body, err := fs.ReadFile(root, path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if bytes.HasPrefix(body, []byte{0x1f, 0x8b}) {
		gz, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if body, err = io.ReadAll(gz); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	if s, ok := into.(*llSurvey); ok && bytes.HasPrefix(bytes.TrimSpace(body), []byte("[")) {
		into = &s.Points
	}
	if err := json.Unmarshal(body, into); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}
