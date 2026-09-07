// SPDX-License-Identifier: BUSL-1.1

package rf_test

// g1_corpus_test.go is the Gate G1 measurement. It scores the CPU path-loss
// model against real AirMagnet Survey Pro walks: surveyor-placed APs on one
// side, what the radio actually heard at each position on the other.
//
// The corpus is not committed — this repository is public and those files are
// the vendor's demo projects. Point the test at a directory and run it with
// -v to print the table that docs/11-GATE-G1-RESULT.md records:
//
//	TRELLIS_SVD_CORPUS=~/AirMagnet/DemoProjects go test ./core/rf/ -run G1 -v
//
// What it asserts on every run is narrower than the gate itself: that the
// corpus still joins placements to measurements at all (the BSSID column in an
// AP-placement row runs the address together with the media type, and reading
// it whole silently unjoined every placement in the corpus), and that
// calibration never makes a floor worse.

import (
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/rf"
	"github.com/MustardSeedNetworks/trellis/core/survey"
)

const (
	svdCorpusEnv = "TRELLIS_SVD_CORPUS"
	ampCorpusEnv = "TRELLIS_AMP_CORPUS"
)

// feetPerSurveyUnit converts AirMagnet's plan coordinates to metres.
//
// An .svd carries positions and plan dimensions but no unit for them; the
// project file next to it says only "ScaleUnits=0". Feet is the reading that
// makes the demo floors building-sized. It matters only to the uncalibrated
// number: a constant scale error is absorbed whole by the fitted reference
// loss, which TestFitAbsorbsAUnitErrorIntoTheReferenceLoss pins.
const metresPerSurveyUnit = 0.3048

// defaultExponent is the uncalibrated path-loss exponent: ITU-R P.1238's
// office value, the number a planner would start from with no measurements.
const defaultExponent = 3.0

// defaultTxPowerDBm is used when a placement row carries no power column.
const defaultTxPowerDBm = 20.0

// censoredDBm is AirMagnet's floor. A row at or below it is the scale bottoming
// out, not a reading: one voice walk writes 137 of them, and scoring a model
// against a censored value measures the sentinel, not the propagation.
const censoredDBm = -99

