package survey

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/MustardSeedNetworks/trellis/core/rf"
)

// A prediction is refused rather than drawn empty when the floor cannot carry
// one (ADR-0011): without a scale a pixel is no distance, and without a
// placement there is nothing to predict from.
var (
	ErrFloorUnscaled = errors.New("floor plan has no scale (metres per pixel); set one before predicting")
	ErrNoPlacements  = errors.New("floor has no AP placed on this band")
	ErrUnknownBand   = errors.New(`band must be "2.4 GHz", "5 GHz" or "6 GHz"`)
	// ErrNothingToCompare is a delta with no cell holding both sides: no
	// walked point heard a placed AP's BSSID on the band.
	ErrNothingToCompare = errors.New("no walked point heard a placed AP's BSSID")
)

// bandCentreMHz is the carrier a band's free-space reference loss is taken at:
// the middle of each band's channel plan. Across a band the reference loss
// moves by under 1 dB, well inside the model's error.
var bandCentreMHz = map[string]float64{
	"2.4 GHz": 2442,
	"5 GHz":   5500,
	"6 GHz":   6525,
}

// generatePredictedHeatmap draws the best-server signal predicted from the
// floor's placed APs on config.Band, over the same grid the measured map of
// this floor uses. Each AP is calibrated to the floor's own walk where the
// walk heard it enough to fit; the result says, per AP, which happened.
func generatePredictedHeatmap(floor *Floor, config HeatmapConfig) (*HeatmapResult, error) {
	config = withHeatmapDefaults(config)
	grid, calibrations, err := predictGrid(floor, config, func(APLocation) bool { return true })
	if err != nil {
		return nil, err
	}
	plan := floor.FloorPlan
	result, err := paintHeatmap(grid, nil, plan, plan.Width, plan.Height, config)
	if err != nil {
		return nil, err
	}
	result.Calibrations = calibrations
	return result, nil
}

// predictGrid is the prediction both predicted layers paint: the placed APs
// on config.Band that keep accepts, each calibrated to the walk where it can
// be, as best-server dBm over the measured map's grid.
func predictGrid(floor *Floor, config HeatmapConfig, keep func(APLocation) bool) ([][]float64, []APCalibration, error) {
	carrier, ok := bandCentreMHz[config.Band]
	if !ok {
		return nil, nil, fmt.Errorf("%w, not %q", ErrUnknownBand, config.Band)
	}
	plan := floor.FloorPlan
	if plan == nil || plan.Width <= 0 || plan.Height <= 0 || plan.ScaleM <= 0 {
		return nil, nil, ErrFloorUnscaled
	}

	refLoss := rf.FreeSpaceLossDB(carrier)
	var txs []rf.Transmitter
	var calibrations []APCalibration
	for _, ap := range floor.APLocations {
		// An AP whose band was not stated is taken to radiate on every band:
		// dropping it would draw its coverage as a hole.
		if (ap.Band != "" && ap.Band != config.Band) || !keep(ap) {
			continue
		}
		power := rf.DefaultTxPowerDBm
		if ap.TxPowerDBm != nil {
			power = *ap.TxPowerDBm
		}
		model, calibration := calibrate(ap, floor.Samples, plan.ScaleM, config.Band, rf.Model{
			TxPowerDBm: power,
			RefLossDB:  refLoss,
			Exponent:   rf.DefaultExponent,
		})
		calibrations = append(calibrations, calibration)
		txs = append(txs, rf.Transmitter{
			XM:    float64(ap.X) * plan.ScaleM,
			YM:    float64(ap.Y) * plan.ScaleM,
			Model: model,
		})
	}
	if len(txs) == 0 {
		return nil, nil, fmt.Errorf("%w: %s", ErrNoPlacements, config.Band)
	}

	cols := (plan.Width + config.CellSize - 1) / config.CellSize
	rows := (plan.Height + config.CellSize - 1) / config.CellSize
	return rf.PredictGrid(txs, cols, rows, float64(config.CellSize)*plan.ScaleM), calibrations, nil
}

