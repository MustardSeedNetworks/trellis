# Seed → Trellis — Survey Migration Inventory

Seed already grew a **mature, tested, measured-survey subsystem** (a licensed Wi-Fi
feature). Under the survey/planning-ownership decision (`08-SEED-TRELLIS-BOUNDARY.md`,
2026-09-03: Trellis owns survey/planning) it migrates here. This doc inventories what
moves, what's reused as-is, and what's net-new — so Trellis is a **migrate-and-extend**,
not a greenfield, for everything except the predictive engine.

## What Seed has today (measured-survey only — confirmed)
`seed/internal/wifi/survey/` — **~3,868 LOC Go, 10 src + 12 test files**, plus API
handlers and UI. It does **measured** survey end-to-end; it does **no prediction**
(grep across the pkg: Wall 0 · Material 0 · Antenna 0 · PathLoss 0 · TxPower 0 · Predict 0).

| Seed source | Does | Disposition in Trellis |
|---|---|---|
| `internal/wifi/survey/airmapper_parser.go` | parse AirMapper `.amp` → survey points/APs | **Reuse** (Go) → Trellis core import; also the G1 ground-truth path |
| `internal/wifi/survey/interpolation.go` | measured points → grid (IDW/…) | **Reuse** (Go) → core |
| `internal/wifi/survey/heatmap.go`, `colorscale.go` | heatmap raster + color mapping | **Reuse** → core (UI shades client-side; keep server raster for reports) |
| `internal/wifi/survey/analysis.go` | dead-zone detect, coverage score, recommendations | **Reuse** → core |
| `internal/wifi/survey/multifloor.go` | multi-floor measured | **Reuse**, extend for predictive multi-floor |
| `internal/wifi/survey/report.go` | survey reports | **Reuse**, retarget to Trellis reporter |
| `internal/wifi/survey/anomaly.go` | survey anomalies | **Reuse** |
| `internal/wifi/survey/migration.go`, `survey.go` | project/data model + migrations | **Reshape** to Trellis project format (`05-DATA-MODEL.md`) + protobuf |
| `internal/api/handlers_survey*.go` (samples/floors/analysis/report/floorplan/license) | survey REST API | **Reshape** to the Trellis protobuf API (`contracts/api.proto`) |
| `ui/src/utils/airmapper.ts`, `ui/src/schemas/airmapper.ts` (valibot) | AirMapper import + validation | **Reuse** in Trellis UI |
| `ui/src/components/survey/*` | survey UI | **Reuse/port** into Trellis UI (Project mode) |
| `data/surveys/` + sample data | fixtures | **Reuse** as test fixtures |
| `handlers_survey_license` | feature licensing | **Replace** with Trellis Ed25519 (`ADR-0005`) |

## What's net-new (the only greenfield — and the bet)
- **Predictive RF engine** (C++/GPU): walls, materials, antennas, propagation models,
  AP placement, tx-power → *predicted* heatmaps; calibration (fit predicted→measured).
- **Planning UX**: draw walls/materials, place/configure APs (Project mode editing).
- **Live mode** (thin troubleshooting) — small, on the shared capture core.

## Architecture consequence (refines `02`/`03`)
The measured pipeline is **already Go and light** (73 points → grid) → it lives in the
**Go core**, lifted from Seed. That **narrows the C++/GPU engine to *prediction only***
(where GPU matters: interactive recompute on AP move, ray-trace, multi-floor leakage).
Two grid producers, one representation:
- **Measured grids** — Go core (reused Seed interpolation/heatmap).
- **Predicted grids** — C++/GPU engine (new).
Both emit the same `GridDescriptor` (`contracts/engine.proto`) so the UI/report treat
them identically, and **calibration** is the bridge that makes predictions trustworthy.

## Migration approach
1. **Lift** `internal/wifi/survey/` into a Trellis Go module (`/core/survey`), keeping
   tests; swap Seed-specific deps (logging/license) for Trellis equivalents.
2. **Reshape** the project/data model to the Trellis bundle format + protobuf API.
3. **Port** the AirMapper import + survey UI into the Trellis UI (Project mode).
4. **Add** the predictive engine alongside; reuse the *same* heatmap/colorscale render.
5. **Seed side:** once Trellis owns it, deprecate/remove `seed/internal/wifi/` and its
   survey handlers/UI; update `LICENSE_STRATEGY.md`.

## Why this is good news
The risky, research-grade part of Trellis is now isolated to **one component** (the
predictive engine). Everything else — import, interpolation, heatmaps, analysis,
reports, multi-floor, UI — is proven Seed code that migrates. And Seed's measured
pipeline already produces the **G1 ground-truth heatmap** from the real 73-point
Everett survey, so the engine has something real to be validated against on day one.
