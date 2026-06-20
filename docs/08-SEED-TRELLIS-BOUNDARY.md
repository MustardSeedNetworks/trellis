# Seed ↔ Trellis — Wi-Fi boundary (OPEN DECISION)

Status: **DECIDED — Option A (2026-06-20): all Wi-Fi moves out of Seed into Trellis.**
Seed = wired diagnostics / security / compliance. Trellis = the sole Wi-Fi product
(Live troubleshooting + Project survey/planning) on one shared Wi-Fi/RF/capture core.
Rationale below kept for the record.

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

## Decision: Option A
**All Wi-Fi moves to Trellis; Seed exits Wi-Fi.** Picked over B because two products
both touching Wi-Fi is a positioning muddle ("which one do I buy for Wi-Fi?"), and
pre-alpha is the cheapest time to make the cut. Trellis grows a **Live mode** (the
ex-`canopy` troubleshooting/visibility) alongside Project mode; both sit on the shared
core — so we still build the RF/capture stack once, just consume it from two modes
inside one product instead of two products.

## Follow-ups now open (tracked)
- **Seed repo:** plan removal/deprecation of `internal/wifi` (ex-`canopy`); decide
  reuse-as-reference vs. rewrite of the troubleshooting logic into Trellis Live mode.
- **`LICENSE_STRATEGY.md`:** name Trellis the Wi-Fi product; remove Wi-Fi from Seed's
  scope and tier matrix; reflect that "no planning in Seed" now reads "no Wi-Fi in Seed."
- **Trellis PRD:** Live-mode capability set added (see `01-PRD.md`).
- Shared-core packaging (lib vs daemon vs both) — resolved *inside* Trellis now, not a
  cross-product contract, which simplifies it.
