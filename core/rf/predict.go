package rf

import (
	"math"
	"runtime"
	"sync"
)

// The uncalibrated inputs ADR-0011 fixes: the exponent a floor gets before its
// own walk has calibrated one, and the transmit power assumed for an AP whose
// placement does not state one. 10 dBm is the power Gate G1 scored at.
const (
	DefaultExponent   = 3.0
	DefaultTxPowerDBm = 10.0
)

// Transmitter is one placed AP: where it sits on the floor, in metres from the
// plan's top-left corner, and the model it radiates by.
type Transmitter struct {
	XM, YM float64
	Model  Model
}

// PredictGrid returns the best-server RSSI, in dBm, at the centre of each cell
// of a cols-by-rows grid of cellM-metre cells: the strongest prediction over
// every transmitter. The grid is row-major, laid out as the measured heatmap
// lays out its own, so the two can be painted and compared alike.
//
// With no transmitters every cell is -Inf; callers refuse that case before
// asking.
func PredictGrid(txs []Transmitter, cols, rows int, cellM float64) [][]float64 {
	grid := make([][]float64, rows)
	for row := range grid {
		grid[row] = make([]float64, cols)
	}
	if rows == 0 {
		return grid
	}

	workers := min(runtime.GOMAXPROCS(0), rows)
	var wg sync.WaitGroup
	for w := range workers {
		wg.Go(func() {
			for row := w; row < rows; row += workers {
				y := (float64(row) + 0.5) * cellM
				for col := range cols {
					x := (float64(col) + 0.5) * cellM
					best := math.Inf(-1)
					for _, tx := range txs {
						best = math.Max(best, tx.Model.PredictRSSI(math.Hypot(x-tx.XM, y-tx.YM)))
					}
					grid[row][col] = best
				}
			}
		})
	}
	wg.Wait()

	return grid
}
