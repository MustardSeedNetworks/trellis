package rf_test

import (
	"errors"
	"math"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/rf"
)

func TestPredictFallsWithDistance(t *testing.T) {
	m := rf.Model{TxPowerDBm: 20, RefLossDB: rf.FreeSpaceLossDB(2437), Exponent: 3}
	prev := math.Inf(1)
	for d := 1.0; d <= 60; d += 0.5 {
		got := m.PredictRSSI(d)
		if got >= prev {
			t.Fatalf("RSSI at %.1f m = %.2f, not below %.2f at the step before", d, got, prev)
		}
		prev = got
	}
}

// A survey point can land on the placed AP. Log-distance path loss is
// undefined at zero, so the model floors the distance rather than returning
// an infinity that would poison every average downstream.
func TestPredictIsFiniteAtZeroDistance(t *testing.T) {
	m := rf.Model{TxPowerDBm: 20, RefLossDB: 40, Exponent: 3}
	got := m.PredictRSSI(0)
	if math.IsInf(got, 0) || math.IsNaN(got) {
		t.Fatalf("RSSI at 0 m = %v, want a finite value", got)
	}
	if want := m.PredictRSSI(rf.MinDistanceM); got != want {
		t.Errorf("RSSI at 0 m = %.2f, want the reference-distance value %.2f", got, want)
	}
}

func TestFreeSpaceLossMatchesTheTextbookValues(t *testing.T) {
	for _, tc := range []struct {
		freqMHz, want float64
	}{
		{2437, 40.2}, // 2.4 GHz, channel 6
		{5180, 46.7}, // 5 GHz, channel 36
	} {
		if got := rf.FreeSpaceLossDB(tc.freqMHz); math.Abs(got-tc.want) > 0.2 {
			t.Errorf("free-space loss at %.0f MHz = %.2f dB, want ~%.1f", tc.freqMHz, got, tc.want)
		}
	}
}

func TestFitRecoversTheModelItWasGeneratedFrom(t *testing.T) {
	truth := rf.Model{TxPowerDBm: 20, RefLossDB: 41, Exponent: 3.2}
	var samples []rf.Sample
	for d := 1.0; d <= 40; d++ {
		samples = append(samples, rf.Sample{DistanceM: d, RSSIDBm: truth.PredictRSSI(d)})
	}
	cal, err := rf.Fit(truth.TxPowerDBm, samples)
	if err != nil {
		t.Fatalf("fit: %v", err)
	}
	if cal.Clamped {
		t.Errorf("fit reported a clamp on a model inside the physical bounds")
	}
	if math.Abs(cal.Model.Exponent-truth.Exponent) > 1e-6 || math.Abs(cal.Model.RefLossDB-truth.RefLossDB) > 1e-6 {
		t.Fatalf("fit = %+v, want %+v", cal.Model, truth)
	}
}

// A walk can fit a rising signal — near-side wall, far-side open floor — and
// the unbounded least-squares line through it has a negative exponent, which
// is not propagation. The fit is pulled back to the physical bound and says
// so, rather than shipping a model that predicts a stronger signal further
// from the AP.
func TestFitClampsAnUnphysicalExponent(t *testing.T) {
	var rising []rf.Sample
	for d := 5.0; d <= 40; d++ {
		rising = append(rising, rf.Sample{DistanceM: d, RSSIDBm: -80 + d})
	}
	cal, err := rf.Fit(20, rising)
	if err != nil {
		t.Fatalf("fit: %v", err)
	}
	if !cal.Clamped {
		t.Errorf("a rising signal fitted without a clamp: n = %.2f", cal.Model.Exponent)
	}
	if cal.RawExponent >= 0 {
		t.Errorf("unbounded exponent = %.2f, want the negative fit the samples describe", cal.RawExponent)
	}
	if cal.Model.Exponent != rf.MinExponent {
		t.Errorf("exponent = %.2f, want the lower bound %.2f", cal.Model.Exponent, rf.MinExponent)
	}
	if got := cal.Model.PredictRSSI(40); got >= cal.Model.PredictRSSI(5) {
		t.Errorf("the clamped model still predicts more signal further away: %.2f at 40 m", got)
	}
}

// The unit the survey coordinates are written in is not recorded in an
// AirMagnet export. This is why that is survivable: a constant scale on every
// distance moves the fitted reference loss and nothing else, so the calibrated
// error — the number Gate G1 turns on — does not depend on the unit.
func TestFitAbsorbsAUnitErrorIntoTheReferenceLoss(t *testing.T) {
	truth := rf.Model{TxPowerDBm: 20, RefLossDB: 41, Exponent: 3.2}
	var metres, feetReadAsMetres []rf.Sample
	for d := 1.0; d <= 40; d++ {
		rssi := truth.PredictRSSI(d)
		metres = append(metres, rf.Sample{DistanceM: d, RSSIDBm: rssi})
		feetReadAsMetres = append(feetReadAsMetres, rf.Sample{DistanceM: d / 0.3048, RSSIDBm: rssi})
	}
	right, err := rf.Fit(truth.TxPowerDBm, metres)
	if err != nil {
		t.Fatalf("fit: %v", err)
	}
	wrong, err := rf.Fit(truth.TxPowerDBm, feetReadAsMetres)
	if err != nil {
		t.Fatalf("fit: %v", err)
	}
	if math.Abs(right.Model.Exponent-wrong.Model.Exponent) > 1e-9 {
		t.Errorf("exponent moved with the unit: %.6f vs %.6f", right.Model.Exponent, wrong.Model.Exponent)
	}
	rightErr := rf.Evaluate(right.Model, metres)
	wrongErr := rf.Evaluate(wrong.Model, feetReadAsMetres)
	if math.Abs(rightErr.MeanAbsDB-wrongErr.MeanAbsDB) > 1e-9 {
		t.Errorf("calibrated error moved with the unit: %.6f vs %.6f dB",
			rightErr.MeanAbsDB, wrongErr.MeanAbsDB)
	}
}

func TestFitRefusesSamplesWithNoSpread(t *testing.T) {
	samples := []rf.Sample{{DistanceM: 5, RSSIDBm: -50}, {DistanceM: 5, RSSIDBm: -60}}
	if _, err := rf.Fit(20, samples); !errors.Is(err, rf.ErrNotEnoughSpread) {
		t.Fatalf("err = %v, want ErrNotEnoughSpread", err)
	}
}

func TestEvaluateReportsAbsoluteErrorAndBiasSeparately(t *testing.T) {
	m := rf.Model{TxPowerDBm: 20, RefLossDB: 40, Exponent: 3}
	// Two samples the model over-reads by 4 dB and two it under-reads by 4:
	// the mean absolute error is 4 dB and the bias cancels to zero.
	var samples []rf.Sample
	for i, offset := range []float64{4, 4, -4, -4} {
		d := float64(5 + i)
		samples = append(samples, rf.Sample{DistanceM: d, RSSIDBm: m.PredictRSSI(d) - offset})
	}
	got := rf.Evaluate(m, samples)
	if math.Abs(got.MeanAbsDB-4) > 1e-9 {
		t.Errorf("mean absolute error = %.3f dB, want 4", got.MeanAbsDB)
	}
	if math.Abs(got.MeanBiasDB) > 1e-9 {
		t.Errorf("bias = %.3f dB, want 0 — the signs cancel", got.MeanBiasDB)
	}
	if got.Samples != 4 {
		t.Errorf("samples = %d, want 4", got.Samples)
	}
	if math.Abs(got.P95AbsDB-4) > 1e-9 {
		t.Errorf("95th percentile = %.3f dB, want 4", got.P95AbsDB)
	}
}
