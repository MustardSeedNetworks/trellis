// SPDX-License-Identifier: BUSL-1.1

// Command walk-compare scores a Trellis walk against an AirMapper walk of the
// same floor: the comparison T-KILL's alpha kill criterion asks for.
//
// Both walks must be drawn on the same floor-plan image, so a pixel on one is
// the same spot on the other; docs/14-T-KILL-WALK.md is the procedure that
// makes that true. Each Trellis point is paired with the nearest AirMapper
// point within pairRadiusM, and every BSSID both heard there is one pair. The
// report states, per band, how far Trellis reads from AirMapper and how often
// it missed an AP AirMapper heard clearly. It reports; the pass bar is the
// owner's (docs/14-T-KILL-WALK.md, "Reading the result").
//
// The store is opened in place under the daemon's instance lock, so it must be
// a copy, or trellisd must be stopped. Usage:
//
//	walk-compare -amp floor.amp -store ~/Trellis-Corpus/t-kill/store -survey "T-KILL floor 3"
package main

import (
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/MustardSeedNetworks/foundation/pkg/instance"
	"github.com/MustardSeedNetworks/trellis/core/survey"
)

const (
	// pairRadiusM is how far apart two readings may be and still count as the
	// same spot. A walk at about 1.2 m/s yields a Linux point every ~3 s, so
	// consecutive Trellis points are 3-4 m apart; 2 m pairs each with at most
	// its own stretch of the AirMapper path.
	pairRadiusM = 2.0

	// clearSignalDBm is the AirMapper reading above which a BSSID Trellis did
	// not report at the same spot counts as missed. Weaker readings drop in
	// and out of any scan and say nothing about the capture path.
	clearSignalDBm = -75
)

func main() {
	amp := flag.String("amp", "", "AirMapper archive (.amp) of the floor")
	store := flag.String("store", "", "copy of the trellisd data directory holding the Trellis walk")
	name := flag.String("survey", "", "name or ID of the Trellis walk in that store")
	flag.Parse()
	if *amp == "" || *store == "" || *name == "" {
		flag.Usage()
		os.Exit(2)
	}

	report, err := run(*amp, *store, *name)
	if err != nil {
		fmt.Fprintln(os.Stderr, "walk-compare:", err)
		os.Exit(1)
	}
	fmt.Print(report)
}

