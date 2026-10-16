package survey

import (
	"math"
	"testing"
)

func TestPredictionDelta(t *testing.T) {
	nan := math.NaN()
	predicted := [][]float64{{-40, -55}, {-70, -82}}
	for _, tc := range []struct {
		name     string
		measured [][]float64
		want     [][]float64
	}{
		{"identical inputs give zero", predicted, [][]float64{{0, 0}, {0, 0}}},
		{
			"a 3 dB weaker walk gives +3",
			[][]float64{{-43, -58}, {-73, -85}},
			[][]float64{{3, 3}, {3, 3}},
		},
		{
			"empty where the walk did not go",
			[][]float64{{-43, nan}, {nan, -85}},
			[][]float64{{3, nan}, {nan, 3}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := predictionDelta(predicted, tc.measured)
			for row := range tc.want {
				for col, want := range tc.want[row] {
					d := got[row][col]
					if math.IsNaN(want) != math.IsNaN(d) || (!math.IsNaN(want) && math.Abs(d-want) > 1e-9) {
						t.Errorf("cell (%d,%d) = %v, want %v", row, col, d, want)
					}
				}
			}
		})
	}

	// The prediction side can be empty too: a cell with no value there has no
	// delta whatever the walk heard.
	if d := predictionDelta([][]float64{{nan}}, [][]float64{{-60}})[0][0]; !math.IsNaN(d) {
		t.Errorf("delta with no prediction = %v, want empty", d)
	}
}

func TestSummariseDelta(t *testing.T) {
	nan := math.NaN()
	// Twenty defined cells, |error| 1..20 dB, half of them negative; the
	// nearest-rank 95th percentile of twenty is the 19th.
	grid := [][]float64{make([]float64, 21)}
	for i := range 20 {
		grid[0][i] = float64(i + 1)
		if i%2 == 0 {
			grid[0][i] = -grid[0][i]
		}
	}
	grid[0][20] = nan

	got, ok := summariseDelta(grid)
	if !ok {
		t.Fatal("summariseDelta found nothing to summarise")
	}
	if got.Cells != 20 || got.MeanAbsDB != 10.5 || got.P95AbsDB != 19 {
		t.Errorf("summary = %+v, want 20 cells, mean 10.5, p95 19", got)
	}

	if _, ok := summariseDelta([][]float64{{nan, nan}}); ok {
		t.Error("an all-empty delta was summarised; want nothing to compare")
	}
}
