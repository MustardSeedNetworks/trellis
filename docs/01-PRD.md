# Trellis — Product Requirements

Status: draft. Requirements are tagged **[MVP]** (v0.1) or **[Later]**.

## Functional

### Floorplan & project
- **[MVP]** Import floorplan as image (PNG/JPG) or PDF; **[Later]** CAD (DXF/DWG).
- **[MVP]** Scale calibration (draw a known distance).
- **[MVP]** Multi-floor buildings (stacked floors, elevation, ceiling height).
- **[MVP]** Project = open, documented bundle; save/load; **[Later]** versioning.

### Modeling
- **[MVP]** Draw walls; assign materials from a library with per-band attenuation.
- **[MVP]** Material library (editable); sensible defaults (drywall/concrete/glass/...).
- **[MVP]** Place APs; per-radio band/channel/width/tx-power/PHY; antenna pattern +
  orientation/downtilt.
- **[Later]** Zones/areas with coverage requirements; auto-AP-placement optimization.

### Predictive heatmaps
- **[MVP]** RX power, SNR, data-rate, coverage-vs-threshold — per band, per floor.
- **[MVP]** Interactive recompute on AP move (**< 100 ms** incremental, see NFR).
- **[MVP]** Multi-Wall fast model; **[Later]** ray-trace "final" model.
- **[Later]** Interference (co/adjacent channel), AP-overlap/roaming, channel plan.
- **[Later]** Inter-floor leakage in 3D.

### Survey (measured)
- **[MVP]** Active/passive capture via a supported radio (external HW first).
- **[MVP]** Position tagging: manual pin-drop on the floorplan; **[Later]** continuous/GPS.
- **[MVP]** Measured heatmaps (interpolated from points).
- **[Later]** Spectrum integration; calibration (fit prediction to measurement) —
  *calibration may land MVP if Phase 5 allows.*

### Analysis & reporting
- **[MVP]** Layer toggles, thresholds, legends, per-AP inspection.
- **[MVP]** PDF report from a template (coverage maps + AP table + summary).
- **[Later]** AirWISE-style advisories; measured-vs-predicted diff report.

### Automation
- **[MVP]** Headless engine + CLI (scriptable plan/compute/export) — falls out of the
  architecture for free and is a differentiator.

## Non-functional (NFR)
- **Cross-platform:** Windows, macOS, Linux for the **planner**; capture is HW-gated
  (host-NIC best-effort, external HW primary). Web planner [Later].
- **Performance budgets:**
  - Incremental heatmap recompute on AP move: **< 100 ms** at 0.25 m grid on a
    ~5,000 m² floor (GPU).
  - Full-building recompute (all floors/bands/metrics, fast model): **< 3 s**.
  - Survey ingest: sustain **≥ 1,000 measurement points/s** without UI stall.
- **Scale:** projects with **10+ floors**, **multi-million-cell** grids, **100k+**
  survey points.
- **Determinism:** identical scene → identical grids (golden-testable).
- **Reliability:** an engine/GPU crash must not lose project data or kill the UI
  (process isolation).
- **Security/licensing:** offline Ed25519 license verification; no phone-home required.
- **Footprint:** desktop install < 300 MB; cold start < 3 s.

## Explicit non-goals (v0.1)
- Real-time troubleshooting / packet decode (Seed's lane).
- Network simulation (NIAC/Greenhouse).
- Cloud collaboration (Phase 7).
- Auto-design/optimization (Later).
