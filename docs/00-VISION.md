# Trellis — Vision & Positioning

## Problem
- **AirMagnet Survey/Planner is EOL** — abandoned, Windows-only, brittle (we reverse-
  engineered its licensing and internals; it's a dead-end stack: native C++ + .NET WPF
  + Crystal Reports + NDIS capture drivers + MD5/rotor licensing).
- The live alternatives — **Ekahau, Hamina, iBwave** — are powerful but **expensive,
  closed, and (mostly) tied to vendor hardware or cloud**. The category has no modern,
  open, cross-platform, scriptable option.

## Opportunity
We proved this session that the hard part — a faithful RF + license + capture
understanding — is tractable. A clean rebuild can be **cross-platform, GPU-accelerated,
scriptable (headless engine + open project format), and modern** in a way the
incumbents aren't.

## What Trellis is — MSN's Wi-Fi survey/planning product, with analysis too (two modes)
Both Seed and Trellis carry Wi-Fi analysis; Trellis owns survey/planning and carries it
as its primary focus (decided 2026-09-03, superseding the 2026-06-20 "Seed exits Wi-Fi"
decision — see Decision history below).
- **Live mode** (troubleshooting/visibility, secondary to Project mode here — Seed
  carries the same job independently): connected-SSID signal/SNR, neighbor-AP scan,
  channel utilization, roam/association forensics. Thin, instant, ad-hoc.
- **Project mode** (survey + planning + design — Trellis's primary focus):
  - *Plan*: floorplan + walls/materials, AP/antenna placement, coverage/SNR/data-rate/
    interference heatmaps, multi-floor, requirement validation (e.g. -67 dBm voice).
  - *Survey*: walk a site, capture measurements tied to location, measured heatmaps.
  - *Calibrate*: fit predictions to measurements so plans are trustworthy.
  - *Report*: templated, versionable PDF/HTML.

Both modes sit on one shared **Wi-Fi/RF/capture core** (`internal/capture` + the engine).
A follow-up (owner-tracked, not yet scheduled) is collapsing Seed's and Trellis's
separate analysis surfaces onto one shared Wi-Fi core (scan model, dot11 decode,
anomaly catalog), defaulting to Seed's `internal/wifi` as the source after Seed v1.

## What Trellis is *not* (scope discipline)
- Not wired diagnostics / security / compliance (that's **Seed**).
- Not a network simulator (that's **NIAC / "Greenhouse"**).
- Not RF spectrum hardware — it *consumes* capture from supported radios/appliances.

## Positioning
| | AirMagnet | Ekahau/Hamina/iBwave | **Trellis** |
|---|---|---|---|
| Platform | Windows-only, EOL | Win/Mac (mostly), cloud | **Win/Mac/Linux + web** |
| Openness | closed | closed | **documented project format, source-available (BUSL-1.1)** |
| Engine | CPU, native+WPF | proprietary | **C++/GPU, headless, testable (planned — not built)** |
| Capture | NDIS drivers | vendor HW | **external HW first + host-NIC** |
| Cost model | dead | premium | TBD — pre-alpha, no license tier yet |

## Strategy alignment (updated 2026-09-03)
The MSN locked strategy's "no planning / planning is hardware-defended" line
(`LICENSE_STRATEGY.md`, 2026-05-19) was **scoped to Seed** — i.e. *Seed* doesn't grow
into a survey/planning tool. A **separate** product (Trellis) owning Wi-Fi planning is
fully consistent with that: planning simply doesn't live in Seed.

**Decided (2026-09-03): Seed carries Wi-Fi analysis (troubleshooting/visibility), and
Trellis carries both Wi-Fi analysis and Wi-Fi survey/planning, with survey/planning as
its primary focus.** Neither product exits Wi-Fi. This supersedes the 2026-06-20
decision that all Wi-Fi would move to Trellis and Seed would exit Wi-Fi entirely; that
plan was never carried out — Seed's `internal/wifi` was never deprecated and still ships
Seed's own analysis capability. See `docs/08-SEED-TRELLIS-BOUNDARY.md`.

**Follow-ups (owner-tracked):** collapse the two products' separate analysis surfaces
onto one shared Wi-Fi core after Seed v1, defaulting to Seed's `internal/wifi`;
`LICENSE_STRATEGY.md` still has no Trellis tier (draft tracked separately).

### Decision history
- **2026-06-20:** all Wi-Fi moves out of Seed into Trellis; Seed exits Wi-Fi entirely.
  **Superseded 2026-09-03** — never implemented; both products keep Wi-Fi analysis.
- **2026-09-03:** current decision, above.

## Audience
WLAN engineers, integrators, and MSPs doing site design and validation surveys who
want a modern, cross-platform, scriptable tool without per-seat cloud lock-in.
