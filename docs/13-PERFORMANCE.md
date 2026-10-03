# Performance against the PRD budgets

**Measured 2026-10-03.** Plan of record row T-C25, issue #682. Import
re-measured the same day after its fix (T-C30, #683).

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
| `GetHeatmap` | 1,000 | 1.26 s | — | < 3 s (see below) | Met |
| `GetHeatmap` | 10,000 | 8.7 s | — | < 3 s (see below) | Miss, #684 |
| `GetHeatmap` | 100,000 | 80.4 s | — | < 3 s (see below) | Miss, #684 |
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
- **Heatmap** time grows linearly with the point count. Each of the
  plan's 30,000 cells (10 px each) weighs every sample by inverse distance,
  on one goroutine, and `math.Pow` takes most of that time (#684).
- **Coverage** never interpolates. It reaches 100k points in 11 ms.
