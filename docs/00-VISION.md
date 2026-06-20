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

## ⚠️ Strategy note (recorded honestly)
The MSN locked strategy (2026-05-19, `LICENSE_STRATEGY.md`) explicitly **removed**
predictive survey / AP-placement / heatmap design as a direction ("planning is
hardware-defended; don't compete with Ekahau/Hamina/iBwave"). Trellis **reverses that
position**. Standing it up is an owner-level strategy decision, made deliberately on
the strength of the AirMagnet deep-dive (we *can* build the engine). If this reversal
stands, `LICENSE_STRATEGY.md` should be updated to reflect Trellis as a sanctioned
product so the docs stop contradicting each other.

## Audience
WLAN engineers, integrators, and MSPs doing site design and validation surveys who
want a modern, cross-platform, scriptable tool without per-seat cloud lock-in.
