# ADR-0011 — The predictive model is `core/rf`'s log-distance model, in Go, on the CPU

Status: Accepted · Date: 2026-10-15 · Accepted by the owner's review and merge
of this ADR · Supersedes [ADR-0004](ADR-0004-gpu-via-wgpu-dawn.md) · Amends
[ADR-0002](ADR-0002-language-per-component.md)

## Context

ADR-0002 put the RF engine in C++20 behind a C ABI, and ADR-0004 put its
kernels on the GPU through wgpu-native or Dawn with a SIMD fallback.
`docs/04-RF-ENGINE.md` specifies that engine: multi-wall-and-floor path loss,
ray tracing, derived SNR, data-rate and interference layers. None of it was
ever written. The roadmap made the build conditional on Gate G1.

G1 passed on 2026-10-15 (`docs/11-GATE-G1-RESULT.md`, #813). It measured a
smaller thing than the engine those ADRs describe: the closed-form
log-distance model in `core/rf`, with no wall term, no AP height and no
antenna pattern. Against 510 radios on surveyor-placed APs in ten Link-Live
surveys it scored 8.30 dB mean error uncalibrated at 10 dBm and 3.55 dB once
calibrated per AP. So the model that cleared the bar is already in the tree,
in Go. Nothing measured shows that walls, a GPU or a ray tracer close the
remaining gap, and walls need plan vectorisation and an editor before they
can be tried at all.

## Decision

1. **The predictive model is `core/rf`'s log-distance model.** Per cell,
   `RSSI = TxP − (PL0 + 10·n·log10(d))`, with `PL0` the free-space loss at
   1 m at the band's centre, `n` = 3.0 uncalibrated, and the best server
   taken over a floor's placed APs. It runs in Go on the CPU, inside
   `trellisd`, with no cgo.
2. **It emits the measured heatmap's grid.** A prediction returns the same
   `HeatmapResult` (grid, cell size, colour scale, statistics) as
   interpolated survey data for the same floor, so the Coverage page, the
   legend and the PDF report treat both alike.
3. **Calibration is per AP, from the floor's own walk.** Where a floor has
   placements and measured points, `rf.Fit` replaces the defaults per AP
   within the physical bounds G1 used, and a clamped or under-sampled fit
   falls back to the defaults with its reason reported.
4. **Transmit power is an input.** It is the largest single error term G1
   found (8.30 dB at 10 dBm, 11.15 dB at 20 dBm), so each placement carries
   its configured power and the default is the stated 10 dBm G1 scored at.
5. **Walls, the GPU and ray tracing stay closed** until a measured result
   shows the closed form missing a bar that matters, on data the product
   holds. Reopening any of them is a new ADR with that measurement in it.

## Why

- A closed-form model over a 5,000 m² floor at 0.25 m cells is about 80,000
  cells per AP. That is milliseconds in Go; the PRD's 100 ms incremental
  budget does not need a GPU.
- One language and one process for the model keeps ADR-0006's single binary
  and adds no toolchain, C ABI or shared-memory seam.
- The model the product ships is the model G1 scored. A different engine
  would need its own gate.

## Consequences

- ADR-0004 is superseded. ADR-0002's C++ engine line no longer holds; its Go
  core and TypeScript UI lines stand. ADR-0003's shared-memory grid seam has
  no engine process to serve, and the draft `docs/contracts/engine.proto`
  stays unimplemented.
- The roadmap's Phases 2–4 become the predictive-phase rows of the Trellis
  plan (placements on floors, a `predicted_rssi` heatmap, placement editing,
  per-AP calibration, the placement UI, and a measured-minus-predicted layer
  in the report).
- `docs/04-RF-ENGINE.md` stays as the long-range specification and now
  records G1's two limits at its head.
- A prediction is only as good as the stated transmit power and the floor's
  scale; a floor with no scale or no placements is refused, not drawn empty.

## Alternatives rejected

- **Build the C++/GPU engine as specified.** Months of work for accuracy no
  measurement says it adds, ahead of a 2026-12-03 box on a non-market build.
- **Wall-aware model in Go now.** Floor plans are raster images; there is no
  wall geometry to sum over without vectorisation and an editor, which is a
  larger build than the whole predictive phase.
