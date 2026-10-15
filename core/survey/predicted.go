package survey

import (
	"errors"
	"fmt"

	"github.com/MustardSeedNetworks/trellis/core/rf"
)

// A prediction is refused rather than drawn empty when the floor cannot carry
// one (ADR-0011): without a scale a pixel is no distance, and without a
// placement there is nothing to predict from.
var (
	ErrFloorUnscaled = errors.New("floor plan has no scale (metres per pixel); set one before predicting")
	ErrNoPlacements  = errors.New("floor has no AP placed on this band")
	ErrUnknownBand   = errors.New(`band must be "2.4 GHz", "5 GHz" or "6 GHz"`)
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
// this floor uses.
func generatePredictedHeatmap(floor *Floor, config HeatmapConfig) (*HeatmapResult, error) {
	carrier, ok := bandCentreMHz[config.Band]
	if !ok {
		return nil, fmt.Errorf("%w, not %q", ErrUnknownBand, config.Band)
	}
	plan := floor.FloorPlan
	if plan == nil || plan.Width <= 0 || plan.Height <= 0 || plan.ScaleM <= 0 {
		return nil, ErrFloorUnscaled
	}

	refLoss := rf.FreeSpaceLossDB(carrier)
	var txs []rf.Transmitter
	for _, ap := range floor.APLocations {
		// An AP whose band was not stated is taken to radiate on every band:
		// dropping it would draw its coverage as a hole.
		if ap.Band != "" && ap.Band != config.Band {
			continue
		}
		power := rf.DefaultTxPowerDBm
		if ap.TxPowerDBm != nil {
			power = *ap.TxPowerDBm
		}
		txs = append(txs, rf.Transmitter{
			XM: float64(ap.X) * plan.ScaleM,
			YM: float64(ap.Y) * plan.ScaleM,
			Model: rf.Model{
				TxPowerDBm: power,
				RefLossDB:  refLoss,
				Exponent:   rf.DefaultExponent,
			},
		})
	}
	if len(txs) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNoPlacements, config.Band)
	}

	config = withHeatmapDefaults(config)
	cols := (plan.Width + config.CellSize - 1) / config.CellSize
	rows := (plan.Height + config.CellSize - 1) / config.CellSize
	grid := rf.PredictGrid(txs, cols, rows, float64(config.CellSize)*plan.ScaleM)
	return paintHeatmap(grid, nil, plan, plan.Width, plan.Height, config)
}