func run(ampPath, storeDir, name string) (string, error) {
	raw, err := os.ReadFile(ampPath)
	if err != nil {
		return "", err
	}
	// The archive is imported into a throwaway store, never the walk's.
	refDir, err := os.MkdirTemp("", "walk-compare-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(refDir) }()
	refMgr, err := survey.NewManager(refDir, nil, nil, nil, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = refMgr.Close() }()
	ref, err := refMgr.ImportAirMapper("reference", raw)
	if err != nil {
		return "", err
	}

	lock, err := instance.Acquire(storeDir)
	if err != nil {
		return "", fmt.Errorf("store is in use (stop trellisd or copy it): %w", err)
	}
	defer func() { _ = lock.Release() }()
	mgr, err := survey.NewManager(storeDir, nil, nil, nil, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = mgr.Close() }()
	if err := mgr.LoadSurveys(); err != nil {
		return "", err
	}
	walk, err := findSurvey(mgr, name)
	if err != nil {
		return "", err
	}

	result, err := compare(ref.GetActiveFloor(), walk.GetActiveFloor())
	if err != nil {
		return "", err
	}
	return result.String(), nil
}

func findSurvey(mgr *survey.Manager, name string) (*survey.Survey, error) {
	var found []*survey.Survey
	for _, s := range mgr.ListSurveys() {
		if s.ID == name || s.Name == name {
			found = append(found, s)
		}
	}
	switch len(found) {
	case 0:
		return nil, fmt.Errorf("no survey named %q in the store", name)
	case 1:
		return found[0], nil
	default:
		return nil, fmt.Errorf("%d surveys are named %q; pass the ID", len(found), name)
	}
}

// bandStats accumulates one band's pairs.
type bandStats struct {
	diffs  []float64 // Trellis minus AirMapper, dBm
	clear  int       // AirMapper readings at or above clearSignalDBm at a paired spot
	missed int       // of those, ones Trellis did not report
}

type comparison struct {
	walkPoints   int
	pairedPoints int
	bands        map[string]*bandStats
}

func compare(ref, walk *survey.Floor) (*comparison, error) {
	if ref == nil || walk == nil {
		return nil, errors.New("both surveys need a floor")
	}
	rp, wp := ref.FloorPlan, walk.FloorPlan
	if rp == nil || wp == nil {
		return nil, errors.New("both floors need a floor plan")
	}
	// Different images make pixel positions incomparable, and nothing in
	// either survey records how one maps onto the other.
	if rp.Width != wp.Width || rp.Height != wp.Height {
		return nil, fmt.Errorf("floor plans differ: AirMapper %dx%d, Trellis %dx%d; walk both on the same image",
			rp.Width, rp.Height, wp.Width, wp.Height)
	}
	if rp.ScaleM <= 0 {
		return nil, errors.New("the AirMapper floor plan carries no scale")
	}
	radiusPx := pairRadiusM / rp.ScaleM

	refPoints := passivePoints(ref)
	c := &comparison{bands: map[string]*bandStats{}}
	for _, p := range passivePoints(walk) {
		c.walkPoints++
		nearest, ok := nearestWithin(p, refPoints, radiusPx)
		if !ok {
			continue
		}
		c.pairedPoints++
		heard := readings(p.sample)
		for bssid, r := range readings(nearest.sample) {
			b := c.band(r.band)
			w, ok := heard[bssid]
			if ok {
				b.diffs = append(b.diffs, float64(w.signal-r.signal))
			}
			if r.signal >= clearSignalDBm {
				b.clear++
				if !ok {
					b.missed++
				}
			}
		}
	}
	return c, nil
}

func (c *comparison) band(name string) *bandStats {
	b, ok := c.bands[name]
	if !ok {
		b = &bandStats{}
		c.bands[name] = b
	}
	return b
}

type point struct {
	x, y   float64
	sample *survey.PassiveSample
}

func passivePoints(f *survey.Floor) []point {
	var out []point
	for _, s := range f.MeasuredSamples() {
		if ps, ok := s.SampleData.(*survey.PassiveSample); ok {
			out = append(out, point{float64(s.X), float64(s.Y), ps})
		}
	}
	return out
}

func nearestWithin(p point, candidates []point, radius float64) (point, bool) {
	best, bestD := point{}, math.Inf(1)
	for _, q := range candidates {
		if d := math.Hypot(p.x-q.x, p.y-q.y); d <= radius && d < bestD {
			best, bestD = q, d
		}
	}
	return best, !math.IsInf(bestD, 1)
}

type reading struct {
	signal int
	band   string
}

func readings(s *survey.PassiveSample) map[string]reading {
	out := make(map[string]reading, len(s.Networks))
	for _, n := range s.Networks {
		out[strings.ToLower(n.BSSID)] = reading{n.Signal, bandOf(n.Frequency)}
	}
	return out
}

// bandOf uses the same edges as PassiveSample.CalculateAggregations.
func bandOf(mhz int) string {
	switch {
	case mhz >= 2400 && mhz <= 2500:
		return "2.4 GHz"
	case mhz >= 5000 && mhz < 5900:
		return "5 GHz"
	case mhz >= 5900:
		return "6 GHz"
	default:
		return "unknown"
	}
}

// String renders the report. Bias is the adapters' calibration offset; a
// constant offset is not a capture defect, so the error is also given with it
// removed.
func (c *comparison) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Trellis points: %d, paired with an AirMapper point within %.1f m: %d\n\n",
		c.walkPoints, pairRadiusM, c.pairedPoints)
	b.WriteString("| Band | Pairs | Bias dB | MAE dB | MAE less bias dB | p95 less bias dB | Missed (AirMapper >= -75 dBm) |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
	names := make([]string, 0, len(c.bands))
	for n := range c.bands {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		s := c.bands[n]
		missed := "-"
		if s.clear > 0 {
			missed = fmt.Sprintf("%d/%d (%.0f%%)", s.missed, s.clear, 100*float64(s.missed)/float64(s.clear))
		}
		if len(s.diffs) == 0 {
			fmt.Fprintf(&b, "| %s | 0 | - | - | - | - | %s |\n", n, missed)
			continue
		}
		bias := mean(s.diffs)
		abs := make([]float64, len(s.diffs))
		centred := make([]float64, len(s.diffs))
		for i, d := range s.diffs {
			abs[i] = math.Abs(d)
			centred[i] = math.Abs(d - bias)
		}
		fmt.Fprintf(&b, "| %s | %d | %+.2f | %.2f | %.2f | %.2f | %s |\n",
			n, len(s.diffs), bias, mean(abs), mean(centred), p95(centred), missed)
	}
	return b.String()
}

func mean(v []float64) float64 {
	sum := 0.0
	for _, x := range v {
		sum += x
	}
	return sum / float64(len(v))
}

// p95 is the nearest-rank 95th percentile.
func p95(v []float64) float64 {
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	return s[int(math.Ceil(0.95*float64(len(s))))-1]
}