// PredictionError is how far a prediction sits from the walk, over the cells
// of a predicted_delta map that hold both.
type PredictionError struct {
	Cells     int     `json:"cells"`
	MeanAbsDB float64 `json:"meanAbsDb"`
	P95AbsDB  float64 `json:"p95AbsDb"`
}

// generatePredictedDeltaHeatmap draws, per cell, the prediction minus what
// the walk heard there: where the plan promises more than the floor delivers,
// and by how much. Only placements with a BSSID take part, on both sides: a
// placement without one cannot be told apart in the walk, so neither its
// prediction nor anyone's readings of it can be compared.
func generatePredictedDeltaHeatmap(floor *Floor, config HeatmapConfig) (*HeatmapResult, error) {
	config = withHeatmapDefaults(config)
	placed := map[string]bool{}
	grid, calibrations, err := predictGrid(floor, config, func(ap APLocation) bool {
		if ap.BSSID == "" {
			return false
		}
		placed[strings.ToLower(ap.BSSID)] = true
		return true
	})
	if err != nil {
		return nil, err
	}

	walked, samples := walkedGrid(floor.MeasuredSamples(), placed, config.Band, len(grid[0]), len(grid), config.CellSize)
	delta := predictionDelta(grid, walked)
	summary, ok := summariseDelta(delta)
	if !ok {
		return nil, fmt.Errorf("%w on %s", ErrNothingToCompare, config.Band)
	}

	plan := floor.FloorPlan
	result, err := paintHeatmap(delta, samples, plan, plan.Width, plan.Height, config)
	if err != nil {
		return nil, err
	}
	result.Calibrations = calibrations
	result.PredictionError = &summary
	return result, nil
}

// walkedGrid averages, per cell, the strongest reading of any placed BSSID on
// band at each walked point in it; a cell no such point fell in is NaN. The
// points themselves are returned so the map can mark where the walk went.
func walkedGrid(
	points []*SamplePoint, placed map[string]bool, band string, cols, rows, cellSize int,
) ([][]float64, []SampleValue) {
	sums := make([][]float64, rows)
	counts := make([][]int, rows)
	for row := range rows {
		sums[row] = make([]float64, cols)
		counts[row] = make([]int, cols)
	}

	var samples []SampleValue
	for _, sp := range points {
		col, row := sp.X/cellSize, sp.Y/cellSize
		if sp.X < 0 || sp.Y < 0 || col >= cols || row >= rows {
			continue
		}
		heard, ok := strongestHeard(sp, band, func(bssid string) bool { return placed[strings.ToLower(bssid)] })
		if !ok {
			continue
		}
		sums[row][col] += heard
		counts[row][col]++
		samples = append(samples, SampleValue{Point: Point2D{X: float64(sp.X), Y: float64(sp.Y)}, Value: heard})
	}

	for row := range rows {
		for col := range cols {
			if counts[row][col] == 0 {
				sums[row][col] = math.NaN()
				continue
			}
			sums[row][col] /= float64(counts[row][col])
		}
	}
	return sums, samples
}

// predictionDelta is predicted minus measured per cell, NaN wherever either
// side has no value. The two grids share one shape by construction.
func predictionDelta(predicted, measured [][]float64) [][]float64 {
	delta := make([][]float64, len(predicted))
	for row, cells := range predicted {
		walked := measured[row]
		delta[row] = make([]float64, len(cells))
		for col, p := range cells {
			// NaN propagates through the subtraction, which is the rule.
			delta[row][col] = p - walked[col]
		}
	}
	return delta
}

// summariseDelta is the mean and 95th-percentile (nearest rank) absolute
// error over the cells the delta is defined on; false when there are none.
func summariseDelta(delta [][]float64) (PredictionError, bool) {
	var errs []float64
	for _, row := range delta {
		for _, d := range row {
			if !math.IsNaN(d) {
				errs = append(errs, math.Abs(d))
			}
		}
	}
	if len(errs) == 0 {
		return PredictionError{}, false
	}
	slices.Sort(errs)
	var sum float64
	for _, e := range errs {
		sum += e
	}
	rank := int(math.Ceil(0.95*float64(len(errs)))) - 1
	return PredictionError{Cells: len(errs), MeanAbsDB: sum / float64(len(errs)), P95AbsDB: errs[rank]}, true
}
