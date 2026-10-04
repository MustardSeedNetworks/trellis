# Performance against the PRD budgets

**Measured 2026-10-03.** Plan of record row T-C25, issue #682. Import and
the heatmap were re-measured the same day after their fixes (T-C30, #683;
T-C31, #684). The heatmap was measured again on 2026-10-04 after its
neighbour cutoff (T-C32, #691).

`docs/01-PRD.md` sets an ingest budget of at least 1,000 measurement points/s
and a scale target of 100k+ survey points. This page records what the measured
survey path does at 1k, 10k and 100k points. Each missed budget has its own
issue. Fixing a miss is that issue's work, not this page's.

## How it was measured

```bash
go test -p 1 -run x -bench . -timeout 30m ./core/survey/ ./internal/api/
```

`-p 1` runs the two packages one after the other. With the default
parallelism they compete for the CPU, and the numbers come out slower.
The full run takes about eight minutes.

- `BenchmarkImportAirMapper` (`core/survey/bench_test.go`) imports a
  synthetic AirMapper archive: the archive is decoded, then each point is
  stored through the same path a real import takes.
- `BenchmarkFloorReads` (`internal/api/bench_test.go`) calls `GetHeatmap`
  (RSSI) and `GetCoverage` through the handler the UI uses. The floor is
  built in setup, which the timing excludes.

Both benchmarks use the same synthetic floor: a 2000×1500 px plan walked on
an even grid. Every point hears 10 of 40 APs. Ten per point keeps the
100k-point archive under the parser's 64 MiB entry cap. The reference
AirMapper corpus hears more APs per point, but its walks are far shorter.

Machine: dev-srv-ubuntu, 6 vCPU QEMU guest (x86_64), 16 GB RAM, Ubuntu
26.04, Linux 7.0.0, Go 1.27.1.

## Results

| Path | Points | Time per op | Points/s | PRD budget | Verdict |
| --- | --- | --- | --- | --- | --- |
| Import (decode → store) | 1,000 | 0.22 s | 4,618 | ≥ 1,000 points/s | Met (#683) |
| Import (decode → store) | 10,000 | 1.97 s | 5,086 | ≥ 1,000 points/s | Met (#683) |
| Import (decode → store) | 100,000 | 20.3 s | 4,919 | ≥ 1,000 points/s | Met (#683) |
| `GetHeatmap` | 1,000 | 0.31–0.41 s | — | < 3 s (see below) | Met |
| `GetHeatmap` | 10,000 | 0.56–0.61 s | — | < 3 s (see below) | Met (#684) |
| `GetHeatmap` | 100,000 | 0.86–2.5 s | — | < 3 s (see below) | Met (#691) |
| `GetCoverage` | 1,000 | 0.07 ms | — | < 3 s (see below) | Met |
| `GetCoverage` | 10,000 | 0.57 ms | — | < 3 s (see below) | Met |
| `GetCoverage` | 100,000 | 11.0 ms | — | < 3 s (see below) | Met |

The PRD gives no time budget for a measured heatmap. The nearest one it
states is full-building recompute in under 3 s, and the table applies that.
A 100k-point floor whose map takes more than a minute does not meet the
100k+ scale target in any useful sense.

## What the profiles show

- **Import** runs at a flat rate of about 5,000 points/s, whatever the
  walk's length. It ran at about 600 until #683 found three costs: each
  point committed its own transaction, each observation re-parsed its
  INSERT, and closing the import rewrote every point it had just stored.
  Points are now written 1,000 per transaction through statements prepared
  once, and a status change writes only the survey row. Executing the
  inserts is now about 80% of the profile. Decoding the archive is under 2%.
- **Heatmap** time grew linearly with the point count, because each of the
  plan's 30,000 cells (10 px each) weighed every sample by inverse distance.
  Until #684 that ran on one goroutine, with a square root and a `math.Pow`
  per pair: 1.26 s at 1k, 8.7 s at 10k, 80 s at 100k. #684 took the weights
  from the squared distance and shared the rows across the CPUs, which left
  100k at 2.6 s on a quiet guest and 6.4 s with another build on it. #691
  weighs only each cell's 12 nearest samples (next section): 100k now takes
  0.86 s, and 2.5 s while another driver's test suite held the guest at a
  load of 7–9. Interpolation has dropped out of the profile. What remains
  is serial: PNG encoding, drawing the sample dots and the AP inventory.
- **Coverage** never interpolates. It reaches 100k points in 11 ms.

## The heatmap's neighbour cutoff

Each heatmap cell is the IDW (power 2) of its **12 nearest samples**, found
through a bucket grid over the walk (`core/survey/sampleindex.go`). Twelve
is the usual default for local IDW. A cell sitting on a sample still takes
that sample's value outright. A walk of 12 points or fewer maps exactly as
it did before.

This changes the map: it is no longer the IDW of every sample. At power 2 in
two dimensions, the many small weights of far samples add up, so full IDW
pulls every cell toward the floor's mean and flattens the peak under each AP.
More points do not fix that. `TestInterpolateGrid_NearestCutoffDeviation`
pins the change on a floor whose true field is known: six APs, log-distance
path loss, 1,000 points over the 2000×1500 px plan.

| Comparison (1,000 points, 30,000 cells) | Mean | Max |
| --- | --- | --- |
| 12 nearest vs full IDW | 1.62 dB | 11.9 dB |
| Error vs the true field, 12 nearest | 0.41 dB | — |
| Error vs the true field, full IDW | 1.82 dB | — |

The large deviations are full IDW's error, under the APs. On the same field,
full IDW's mean error stays at 1.4–2.6 dB from 100 to 10,000 points. The
12-nearest error falls from 2.0 dB to 0.09 dB. The test fails if the mean
deviation exceeds 1.75 dB or the maximum exceeds 12.5 dB. It also fails if
the cutoff stops beating full IDW against the truth.
