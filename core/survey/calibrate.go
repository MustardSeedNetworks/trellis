// SPDX-License-Identifier: BUSL-1.1

package survey

import (
	"fmt"
	"math"
	"strings"

	"github.com/MustardSeedNetworks/trellis/core/rf"
)

// APCalibration is how one placed AP's prediction was fitted to the floor's
// own walk, or why it was not and the defaults were drawn instead.
type APCalibration struct {
	PlacementID string `json:"placementId"`
	Label       string `json:"label,omitempty"`
	BSSID       string `json:"bssid,omitempty"`
	// Pairs is how many walked points heard the placement's BSSID on the band.
	Pairs int `json:"pairs"`
	// Exponent is the path-loss exponent the prediction was drawn with.
	Exponent float64 `json:"exponent"`
	// FittedExponent is the least-squares exponent before the physical bounds,
	// set whenever a fit ran: a clamped fit is reported with what it found.
	FittedExponent *float64 `json:"fittedExponent,omitempty"`
	// Clamped says the fit's exponent was pulled back to the nearest physical
	// bound. The AP is still drawn with that fit: its reference loss tracks the
	// walk's level, which the defaults do not.
	Clamped bool `json:"clamped,omitempty"`
	// MeanAbsErrorDB is how far the drawn model sits from the readings it was
	// joined to; nil when there were none to compare against.
	MeanAbsErrorDB *float64 `json:"meanAbsErrorDb,omitempty"`
	Calibrated     bool     `json:"calibrated"`
	// Fallback says why the defaults were drawn; empty when Calibrated.
	Fallback string `json:"fallback,omitempty"`
}

// calibrate fits a placed AP's model to the readings of its BSSID on band
// from the floor's own walk (ADR-0011's calibration step, Gate G1's 3.55 dB).
// It returns the model to draw and the record of how it was chosen; any
// reason no fit can be made draws uncalibrated, the reason given.
//
// The transmit power stays as stated: a fit moves the reference loss and the
// exponent, so a wrong stated power is absorbed by the reference loss rather
// than carried into every cell.
func calibrate(ap APLocation, samples []*SamplePoint, scaleM float64, band string, uncalibrated rf.Model) (rf.Model, APCalibration) {
	report := APCalibration{PlacementID: ap.ID, Label: ap.Label, BSSID: ap.BSSID, Exponent: uncalibrated.Exponent}
	if ap.BSSID == "" {
		report.Fallback = "no BSSID recorded for this placement, so no reading can be joined to it"
		return uncalibrated, report
	}

	pairs := joinReadings(ap, samples, scaleM, band)
	report.Pairs = len(pairs)
	model := fitOrFallback(uncalibrated, pairs, band, &report)
	if len(pairs) > 0 {
		e := rf.Evaluate(model, pairs).MeanAbsDB
		report.MeanAbsErrorDB = &e
	}
	return model, report
}

// fitOrFallback returns the fitted model when a fit can be made, and
// otherwise uncalibrated with the reason recorded on report. A clamped fit is
// drawn at the bound exponent with its own reference loss, which sits far
// closer to the walk than the defaults (docs/11-GATE-G1-RESULT.md).
func fitOrFallback(uncalibrated rf.Model, pairs []rf.Sample, band string, report *APCalibration) rf.Model {
	if len(pairs) < rf.MinFitPairs {
		report.Fallback = fmt.Sprintf("%d readings on %s; a fit needs at least %d", len(pairs), band, rf.MinFitPairs)
		return uncalibrated
	}
	// With MinFitPairs readings in hand the only way a fit fails is that they
	// all sit at one distance.
	fit, err := rf.Fit(uncalibrated.TxPowerDBm, pairs)
	if err != nil {
		report.Fallback = "every reading is at the same distance from the AP; no exponent can be fitted"
		return uncalibrated
	}
	report.FittedExponent = &fit.RawExponent
	report.Clamped = fit.Clamped
	report.Exponent, report.Calibrated = fit.Model.Exponent, true
	return fit.Model
}

// joinReadings pairs each walked point that heard ap's BSSID on band with its
// distance from the AP: the strongest reading of that BSSID there, as the
// per-AP map reads it (Floor.ForBSSID).
func joinReadings(ap APLocation, samples []*SamplePoint, scaleM float64, band string) []rf.Sample {
	var pairs []rf.Sample
	for _, sp := range samples {
		heard, ok := strongestHeard(sp, band, func(bssid string) bool { return strings.EqualFold(bssid, ap.BSSID) })
		if !ok {
			continue
		}
		pairs = append(pairs, rf.Sample{
			DistanceM: math.Hypot(float64(sp.X-ap.X), float64(sp.Y-ap.Y)) * scaleM,
			RSSIDBm:   heard,
		})
	}
	return pairs
}

// strongestHeard is the strongest reading at sp, on band, of any BSSID match
// accepts; false when sp heard none.
func strongestHeard(sp *SamplePoint, band string, match func(bssid string) bool) (float64, bool) {
	ps := getPassiveSampleFromPoint(sp)
	if ps == nil {
		return 0, false
	}
	heard := math.Inf(-1)
	for _, net := range ps.Networks {
		if match(net.BSSID) && bandLabel(net.Frequency, net.Channel) == band {
			heard = math.Max(heard, float64(net.Signal))
		}
	}
	return heard, !math.IsInf(heard, -1)
}
