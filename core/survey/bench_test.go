// SPDX-License-Identifier: BUSL-1.1

package survey_test

// The PRD's ingest budget is at least 1,000 measurement points per second, and
// its scale target is 100k+ survey points. These benchmarks measure the import
// path an operator actually uses for a finished walk -- an AirMapper archive,
// decoded and stored -- at 1k, 10k and 100k points. docs/13-PERFORMANCE.md
// records the results against the budgets.

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/png"
	"testing"
)

// Synthetic floor: a 2000×1500 px plan, walked on an even grid, with the same
// number of APs heard at every point. Ten per point keeps the 100k archive
// under the parser's 64 MiB entry cap; the reference corpus averages more per
// point over far fewer points.
const (
	benchPlanW     = 2000
	benchPlanH     = 1500
	benchAPs       = 40
	benchHeardPerP = 10
)

var benchPointCounts = []uint64{1_000, 10_000, 100_000}

// benchAMP builds an AirMapper archive holding a floor-plan PNG and a
// .SurveyResult of the given number of passive walk points.
func benchAMP(tb testing.TB, points uint64) []byte {
	tb.Helper()

	var plan bytes.Buffer
	if err := png.Encode(&plan, image.NewGray(image.Rect(0, 0, benchPlanW, benchPlanH))); err != nil {
		tb.Fatalf("encode plan: %v", err)
	}

	var result []byte
	for i := range points {
		x, y := benchPosition(i, points)
		parts := [][]byte{
			varintField(tagPointX, x),
			varintField(tagPointY, y),
			varintField(tagPointTime, refMillis+i*1000),
		}
		for j := range uint64(benchHeardPerP) {
			ap := (i + j*7) % benchAPs
			parts = append(parts, observation(
				fmt.Sprintf("aa:bb:cc:00:00:%02x", ap), "corp", []uint64{1, 6, 11, 36, 149}[ap%5],
				negDBm(40+(i+ap*13)%50), negDBm(95), refMillis+i*1000,
			))
		}
		result = append(result, point(parts...)...)
	}

	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	for _, entry := range []struct {
		name string
		data []byte
	}{{"floorplan.png", plan.Bytes()}, {"survey.SurveyResult", result}} {
		w, err := zw.Create(entry.name)
		if err != nil {
			tb.Fatalf("zip create %s: %v", entry.name, err)
		}
		if _, err := w.Write(entry.data); err != nil {
			tb.Fatalf("zip write %s: %v", entry.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		tb.Fatalf("zip close: %v", err)
	}
	return archive.Bytes()
}

// benchPosition spreads n points over the plan on a near-square grid.
func benchPosition(i, n uint64) (uint64, uint64) {
	cols := uint64(1)
	for cols*cols*benchPlanH < n*benchPlanW {
		cols++
	}
	rows := (n + cols - 1) / cols
	return (i%cols)*benchPlanW/cols + 1, (i/cols)*benchPlanH/rows + 1
}

func BenchmarkImportAirMapper(b *testing.B) {
	for _, n := range benchPointCounts {
		b.Run(fmt.Sprintf("points=%d", n), func(b *testing.B) {
			data := benchAMP(b, n)
			mgr := mustManager(b, b.TempDir(), nil, nil, nil, nil)
			var imports uint64
			for b.Loop() {
				svy, err := mgr.ImportAirMapper("bench", data)
				if err != nil {
					b.Fatalf("ImportAirMapper: %v", err)
				}
				if got := uint64(len(svy.GetActiveFloor().Samples)); got != n {
					b.Fatalf("imported %d points, want %d", got, n)
				}
				imports++
			}
			b.ReportMetric(float64(n*imports)/b.Elapsed().Seconds(), "points/s")
		})
	}
}
