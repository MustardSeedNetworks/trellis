# ADR-0008 — Sample markers are sized by how close the readings lie

Status: Accepted · Date: 2026-09-07 · Answers [#326](https://github.com/MustardSeedNetworks/trellis/issues/326)

## Context

Every reading on a coverage map is marked with a black circle of radius 4 px
carrying a white centre. That size was chosen against a stop-and-go walk, which
produces three to thirty pinned points on a floor, and it is right there: the
marker says where the operator stood.

Continuous capture (#299) and the survey importers do not produce thirty points.
The AirMagnet import from #324, rendered through the report from #325, is a
single 614-point walk, and at that density the markers touch. They merge into
one solid black stroke that runs the length of the walk, covering the
interpolated values underneath it. The heatmap is most trustworthy along the
line the operator actually measured and, before this change, least readable
there.

Both surfaces #326 names — the Coverage page and the PDF report — render through
`GenerateFloorHeatmap`, so the marker style has one home:
`renderSamplePoints` in `core/survey/heatmap.go`.

## Decision

The marker is sized against the median distance from a reading to its nearest
other reading, measured in image pixels:

| Median neighbour spacing | Marker |
| --- | --- |
| 10 px or more | radius 4 with the white centre — unchanged |
| 4 px to 10 px | radius `(spacing - 2) / 2`, no white centre |
| under 4 px | radius 1, and the run is thinned so no two drawn markers are within 4 px |

Two rules hold this together. A marker never grows past the radius 4 a sparse
walk gets, so a stop-and-go survey renders pixel-identically to before. And two
markers always keep two pixels of background between them, so what is drawn
reads as separate readings rather than as a stroke.

The full-marker case is decided in float64, before any conversion to int. A
single reading has no neighbour and its spacing is `+Inf`, and converting `+Inf`
to `int` is architecture-defined in Go: arm64 saturates to maxint and amd64 to
minint. A first version clamped after the conversion and so drew the full marker
on the developer's M2 and a bare dot on the Linux runner, where CI caught it.

The white centre is dropped below radius 2 because at that size it consumes the
marker rather than punctuating it.

Thinning governs only the markers. The heat itself is interpolated from every
reading, so nothing measured is dropped from the map — what is dropped is a
duplicate claim about a position already marked one pixel away.

The spacing is a median rather than a minimum: one pair of readings that happen
to land on the same pixel should not shrink the markers for a whole floor. It is
estimated from at most 512 evenly strided probes measured against every reading,
which bounds the cost on an imported walk without changing the answer at the
densities that matter.

## Alternatives rejected

**Opacity by local density.** A semi-transparent stroke is still a stroke. It
would make the values underneath faintly visible without ever letting an
operator count the readings, which is what the marker is for.

**Markers as a toggle, off by default in the report.** A knob is a decision
deferred onto the user plus a permanent test surface, and the default would
still have to be right for both walk shapes. Deciding is cheaper.

**Full markers for pinned points, dots for interpolated ones.** `SamplePoint`
already carries `Interpolated` (#301), and a pin is genuinely a different claim
from an interpolated position. But the defect was found on an _import_, and an
imported point carries no pin flag: every reading in the 614-point AirMagnet
walk would read as a pin and the map would be exactly as unreadable. The
distinction is real and remains available if a future row wants to draw the two
differently; it cannot be the mechanism that fixes this.

## Consequences

A dense walk now renders as a countable line of dots — the 600-point fixture in
`heatmap_density_test.go` draws 150 separate markers where it previously drew
one 602-pixel run. An operator reading a continuous survey sees fewer marks than
readings, which is the honest trade: the alternative is a mark per reading that
says nothing about any of them.
