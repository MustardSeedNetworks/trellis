# Trellis — Roadmap

Each phase ends in something **demoable** and has explicit exit criteria. No phase
starts before the prior phase's contract is frozen. Order is dependency-driven, not
feature-driven.

## Phase 0 — Plan freeze (now)
- Settle `docs/` (vision, PRD, architecture, tech stack, ADRs).
- **Freeze the three `.proto` contracts** (`engine`, `api`, `capture`) — these
  de-risk everything downstream.
- **Exit:** docs reviewed; `buf lint` clean; contracts tagged v1-draft.

## Phase 1 — Engine MVP (C++/CPU, no GPU)
- Geometry + Multi-Wall-and-Floor model; single metric (RX power) → grid.
- Pure function behind the C ABI; shared-memory grid output.
- **Golden-file tests** (known scene → known grid ±tolerance) + property tests.
- **Exit:** `engine compute scene.pb → grid` matches goldens; runs headless in CI.

## Phase 2 — Go core + seams + project model
- SQLite schema (`sqlc`) + project bundle format; load/save.
- Go ↔ engine over UDS + shmem; incremental scene cache.
- A **headless CLI** that drives the engine (import → place AP → emit heatmap PNG).
- **Exit:** end-to-end plan with no UI; contract tests green on the Go↔engine seam.

## Phase 3 — UI + Wails (planner-only)
- Floorplan import + scale calibration; wall/material editing; AP placement.
- Binary grid → WebGL texture → color-map shader; layer switching client-side.
- **Exit:** first real desktop planner — drag an AP, see the heatmap update.

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
- Engine and Go core can proceed in parallel once contracts freeze.
- UI can mock the API from the `.proto` before the Go core is done.
- GPU (Phase 4) is deferred on purpose: prove correctness on CPU first, then accelerate.
