// Package rf is the CPU path-loss model the Gate G1 spike measures.
//
// It is deliberately small: one log-distance model, one least-squares
// calibration, one error summary. Gate G1 asks whether a closed-form model can
// predict a measured floor closely enough to be worth building a predictive
// engine around; answering that needs a number, not an engine. See
// docs/11-GATE-G1-RESULT.md for the measurement and the verdict.
package rf

import (
	"errors"
	"math"
	"sort"
)

// Model is log-distance path loss with a transmit power:
//
//	RSSI(d) = TxPowerDBm - (RefLossDB + 10*Exponent*log10(d))
//
// RefLossDB is the loss at the 1 m reference distance and Exponent is the
// path-loss exponent n. There is no wall term: a floor plan in this codebase
// is a raster image with no wall geometry, so multi-wall attenuation has
// nothing to sum over. That absence is the point of the gate, not an omission
// to fix later — it is what makes the measured error meaningful.
type Model struct {
	TxPowerDBm float64
	RefLossDB  float64
	Exponent   float64
}

// MinDistanceM floors the distance used for a prediction. Log-distance path
// loss diverges at zero and is not defined inside the reference distance, and
// survey points really do land on top of a placed AP.
const MinDistanceM = 1.0

// PredictRSSI returns the received power in dBm at distanceM metres.
func (m Model) PredictRSSI(distanceM float64) float64 {
	d := math.Max(distanceM, MinDistanceM)
	return m.TxPowerDBm - (m.RefLossDB + 10*m.Exponent*math.Log10(d))
}

// FreeSpaceLossDB is free-space loss at 1 m for a carrier in MHz — the
// physical floor for RefLossDB, and the pre-calibration default.
func FreeSpaceLossDB(frequencyMHz float64) float64 {
	return 20*math.Log10(frequencyMHz) - 27.55
}

// Sample is one measured pair: how far the receiver was from the transmitter,
// and what it heard.
type Sample struct {
	DistanceM float64
	RSSIDBm   float64
}

// ErrNotEnoughSpread is returned by Fit when the samples cannot determine a
// slope: every one of them sits at the same distance, so any exponent
// explains them equally well.
var ErrNotEnoughSpread = errors.New("rf: samples carry no distance spread to fit an exponent against")

// Physical bounds on the path-loss exponent. Free space is 2; indoor offices
// run to about 4 and dense obstruction beyond that. A least-squares line
// through real measurements can land outside this — a walk that happened to
// pass a wall on the near side and open floor on the far side fits a rising
// signal — and a negative exponent is not a propagation model, it is a line
// through noise. Fit clamps to the bounds and says that it did, because that
// is the honest reading: on that floor the model explains nothing.
const (
	MinExponent = 1.5
	MaxExponent = 6.0
)

// Calibration is a fitted model and whether the fit had to be pulled back to
// a physical exponent.
type Calibration struct {
	Model   Model
	Clamped bool
	// RawExponent is the least-squares exponent before the bounds were
	// applied. It is what says how badly a floor fits: a negative one is a
	// line saying the signal grows with distance.
	RawExponent float64
}

// Fit calibrates a model to measurements by least squares on
// RSSI = a - 10n*log10(d), holding the transmit power fixed and moving the
// reference loss and the exponent. This is the "post-calibration" half of
// Gate G1.
//
// A unit error in the distances — feet read as metres — is a constant in
// log-distance and lands entirely in the reference loss, so the fitted
// exponent and the residual error are unaffected by it. Only an uncalibrated
// prediction depends on the unit.
func Fit(txPowerDBm float64, samples []Sample) (Calibration, error) {
	if len(samples) < 2 {
		return Calibration{}, errors.New("rf: need at least two samples to fit")
	}
	var sumX, sumY float64
	xs := make([]float64, len(samples))
	for i, s := range samples {
		xs[i] = math.Log10(math.Max(s.DistanceM, MinDistanceM))
		sumX += xs[i]
		sumY += s.RSSIDBm
	}
	meanX, meanY := sumX/float64(len(samples)), sumY/float64(len(samples))
	var num, den float64
	for i, s := range samples {
		dx := xs[i] - meanX
		num += dx * (s.RSSIDBm - meanY)
		den += dx * dx
	}
	if den == 0 {
		return Calibration{}, ErrNotEnoughSpread
	}
	raw := -(num / den) / 10
	exponent := raw
	clamped := false
	switch {
	case exponent < MinExponent:
		exponent, clamped = MinExponent, true
	case exponent > MaxExponent:
		exponent, clamped = MaxExponent, true
	}
	// With the exponent settled, the reference loss is the mean residual: the
	// least-squares intercept when the slope is held fixed.
	intercept := meanY + 10*exponent*meanX
	return Calibration{
		Model: Model{
			TxPowerDBm: txPowerDBm,
			RefLossDB:  txPowerDBm - intercept,
			Exponent:   exponent,
		},
		Clamped:     clamped,
		RawExponent: raw,
	}, nil
}

// Error summarises how far a model's predictions fall from measurements.
type Error struct {
	Samples    int
	MeanAbsDB  float64
	RMSEDB     float64
	P95AbsDB   float64
	MeanBiasDB float64 // predicted minus measured; positive means optimistic
}

// Evaluate scores a model against measurements.
func Evaluate(m Model, samples []Sample) Error {
	if len(samples) == 0 {
		return Error{}
	}
	abs := make([]float64, len(samples))
	var sumAbs, sumSq, sumErr float64
	for i, s := range samples {
		e := m.PredictRSSI(s.DistanceM) - s.RSSIDBm
		abs[i] = math.Abs(e)
		sumAbs += abs[i]
		sumSq += e * e
		sumErr += e
	}
	sort.Float64s(abs)
	n := float64(len(samples))
	return Error{
		Samples:    len(samples),
		MeanAbsDB:  sumAbs / n,
		RMSEDB:     math.Sqrt(sumSq / n),
		P95AbsDB:   abs[percentileIndex(len(abs), 0.95)],
		MeanBiasDB: sumErr / n,
	}
}

// percentileIndex is the nearest-rank index into a sorted slice of length n.
func percentileIndex(n int, p float64) int {
	i := int(math.Ceil(p*float64(n))) - 1
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}