func TestG1PathLossAgainstMeasuredFloors(t *testing.T) {
	root, files := svdCorpus(t)

	var placed []placedAP

	for _, name := range files {
		data, err := fs.ReadFile(root, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		file, err := survey.ParseAirMagnetSVD(data)
		if err != nil {
			continue // planner simulations and header-only exports are refused by design
		}
		if file.Merged {
			// A merge repeats the rows of the walks it was built from; scoring
			// it as well would weight that floor twice.
			continue
		}
		placedAPs := map[string]survey.AirMagnetAP{}
		for _, ap := range file.APs {
			if ap.BSSID != "" {
				placedAPs[strings.ToUpper(ap.BSSID)] = ap
			}
		}
		if len(placedAPs) == 0 {
			continue
		}

		// One sample per (position, placed AP) pair, plus the carrier the
		// measurement itself reported — an AP-placement row's channel column is
		// often zero, and the measurement always knows.
		samples := map[string][]rf.Sample{}
		freqMHz := map[string]float64{}
		censored := map[string]int{}
		for _, p := range file.Points {
			for _, n := range p.Networks {
				key := strings.ToUpper(n.BSSID)
				ap, ok := placedAPs[key]
				if !ok || n.Signal == 0 {
					continue
				}
				if n.Signal <= censoredDBm {
					censored[key]++
					continue
				}
				dx := float64(p.X-ap.X) * metresPerSurveyUnit
				dy := float64(p.Y-ap.Y) * metresPerSurveyUnit
				samples[key] = append(samples[key], rf.Sample{
					DistanceM: math.Hypot(dx, dy),
					RSSIDBm:   float64(n.Signal),
				})
				if n.Frequency > 0 {
					freqMHz[key] = float64(n.Frequency)
				}
			}
		}

		for key, s := range samples {
			ap := placedAPs[key]
			txPower := defaultTxPowerDBm
			if ap.PowerMW > 0 {
				txPower = 10 * math.Log10(float64(ap.PowerMW))
			}
			placed = append(placed, placedAP{
				name:       fmt.Sprintf("%s %s", name, key),
				samples:    s,
				txPowerDBm: txPower,
				carrierMHz: freqMHz[key],
				censored:   censored[key],
			})
		}
	}

	results := scorePlacedAPs(t, placed)
	reportG1(t, results, fmt.Sprintf("readings at or below %d dBm dropped as censored", censoredDBm))
}

// placedAP is one surveyor-placed AP and every reading of it along the walk.
type placedAP struct {
	name       string
	samples    []rf.Sample
	txPowerDBm float64
	carrierMHz float64
	censored   int
}

type floorResult struct {
	name       string
	preCal     rf.Error
	postCal    rf.Error
	exponent   float64
	rawN       float64
	clamped    bool
	censored   int
	medianDist float64
}

// minPairs is the smallest number of (position, AP) pairs that says anything
// about a floor. Below it the fit is describing noise.
const minPairs = 30

// scorePlacedAPs runs the gate's measurement: the uncalibrated model as a
// planner would start from it, then the fit, on every AP with enough pairs.
func scorePlacedAPs(t *testing.T, in []placedAP) []floorResult {
	t.Helper()
	var results []floorResult
	for _, p := range in {
		if len(p.samples) < minPairs {
			continue
		}
		carrier := p.carrierMHz
		if carrier == 0 {
			carrier = 2437
		}
		preCal := rf.Model{
			TxPowerDBm: p.txPowerDBm,
			RefLossDB:  rf.FreeSpaceLossDB(carrier),
			Exponent:   defaultExponent,
		}
		cal, err := rf.Fit(p.txPowerDBm, p.samples)
		if err != nil {
			t.Errorf("%s: fit: %v", p.name, err)
			continue
		}
		preErr := rf.Evaluate(preCal, p.samples)
		postErr := rf.Evaluate(cal.Model, p.samples)
		if !cal.Clamped && postErr.RMSEDB > preErr.RMSEDB+1e-9 {
			t.Errorf("%s: calibration made the floor worse: RMSE %.2f -> %.2f dB",
				p.name, preErr.RMSEDB, postErr.RMSEDB)
		}
		results = append(results, floorResult{
			name:       p.name,
			preCal:     preErr,
			postCal:    postErr,
			exponent:   cal.Model.Exponent,
			rawN:       cal.RawExponent,
			clamped:    cal.Clamped,
			censored:   p.censored,
			medianDist: medianDistance(p.samples),
		})
	}
	if len(results) == 0 {
		t.Fatal("no placed AP in the corpus joined a measurement: the corpus has no ground truth, " +
			"or the AP-placement BSSID no longer matches what the measurements carry")
	}
	return results
}

// reportG1 prints the table docs/11-GATE-G1-RESULT.md records.
func reportG1(t *testing.T, results []floorResult, censoredNote string) {
	t.Helper()
	sort.Slice(results, func(i, j int) bool { return results[i].name < results[j].name })
	t.Logf("%-62s %5s %8s %8s %8s %8s %8s %7s %7s",
		"walk / placed AP", "pairs", "pre-MAE", "pre-bias", "pre-p95", "cal-MAE", "cal-p95", "n", "med-d-m")
	var sumPre, sumPost float64
	clamped, censoredTotal := 0, 0
	for _, r := range results {
		mark := ""
		if r.clamped {
			mark = "*"
			clamped++
			t.Logf("    %s: unbounded fit n = %.2f, pulled back to %.2f",
				r.name, r.rawN, r.exponent)
		}
		t.Logf("%-62s %5d %8.2f %8.2f %8.2f %8.2f %8.2f %6.2f%1s %7.1f",
			r.name, r.postCal.Samples,
			r.preCal.MeanAbsDB, r.preCal.MeanBiasDB, r.preCal.P95AbsDB,
			r.postCal.MeanAbsDB, r.postCal.P95AbsDB,
			r.exponent, mark, r.medianDist)
		sumPre += r.preCal.MeanAbsDB
		sumPost += r.postCal.MeanAbsDB
		censoredTotal += r.censored
	}
	t.Logf("placed APs scored=%d (%d fitted outside the physical exponent bounds, marked *; "+
		"%d %s)  mean pre-calibration MAE=%.2f dB  mean calibrated MAE=%.2f dB",
		len(results), clamped, censoredTotal, censoredNote,
		sumPre/float64(len(results)), sumPost/float64(len(results)))
}

func medianDistance(s []rf.Sample) float64 {
	d := make([]float64, len(s))
	for i, x := range s {
		d[i] = x.DistanceM
	}
	sort.Float64s(d)
	return d[len(d)/2]
}

// svdCorpus and ampCorpus open the corpus directory as a filesystem rooted at
// itself, so every read stays inside the directory the operator named.
func svdCorpus(t *testing.T) (fs.FS, []string) {
	t.Helper()
	return corpus(t, svdCorpusEnv, ".svd")
}

func ampCorpus(t *testing.T) (fs.FS, []string) {
	t.Helper()
	return corpus(t, ampCorpusEnv, ".amp")
}

func corpus(t *testing.T, env, ext string) (fs.FS, []string) {
	t.Helper()
	dir := os.Getenv(env)
	if dir == "" {
		t.Skipf("set %s to a directory of %s files to run this", env, ext)
	}
	if strings.HasPrefix(dir, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, dir[2:])
		}
	}
	root := os.DirFS(dir)
	var files []string
	err := fs.WalkDir(root, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(path), ext) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil || len(files) == 0 {
		t.Fatalf("no %s files under %s (err=%v)", ext, dir, err)
	}
	return root, files
}

// TestG1AirMapperCorpusCarriesAPPlacements guards the ground truth Gate G1
// went looking for. The gate ran on AirMagnet exports because this corpus was
// read as carrying no AP layout at all -- and the tripwire that said so fired
// on 2026-09-07, correctly: the placements were always in the archives, under
// a `.serial` member the importer read from the wrong name. Three archives
// carry 216 of them.
//
// So the assertion is inverted rather than deleted. An importer change that
// drops placements again would otherwise quietly restore the premise the gate
// already recorded as wrong.
func TestG1AirMapperCorpusCarriesAPPlacements(t *testing.T) {
	root, names := ampCorpus(t)

	archives, placements := 0, 0
	for _, name := range names {
		data, err := fs.ReadFile(root, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		file, err := survey.ParseAirMapperFile(data)
		if err != nil {
			continue // an archive this parser refuses carries no placements either
		}
		result, err := file.ToImportResult()
		if err != nil {
			continue
		}
		archives++
		if n := len(result.APLocations); n > 0 {
			placements += n
			t.Logf("%s carries %d AP placement(s) — re-run Gate G1 against it", name, n)
		}
	}
	if archives == 0 {
		t.Fatal("no readable .amp archives in the corpus")
	}
	t.Logf("%d AirMapper archives read, %d AP placements between them", archives, placements)
	if placements == 0 {
		t.Fatalf("no AP placements in %d AirMapper archives: the importer has stopped reading "+
			"them, and Gate G1's ground truth is gone with them", archives)
	}
}
