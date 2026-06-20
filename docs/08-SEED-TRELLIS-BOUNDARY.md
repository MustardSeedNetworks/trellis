# Seed ↔ Trellis — Wi-Fi boundary (OPEN DECISION)

Status: **under consideration** (2026-06-20). Captures the "pull all Wi-Fi out of Seed
into Trellis" thought so it can be decided deliberately.

## The distinction that matters
Wi-Fi work splits into two *different jobs* with different UX, even though they share
the same radios/RF tech underneath:

| | **Live troubleshooting** (Seed today, ex-`canopy`) | **Survey / planning / design** (Trellis) |
|---|---|---|
| Question | "is this Wi-Fi healthy *right now*?" | "design/validate coverage for a *site*" |
| Mode | live, ad-hoc, on a running network | project-based, predictive + measured |
| Examples | connected-SSID signal/SNR, neighbor-AP scan, channel util, roam/assoc forensics | floorplan + walls, AP placement, heatmaps, walk-survey, calibration, reports |
| Weight | thin, instant | heavy, stateful, GPU |

Conflating them in one UX is a trap: a Seed user wanting a 5-second neighbor scan
shouldn't open a survey *project*; a Trellis designer needs the heavy project model.

## What's unambiguous
- **Trellis owns:** survey, prediction, planning, design, calibration, reporting.
- **Seed does *not* own planning** (the locked strategy line — correctly scoped to Seed).

## The actual question
Where does **live Wi-Fi troubleshooting** live, and how do we avoid building the hard
part (radios → measurements → RF) twice?

## Options
**A. Move everything to Trellis.** Seed drops Wi-Fi entirely (wired diagnostics /
security / compliance only); Trellis is the sole Wi-Fi product, with both a light
"live" mode and the heavy planner.
- ✓ One Wi-Fi product, crisp positioning. ✗ Seed loses quick Wi-Fi checks; Trellis must
  grow a lightweight live UX it otherwise wouldn't need.

**B. Share a Wi-Fi/RF/capture core; keep fitting UX in each product (recommended).**
Extract the hard part — the **capture daemon + RF/measurement library** — as a shared
component. Trellis builds survey/planning on it; Seed keeps a *thin* live-troubleshooting
UI on the *same* core. Build the radio/RF stack once, consume it twice.
- ✓ No duplication; each product keeps the UX that fits; clean ownership (Trellis owns
  the core). ✗ Needs a stable shared-core contract + release coordination.

**C. Status quo (duplicate).** Seed keeps its own Wi-Fi; Trellis builds its own.
- ✗ Two RF/capture stacks to maintain. Rejected — this is the thing to avoid.

## Why B fits the architecture we already chose
The Trellis design (ADR-0001/0003) already makes capture and the RF engine **standalone
processes/components with protobuf contracts**. That *is* a shareable core — Seed
consuming the Trellis capture daemon + a subset of the engine is a natural extension,
not a rewrite. "Move Wi-Fi to Trellis" then means **Trellis owns the Wi-Fi core; Seed
becomes a consumer of it**, rather than ripping features around.

## Recommendation (owner decides)
Lean **B**: Trellis owns the Wi-Fi/RF/capture core; Seed keeps a thin live mode on it
(or drops Wi-Fi later if you want Seed purely wired/security — that's a positioning
call, reversible). Either way, **don't duplicate the RF/capture stack.**

## To decide before this closes
- Is Seed "diagnose any network incl. a quick Wi-Fi check" or "wired/security only"?
- Does the shared core ship as a library, a daemon, or both?
- Versioning/release coordination between Seed and Trellis on the shared core.
- Migration path for the existing `internal/wifi` code (reuse vs reference implementation).
