# ADR-0009 — A failed measurement is a point, not a gap

Status: Accepted · Date: 2026-09-08 · Answers the open clause of
[#62](https://github.com/MustardSeedNetworks/trellis/issues/62) and
[#63](https://github.com/MustardSeedNetworks/trellis/issues/63)

## Context

`MeasureThroughput` discarded a measurement that failed, and said why in the
code:

```go
// Nothing is stored. A point saved for a failed test is a zero-speed
// reading at a position where nothing was measured — a dead spot the
// survey invented.
```

Both issues ask for the opposite. #62: "a point where association fails is
recorded as a **failed point**, not silently dropped — a hole in a coverage map
must be distinguishable from an unmeasured area." #63: "a failed capture leaves
a visible **attempted point** rather than a gap."

Both sides are protecting the map from a lie, and they are different lies:

- store a **number** for a failed test and the survey invents a dead spot that
  nothing measured;
- store **nothing** and a place where the measurement failed looks exactly like
  a place nobody walked. The operator cannot tell "there is no coverage here"
  from "we never got a reading here", and the second is often the more urgent
  fact — it is usually where the coverage is worst.

## Decision

Store the attempt. Store no value for it.

A `SamplePoint` gains `Failed *Attempt{Kind, Reason}`. When it is set,
`SampleData` is nil: there is no reading, not a zero one. Consequently:

- **No layer draws it.** `extractSamples` already drops a point that carries no
  value of the metric it is asked for, which is every metric for an attempt, so
  interpolation, every heatmap layer and the dead-zone analysis exclude it
  without a special case.
- **No statistic counts it.** `measuredPoints` filters attempts out of
  `calculateSurveyStats`, which divides by the number of points it was given —
  an attempt in that denominator moved a floor's coverage score from 100 to 50
  in the test that caught it.
- **The canvas does not reach for it.** `dimensionsFromSamples` bounds the
  measured points only; a failed attempt at the far corner of a floor would
  otherwise enlarge the surveyed area and fill the difference with
  interpolation drawn from readings metres away.
- **The wire carries the reason and no numbers.** `SurveySample.failure` is set
  and `download_mbps`, `strongest_dbm` and `network_count` stay absent or zero,
  so a client cannot read a zero as coverage.
- **The UI draws a cross, not a dot.** The colour scale means signal; a dot in
  any colour on that scale reads as a measurement. Shape carries the
  distinction so it survives a reader who cannot use colour, and the reason is
  in a `<title>`.

The kind that was attempted travels with the attempt (`throughput` today).
"A throughput test failed here" is what an operator can act on; "something
failed here" is not.

## Storage

`survey_points` gains a `failure TEXT NOT NULL DEFAULT ''` column
(`00006_attempted_points.sql`). The point keeps the `sample_kind` of the
measurement that was attempted rather than gaining a `failed` kind, because
widening that CHECK constraint means rebuilding `survey_points` in SQLite, and
its children (`samples`, `active_samples`, `throughput_samples`) reference it
`ON DELETE CASCADE` — dropping the old table mid-rebuild takes every stored
reading with it. The load path already tolerates a point whose payload table
has no row, which is exactly the shape an attempt has.

## Consequences

- `TestAFailedMeasurementStoresNothing` is gone; the behaviour it locked in is
  the one being reversed. It is replaced by
  `TestAFailedMeasurementStoresAnAttemptedPoint`.
- A survey can now hold points that no layer will ever draw. That is the point:
  the map shows where the survey tried.
- Only the throughput path produces attempts today, because it is the only
  measurement that can fail with the operator standing still. A passive scan
  that returns nothing is a real reading of an empty airspace, not a failure,
  and is stored as one.
- `calculateSurveyStats` still divides by every point it is given, including
  throughput points that carry no passive reading. That is a pre-existing
  distortion of the RSSI statistics on mixed surveys and is deliberately not
  fixed here; it is unrelated to attempts and changing it would move numbers in
  existing reports.
