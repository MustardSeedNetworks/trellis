# Trellis — RF Engine Internals

The engine is a **pure function**: `(scene, params) → grids`. Deterministic, no I/O,
golden-testable.

**What ships ([ADR-0011](adr/ADR-0011-predictive-model-is-core-rf-on-cpu.md)):** the
log-distance model in `core/rf`, in Go on the CPU, calibrated per AP from the floor's
own walk. The rest of this doc — walls, the GPU, ray tracing, the derived layers — is
the long-range specification. ADR-0011 keeps it closed until a measured result shows
the closed form missing a bar.

## What Gate G1 measured, and its two limits

G1 (`11-GATE-G1-RESULT.md`) scored the `core/rf` model against 510 radios on
surveyor-placed APs: 8.30 dB mean error uncalibrated at 10 dBm, 3.55 dB calibrated.
Two limits come with that number:

- **No wall term.** Floor plans are raster images with no wall geometry, so the
  model has no `Σ wall_atten`, no AP height and no antenna pattern. The calibrated
  exponent absorbs the floor's average obstruction; a single wall between an AP
  and a cell is not modelled.
- **Transmit power is the largest error term.** The same radios score 8.30 dB at an
  assumed 10 dBm and 11.15 dB at 20 dBm. Each placement carries its configured
  power; the default is 10 dBm, the power G1 scored at.

## Propagation models (tiered by speed/accuracy)

| Model | Use | Cost |
| --- | --- | --- |
| **Multi-Wall-and-Floor (COST-231 MWF)** | interactive editing | closed-form per cell; GPU-trivial |
| **ITU-R P.1238 indoor** | interactive alt | closed-form |
| **Dominant Path (DPM)** | accurate-ish | path search per cell |
| **3D ray-trace (SBR / image method)** | final / validation | GPU, seconds |

**MWF core (per AP, per cell):**

```text
RxP(cell) = TxP + Gtx(θ,φ) + Grx − PL
PL = PL0 + 10·n·log10(d/d0) + Σ wall_atten + Σ floor_atten
```

`d` = 3D distance AP→cell; `n` = path-loss exponent (env-tuned, calibratable);
`Σ wall_atten` = per-band material attenuation of walls the AP→cell segment crosses
(segment/wall intersection test); `Σ floor_atten` = inter-floor slabs crossed.

## Derived layers (from the per-AP RxP grids)

- **RX power**: max over APs per cell (best server) + which AP (`STRONGEST_AP`).
- **SNR**: `RxP_best − noise_floor` (default −95 dBm; configurable).
- **Interference**: aggregate co-channel + weighted adjacent-channel power from other
  APs on the same/nearby channels → effective SINR.
- **Data rate**: map SINR → PHY rate via **per-PHY MCS tables** (11n/ac/ax/be), keyed by
  channel width + spatial streams. Tables are data, not code.
- **Coverage**: `RxP ≥ requirement` (e.g. −67 dBm voice) → boolean grid.
- **AP overlap / roaming**: count of APs above a secondary threshold per cell.
- **Channel utilization (predicted)**: from overlap + assumed load (Later).

## Multi-floor

_Long-range; not in ADR-0011's model._

Floors are stacked planes with `interfloor_atten_db`. An AP on floor _k_ contributes to
floor _k±m_ with `m` slab crossings added to PL. 3D voxel mode (Later) for true leakage.

## Survey interpolation (measured → grid)

- **IDW** (fast default), **Kriging** (better, slower), **Natural-Neighbor**.
- Inputs: geotagged `MeasurementPoint`s; output: same grid format as predictions so the
  UI/report treat measured and predicted identically.

## Calibration (the trust-maker)

Given measurements + the current scene, **least-squares fit** the tunable model params
(`n`, `PL0`, per-material attenuation deltas) to minimize measured−predicted error,
within physical bounds. Output: adjusted params + a residual/error map. This is what
makes predictions credible and is a key differentiator vs. naive predictive tools.

## GPU & fallback

_Superseded by ADR-0011: the model runs in Go on the CPU._

- Kernels in **WGSL/SPIR-V** via wgpu-native/Dawn. The MWF model maps to one
  compute dispatch per (floor, band, AP-batch); reduction passes derive best-server/
  SNR/etc.
- **CPU fallback**: same math in SIMD (ISPC or intrinsics) so CI and no-GPU hosts work.
  Golden tests run on the CPU path for determinism.

## Performance design

- **Incremental**: the engine caches the scene; a `SceneDelta` (moved AP) recomputes
  only that AP's contribution and the affected region's reductions — not the building.
- **Grid sizing**: typical 0.25 m cells; a 5,000 m² floor ≈ 80k cells/band → trivial on
  GPU, the budget pressure is _latency of the round-trip + reductions_, not raw compute.

## Testing

- **Golden files**: canonical scenes (free-space, single-wall, two-room) → expected
  grids within tolerance; catch model regressions.
- **Property tests**: monotonicity (adding a wall never raises RxP behind it; greater
  distance never raises RxP), symmetry, energy sanity.
- **Reference validation**: compare against published indoor-propagation datasets.
