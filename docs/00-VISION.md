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

## What Trellis is — **the** MSN Wi-Fi product (two modes, one core)
All Wi-Fi capability lives here (decided 2026-06-20; Seed exits Wi-Fi entirely).
- **Live mode** (troubleshooting/visibility — migrates from Seed's ex-`canopy`):
  connected-SSID signal/SNR, neighbor-AP scan, channel utilization, roam/association
  forensics. Thin, instant, ad-hoc.
- **Project mode** (survey + planning + design):
  - *Plan*: floorplan + walls/materials, AP/antenna placement, coverage/SNR/data-rate/
    interference heatmaps, multi-floor, requirement validation (e.g. -67 dBm voice).
  - *Survey*: walk a site, capture measurements tied to location, measured heatmaps.
  - *Calibrate*: fit predictions to measurements so plans are trustworthy.
  - *Report*: templated, versionable PDF/HTML.

Both modes sit on one shared **Wi-Fi/RF/capture core** (the capture daemon + engine).

## What Trellis is *not* (scope discipline)
- Not wired diagnostics / security / compliance (that's **Seed**).
- Not a network simulator (that's **NIAC / "Greenhouse"**).
- Not RF spectrum hardware — it *consumes* capture from supported radios/appliances.

## Positioning
| | AirMagnet | Ekahau/Hamina/iBwave | **Trellis** |
|---|---|---|---|
| Platform | Windows-only, EOL | Win/Mac (mostly), cloud | **Win/Mac/Linux + web** |
| Openness | closed | closed | **open format, scriptable engine** |
| Engine | CPU, native+WPF | proprietary | **C++/GPU, headless, testable** |
| Capture | NDIS drivers | vendor HW | **external HW first + host-NIC** |
| Cost model | dead | premium | TBD (MSN open-core candidate) |

## Strategy alignment (clarified 2026-06-20)
The MSN locked strategy's "no planning / planning is hardware-defended" line
(`LICENSE_STRATEGY.md`, 2026-05-19) was **scoped to Seed** — i.e. *Seed* doesn't grow
into a survey/planning tool. A **separate** product (Trellis) owning Wi-Fi planning is
fully consistent with that: planning simply doesn't live in Seed.

**Decided (2026-06-20): all Wi-Fi moves out of Seed into Trellis.** Seed becomes
wired-diagnostics / security / compliance only; Trellis is the sole MSN Wi-Fi product
(live troubleshooting *and* survey/planning). Seed's `internal/wifi` (ex-`canopy`) is
deprecated and its live-troubleshooting capability is reimplemented in Trellis's Live
mode on the shared core. See `docs/08-SEED-TRELLIS-BOUNDARY.md`. **Follow-ups:** update
`LICENSE_STRATEGY.md` to name Trellis the Wi-Fi product and remove Wi-Fi from Seed's
scope; plan the `internal/wifi` removal in the Seed repo.

## Audience
WLAN engineers, integrators, and MSPs doing site design and validation surveys who
want a modern, cross-platform, scriptable tool without per-seat cloud lock-in.
