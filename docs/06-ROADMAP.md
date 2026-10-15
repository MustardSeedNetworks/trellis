# Trellis — Roadmap

Each phase ends in something **demoable** and has explicit exit criteria. No phase
starts before the prior phase's contract is frozen. Order is dependency-driven, not
feature-driven.

## Status (2026-10-15)

| Phase | Status | Evidence |
| --- | --- | --- |
| Phase 0 — Plan freeze | Done | `docs/` reviewed; contracts drafted. Only `proto/trellis/survey/v1/survey.proto` is implemented; `docs/contracts/api.proto`, `capture.proto` and `engine.proto` are still marked DRAFT and unimplemented. |
| Phase 1 — Migrate measured-survey | Done | `core/survey` in tree, corpus tests assert values; survey API real; UI wired to the live measured-survey workflow (#268); per-AP heatmap (#679) and per-floor AP table in the PDF (#677). |
| Phase 2 — Predictive model MVP | In progress | ADR-0011: the engine is `core/rf`'s log-distance model in Go on the CPU, the model Gate G1 scored; no C++ or GPU engine. Per-floor placement persistence is T-P1 (#822); the `predicted_rssi` heatmap (T-P3) and per-AP calibration (T-P5) are not yet wired. |
| Gate G1 — Engine credibility | Run 2026-09-07 — **failed**; re-scoped 2026-09-08; **re-scoped run 2026-10-14 — passed** (after the 2026-10-15 harness correction) | CPU log-distance model measured against surveyor-placed APs: 10.42 dB mean error uncalibrated, 4.34 dB calibrated, against the original thresholds of 6 and 3–4 dB. Re-scoped by the owner to **10 dB mean error against the Link-Live oracle**. The corrected run scored 510 radios on surveyor-placed APs across ten Link-Live surveys: **8.30 dB uncalibrated at the most favourable power (10 dBm), 11.15 dB at 20 dBm, 3.55 dB calibrated**. Pass. `docs/11-GATE-G1-RESULT.md`, `docs/12-CROSS-PRODUCT-ORACLE.md`, `core/rf`. |
| Phase 3 — Planning UX | Not started | Place, move and delete an AP through the API (T-P4) and on the Coverage page (UI-TRL-18). No Wails: the UI is a browser page served by `trellisd`, loopback HTTP by default (ADR-0010). |
| Phase 4 — Prediction vs. measurement | Not started | A measured-minus-predicted layer and its two PDF pages (T-P6). The GPU, walls and the SNR, data-rate and interference layers are closed by ADR-0011. |
| Phase 5 — Capture + survey loop | Partially done | Tier 1 host-NIC scan backends real on macOS (CoreWLAN), Linux (nl80211), Windows (Native WiFi), linked into `trellisd` (ADR-0006). Tier 2 monitor-mode acquisition landed on Linux, 2.4 GHz only, opt-in (#419, `docs/10-WIFI-CAPTURE.md`). External hardware and model calibration in the survey loop not started; `core/rf` calibration exists only as the Gate G1 measurement. |
| Phase 6 — Reporting, licensing, polish | Partially done | Pure-Go PDF reporter real (`core/survey/report.go`, fpdf). Releases ship deb and rpm installing a user-session daemon (ADR-0007, #318), plus Linux/macOS tarballs and a Windows zip, each with a Syft SBOM and cosign keyless signature (`.goreleaser.yml`). The macOS `.app` is code-signed by `deploy/macos/build-app.sh` but not notarized; no Windows installer; no Ed25519 licensing. |
| Phase 7 — Cloud/team | Not started | — |

## The bet (read before anything)

The measured-survey half is **proven Seed code we migrate** (`09-SEED-MIGRATION.md`),
and the plumbing (Go core, React UI, capture, cloud) is "just" engineering. The risk is
isolated to **one component: the _predictive_ RF engine's accuracy** — Ekahau's moat is
decades of propagation tuning so people trust a heatmap enough to deploy on it. So this
roadmap **inverts the risk**: migrate the measured tool first (it gives us real ground
truth), then prove the predictive engine against it (**Gate G1**) _before_ building the
full product. If predictions aren't trustworthy we've spent weeks, not a year, finding
out — don't build the cathedral around an unproven engine.

## Phase 0 — Plan freeze (now)

- Settle `docs/` (vision, PRD, architecture, tech stack, ADRs).
- **Freeze the three `.proto` contracts** (`engine`, `api`, `capture`) — these
  de-risk everything downstream.
- **Exit:** docs reviewed; `buf lint` clean; contracts tagged v1-draft.

## Phase 1 — Migrate Seed's measured-survey subsystem (reuse; low-risk)

Lift Seed's proven, tested survey code (`09-SEED-MIGRATION.md`) into Trellis. This is
mature Go, not greenfield.

- Move `seed/internal/wifi/survey/` (~3,868 LOC) into the Trellis **Go core**: AirMapper
  `.amp` import, interpolation, heatmap/colorscale, analysis, multi-floor, reports
  (keep tests). Reshape the data model to the Trellis project bundle + protobuf API.
- Port the AirMapper import + survey UI into the Trellis UI (Project mode).
- **Outcome:** a working **measured-survey tool** _early_ AND the **G1 ground-truth
  pipeline** (it already heatmaps the real 73-pt Everett survey).
- **Exit:** decode + interpolate + heatmap the real Everett `.amp` end-to-end in
  Trellis; migrated tests green.

## Phase 2 — Predictive model MVP — **the bet**

Gate G1 measured `core/rf`'s closed-form log-distance model and it passed, so that
model is the engine (ADR-0011): Go, CPU, no wall term, best server over a floor's
placed APs. The C++/GPU engine this phase used to name is not built.

- **T-P1** — AP and client placements persist, each on its floor, with a transmit
  power and band where the import states them.
- **T-P2** — ADR-0011 records the model and supersedes ADR-0004.
- **T-P3** — a `predicted_rssi` heatmap that returns the measured heatmap's grid, so
  the Coverage page, legend and PDF reuse unchanged. Golden and property tests; a
  floor with no scale or no placements is refused.
- **T-P5** — per-AP calibration from the floor's own walk with `rf.Fit`, falling
  back to the defaults with a reason when a fit is clamped or under-sampled.
- **Exit:** a calibrated mean error of 6 dB or less on the Link-Live oracle's
  surveyor-placed APs through the product path. Above 6 dB the phase stops after
  T-P5 (the kill criterion).

## 🚦 Gate G1 — Engine credibility (make-or-break; do NOT skip)

**Ran 2026-09-07 and failed, was re-scoped by the owner on 2026-09-08, and
passed the re-scoped run (2026-10-14, corrected 2026-10-15) — see
`docs/11-GATE-G1-RESULT.md`.** The section below is the gate
as it was originally specified; the result doc records what was actually
available to measure, what it measured, and the re-scoped bar that now applies.

We now have **real ground truth in hand** (Everett: 73 measured points + AP layout +
floorplan, decoded by the migrated pipeline).

- Feed the engine the Everett floorplan + AP layout + a wall model; predict RX power at
  each of the 73 survey points; **diff predicted vs. the measured values** (mean/95th-
  pct dB error; coverage-boundary agreement). Exercise **calibration**.
- _(Optional G1a)_ also diff against an AirMagnet/Ekahau predicted export of the same
  scene (model-vs-model) for an implementation sanity check.
- **PASS (as originally proposed):** ≤ ~6 dB mean error pre-cal, ≤ ~3–4 dB post-cal;
  coverage boundary within ~1 cell.
- **PASS (re-scoped 2026-09-08, owner):** ≤ 10 dB mean error against the Link-Live
  oracle's AP placements, replacing the 6 dB pre-calibration figure; post-calibration
  unchanged. Four-week box from the first run, closing 2026-10-05. **Fail stops the
  planner for good — there is no third re-scope.**
- **Pass:** green-light the full build. **Fail:** fix the model (or rethink the bet)
  _before_ the cathedral — the cheapest possible place to learn the truth.

## Phase 3 — Planning UX

- **T-P4** — create, move and delete a floor's placements through the survey API,
  behind the same CSRF wrapper and credential gate as the other survey writes.
- **UI-TRL-18** — on the Coverage page, place, drag and delete an AP, set its power
  and band, and pick predicted signal from the metric picker.
- No wall editor, no AP model catalogue, no Wails.
- **Exit:** drag an AP → predicted heatmap updates; toggle measured vs. predicted.

## Phase 4 — Prediction vs. measurement

- **T-P6** — a `predicted_delta` layer (predicted minus measured, per cell) and two
  PDF pages: the predicted map and the delta map with mean and p95 error.
- Closed by ADR-0011 until a measured result reopens them: GPU kernels, walls and
  materials, ray tracing, SNR, data-rate, interference and multi-floor leakage.
- **Exit:** the report shows where the prediction disagrees with the walk.

## Phase 5 — Capture + survey loop

- Capture: host-NIC backends **done** on all three platforms — macOS CoreWLAN,
  Linux `nl80211`, Windows Native Wifi — linked into trellisd by ADR-0006.
  External HW next.
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
- **Migrate before invent (Phase 1 before 2):** standing up Seed's measured tool first
  gives an early shippable _and_ hands the engine its G1 ground truth — so the risky
  engine is built against real data from day one.
- Predictive engine (Phase 2) and the migrated core can proceed in parallel once
  contracts freeze.
- UI can mock the API from the `.proto` before the core is done.
- The GPU is closed by ADR-0011: the closed-form model meets the latency budget on
  the CPU, and nothing measured says a heavier engine is more accurate.
