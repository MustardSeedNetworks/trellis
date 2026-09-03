# Seed ↔ Trellis — Wi-Fi boundary

Status: **DECIDED 2026-09-03 (supersedes the 2026-06-20 Option A decision below): both
products carry Wi-Fi analysis; Trellis owns survey/planning.** Seed keeps its own
troubleshooting/visibility analysis; Trellis carries analysis too (Live mode) plus
survey/planning/design as its primary focus (Project mode). Neither product exits
Wi-Fi. Rationale and the superseded option analysis below kept for the record.

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
Extract the hard part — the **capture backend + RF/measurement library** — as a shared
component. Trellis builds survey/planning on it; Seed keeps a *thin* live-troubleshooting
UI on the *same* core. Build the radio/RF stack once, consume it twice.
- ✓ No duplication; each product keeps the UX that fits; clean ownership (Trellis owns
  the core). ✗ Needs a stable shared-core contract + release coordination.

**C. Status quo (duplicate).** Seed keeps its own Wi-Fi; Trellis builds its own.
- ✗ Two RF/capture stacks to maintain. Rejected — this is the thing to avoid.

## Why B fits the architecture we already chose
The Trellis design (ADR-0001/0003) already makes the RF engine a **standalone process
with a protobuf contract**, and capture a standalone *package* behind one `Scanner`
interface (ADR-0006 — it is linked into the core, because macOS grants Wi-Fi names to
a signed bundle rather than to a privileged daemon). That *is* a shareable core — Seed
consuming Trellis's capture package + a subset of the engine is a natural extension,
not a rewrite. "Move Wi-Fi to Trellis" then means **Trellis owns the Wi-Fi core; Seed
becomes a consumer of it**, rather than ripping features around.

## Superseded: Option A (2026-06-20 → superseded 2026-09-03)
**All Wi-Fi moves to Trellis; Seed exits Wi-Fi.** Picked over B because two products
both touching Wi-Fi looked like a positioning muddle ("which one do I buy for Wi-Fi?").
This was never carried out: Seed's `internal/wifi` was never deprecated and Seed kept
shipping its own Wi-Fi analysis. Kept for the record; superseded below.

## Decision (2026-09-03): both products carry analysis; Trellis also owns survey/planning
Essentially Option B, extended: Seed keeps its own thin live-troubleshooting UI (Live
mode equivalent) on its own `internal/wifi`; Trellis carries the same class of analysis
(Live mode) plus the heavy survey/planning/design work (Project mode) as its primary
focus. Neither product exits Wi-Fi. The two analysis surfaces are not yet unified —
that unification is the open follow-up below, not a precondition for this decision.

## Follow-ups now open (tracked)
- **Shared Wi-Fi core (owner item, not yet scheduled):** collapse Seed's and Trellis's
  separate analysis surfaces — scan model, dot11 decode, anomaly catalog — onto one
  shared core, defaulting to Seed's `internal/wifi` as the source after Seed v1.
- **`LICENSE_STRATEGY.md`:** still has no Trellis tier; a draft is prepared separately
  (T-B7) — Trellis is pre-alpha with no license tier today.
- **Trellis PRD:** Live-mode capability set added (see `01-PRD.md`).
