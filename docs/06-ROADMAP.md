# Trellis — Roadmap

Each phase ends in something **demoable** and has explicit exit criteria. No phase
starts before the prior phase's contract is frozen. Order is dependency-driven, not
feature-driven.

## The bet (read before anything)
The measured-survey half is **proven Seed code we migrate** (`09-SEED-MIGRATION.md`),
and the plumbing (Go core, React UI, capture, cloud) is "just" engineering. The risk is
isolated to **one component: the *predictive* RF engine's accuracy** — Ekahau's moat is
decades of propagation tuning so people trust a heatmap enough to deploy on it. So this
roadmap **inverts the risk**: migrate the measured tool first (it gives us real ground
truth), then prove the predictive engine against it (**Gate G1**) *before* building the
full product. If predictions aren't trustworthy we've spent weeks, not a year, finding
out — don't build the cathedral around an unproven engine.

## Phase 0 — Plan freeze (now)
- Settle `docs/` (vision, PRD, architecture, tech stack, ADRs).
- **Freeze the three `.proto` contracts** (`engine`, `api`, `capture`) — these
  de-risk everything downstream.
- **Exit:** docs reviewed; `buf lint` clean; contracts tagged v1-draft.

## Phase 1 — Migrate Seed's measured-survey subsystem (reuse; low-risk)
Lift Seed's proven, tested survey code (`09-SEED-MIGRATION.md`) into Trellis. This is
mature Go, not greenfield.
- Move `seed/internal/wifi/survey/` (~3,868 LOC) into the Trellis **Go core**: AirMapper
  `.amp` import, interpolation, heatmap/colorscale, analysis, multi-floor, reports
  (keep tests). Reshape the data model to the Trellis project bundle + protobuf API.
- Port the AirMapper import + survey UI into the Trellis UI (Project mode).
- **Outcome:** a working **measured-survey tool** *early* AND the **G1 ground-truth
  pipeline** (it already heatmaps the real 73-pt Everett survey).
- **Exit:** decode + interpolate + heatmap the real Everett `.amp` end-to-end in
  Trellis; migrated tests green.

## Phase 2 — Predictive engine MVP (C++/CPU) — **the bet**
The only greenfield, research-grade component. Build it small and prove it.
- Geometry + Multi-Wall-and-Floor model → predicted RX-power grid; pure function behind
  the C ABI; shared-memory output. **Golden + property tests.**
- Emits the **same `GridDescriptor`** as the measured grids, so predicted/measured are
  interchangeable in UI + reports.
- **Exit:** `engine compute scene.pb → grid` matches goldens; runs headless in CI.

## 🚦 Gate G1 — Engine credibility (make-or-break; do NOT skip)
We now have **real ground truth in hand** (Everett: 73 measured points + AP layout +
floorplan, decoded by the migrated pipeline).
- Feed the engine the Everett floorplan + AP layout + a wall model; predict RX power at
  each of the 73 survey points; **diff predicted vs. the measured values** (mean/95th-
  pct dB error; coverage-boundary agreement). Exercise **calibration**.
- *(Optional G1a)* also diff against an AirMagnet/Ekahau predicted export of the same
  scene (model-vs-model) for an implementation sanity check.
- **PASS (proposed, tune):** ≤ ~6 dB mean error pre-cal, ≤ ~3–4 dB post-cal; coverage
  boundary within ~1 cell.
- **Pass:** green-light the full build. **Fail:** fix the model (or rethink the bet)
  *before* the cathedral — the cheapest possible place to learn the truth.

## Phase 3 — Planning UX + Wails
- Floorplan import + scale calibration; wall/material editing; AP placement/config
  (Project mode). Predicted heatmap via the binary/WebGL path; **reuse** the migrated
  measured heatmap rendering.
- **Exit:** drag an AP → predicted heatmap updates; toggle measured vs. predicted.

## Phase 4 — GPU + full predictive layers
- Port kernels to wgpu-native/Dawn; keep CPU fallback.
- Add SNR, data-rate (MCS tables), interference, coverage, multi-floor leakage.
- Hit the **<100 ms incremental** budget on a reference floorplan.
- **Exit:** all predictive metrics; perf budget met; ray-trace available for "final".

## Phase 5 — Capture + survey loop
- Capture daemon: **external-HW backend first**, then host-NIC per OS.
- Measurement streaming → geotag → Parquet → interpolation → measured heatmap.
- **Calibration**: fit model to measurements; measured-vs-predicted compare.
- **Exit:** walk a real site, capture, see measured heatmap + calibrated prediction.

## Phase 6 — Reporting, licensing, polish
- Report templates → PDF; Ed25519 licensing + feature gates; project format v1.
- Installers + code signing/notarization (per `build-platform-capability-goals`).
- **Exit:** shippable v0.1 desktop build, signed, on all three OSes.

## Phase 7 — Cloud/team (optional)
- Go core as a server; web planner (no capture); object-storage project sync.
- **Exit:** browser planner shares the engine + projects with desktop.

## Sequencing notes
- Contracts before code (Phase 0) is non-negotiable — they're the integration surface.
- **Migrate before invent (Phase 1 before 2):** standing up Seed's measured tool first
  gives an early shippable *and* hands the engine its G1 ground truth — so the risky
  engine is built against real data from day one.
- Predictive engine (Phase 2) and the migrated core can proceed in parallel once
  contracts freeze.
- UI can mock the API from the `.proto` before the core is done.
- GPU (Phase 4) is deferred on purpose: prove correctness on CPU first, then accelerate.
