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

## What Trellis is
A Wi-Fi **survey + predictive planning + design** platform:
- **Plan**: import floorplan, model walls/materials, place APs/antennas, compute
  coverage/SNR/data-rate/interference heatmaps, multi-floor, validate against
  requirements (e.g. -67 dBm for voice).
- **Survey**: walk a site, capture real measurements tied to location, build measured
  heatmaps.
- **Calibrate**: fit predictions to measurements so plans are trustworthy.
- **Report**: templated, versionable PDF/HTML output.

## What Trellis is *not* (scope discipline)
- Not a packet analyzer / troubleshooting tool (that's Seed's Wi-Fi lane).
- Not a network simulator (that's NIAC / "Greenhouse").
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

**Open direction (under consideration):** consolidate **all** Wi-Fi capability into
Trellis — potentially pulling Seed's live Wi-Fi troubleshooting/visibility (the former
`internal/canopy` → `internal/wifi`) over too, leaving Seed focused on wired
diagnostics / security / compliance. The clean way to do that is to **share a Wi-Fi/RF/
capture core** between products rather than duplicate it — see `docs/08-SEED-TRELLIS-BOUNDARY.md`.
`LICENSE_STRATEGY.md` should be updated to name Trellis as the sanctioned Wi-Fi product
once the boundary is settled.

## Audience
WLAN engineers, integrators, and MSPs doing site design and validation surveys who
want a modern, cross-platform, scriptable tool without per-seat cloud lock-in.
