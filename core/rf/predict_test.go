package rf_test

import (
	"math"
	"testing"

	"github.com/MustardSeedNetworks/trellis/core/rf"
)

// spread returns the i-th point of an additive-recurrence sequence in [0, 1)²:
// deterministic, and spread evenly enough that APs do not bunch.
func spread(i int) (float64, float64) {
	_, x := math.Modf(float64(i+1) * 0.6180339887)
	_, y := math.Modf(float64(i+1) * 0.7548776662)
	return x, y
}

func defaultModel(txPowerDBm float64) rf.Model {
	return rf.Model{
		TxPowerDBm: txPowerDBm,
		RefLossDB:  rf.FreeSpaceLossDB(5500),
		Exponent:   rf.DefaultExponent,
	}
}

// The numbers are worked by hand from ADR-0011's formula, not read back from
// the code: FSPL(5500 MHz, 1 m) = 20·log10(5500) − 27.55 = 47.257 dB.
func TestPredictGridGolden(t *testing.T) {
	// 11×11 one-metre cells; the AP sits on the centre of cell (5, 5).
	txs := []rf.Transmitter{{XM: 5.5, YM: 5.5, Model: defaultModel(rf.DefaultTxPowerDBm)}}
	grid := rf.PredictGrid(txs, 11, 11, 1)

	for _, tc := range []struct {
		name     string
		row, col int
		want     float64
	}{
		// Under the AP the distance floors at the 1 m reference: 10 − 47.257.
		{"centre cell", 5, 5, -37.257},
		// Five cells east is 5 m: −37.257 − 30·log10(5).
		{"5 m east", 5, 10, -58.226},
	} {
		if got := grid[tc.row][tc.col]; math.Abs(got-tc.want) > 0.1 {
			t.Errorf("%s = %.3f dBm, want %.3f within 0.1 dB", tc.name, got, tc.want)
		}
	}
}

func TestPredictGridNeverRisesWithDistanceFromOneAP(t *testing.T) {
	const cols, rows, cell = 40, 30, 0.5
	ap := rf.Transmitter{XM: 7.3, YM: 4.1, Model: defaultModel(14)}
	grid := rf.PredictGrid([]rf.Transmitter{ap}, cols, rows, cell)

	type cellAt struct{ d, v float64 }
	cells := make([]cellAt, 0, cols*rows)
	for row := range rows {
		for col := range cols {
			x, y := (float64(col)+0.5)*cell, (float64(row)+0.5)*cell
			cells = append(cells, cellAt{math.Hypot(x-ap.XM, y-ap.YM), grid[row][col]})
		}
	}
	for i, a := range cells {
		for _, b := range cells[i+1:] {
			if (b.d > a.d && b.v > a.v) || (a.d > b.d && a.v > b.v) {
				t.Fatalf("a cell %.2f m away reads %.2f dBm, above one %.2f m away at %.2f dBm",
					math.Max(a.d, b.d), math.Max(a.v, b.v), math.Min(a.d, b.d), math.Min(a.v, b.v))
			}
		}
	}
}

func TestPredictGridAddingAnAPNeverLowersACell(t *testing.T) {
	const cols, rows, cell = 50, 40, 0.5
	var txs []rf.Transmitter
	before := rf.PredictGrid(txs, cols, rows, cell)
	for i := range 12 {
		x, y := spread(i)
		txs = append(txs, rf.Transmitter{
			XM:    x * cols * cell,
			YM:    y * rows * cell,
			Model: defaultModel(5 + float64(i%4)*5),
		})
		after := rf.PredictGrid(txs, cols, rows, cell)
		for row := range rows {
			for col := range cols {
				if after[row][col] < before[row][col] {
					t.Fatalf("AP %d lowered cell (%d, %d) from %.2f to %.2f dBm",
						len(txs), row, col, before[row][col], after[row][col])
				}
			}
		}
		before = after
	}
}

// BenchmarkPredictGrid is the PRD's AP-move budget: a 5,000 m² floor (100 m ×
// 50 m) at 0.25 m cells with 20 APs, under 100 ms. docs/13-PERFORMANCE.md
// records the result.
func BenchmarkPredictGrid(b *testing.B) {
	const cols, rows, cell = 400, 200, 0.25
	txs := make([]rf.Transmitter, 20)
	for i := range txs {
		x, y := spread(i)
		txs[i] = rf.Transmitter{
			XM:    x * cols * cell,
			YM:    y * rows * cell,
			Model: defaultModel(rf.DefaultTxPowerDBm),
		}
	}
	for b.Loop() {
		rf.PredictGrid(txs, cols, rows, cell)
	}
}
