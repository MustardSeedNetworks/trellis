# Gate G1 result — engine credibility

**First run:** 2026-09-07 on AirMagnet demo projects · **re-measured the same
day** on the AirMapper walks that carry AP placements (#362) · **re-scoped
2026-09-08 by the owner** — see [The owner's decision](#the-owners-decision-2026-09-08)
· **re-scoped run 2026-10-14: PASS** after the harness correction of
2026-10-15 — see [The corrected run](#the-corrected-run-2026-10-15).

**Verdict against the thresholds this gate was originally specified with: FAIL,
confirmed on both datasets. The predictive engine is not built.** The gate has
since been re-scoped and is open again; the numbers below are unchanged and the
bar they are measured against is not.
 10.42 dB uncalibrated / 4.34 dB calibrated on AirMagnet; **16.75 dB
uncalibrated / 4.02 dB calibrated on AirMapper**, against thresholds of ≤6 dB
and ≤3–4 dB. The re-measurement is the one that counts, and it is
[below](#re-measurement-on-airmapper-ground-truth-2026-09-07).

`docs/06-ROADMAP.md` makes Gate G1 the make-or-break test before any predictive
engine work: predict received power at measured positions from a placed AP
layout, diff prediction against measurement, and pass only at **≤ ~6 dB mean
error before calibration and ≤ ~3–4 dB after**. This is that measurement.

## What was measured

A CPU-only log-distance path-loss model, `core/rf`:

```text
RSSI(d) = TxPower − (RefLoss1m + 10·n·log10 d)
```

There is no wall term. A floor plan in this codebase is a raster image with no
wall geometry, so the multi-wall model `docs/04-RF-ENGINE.md` specifies has
nothing to sum over. That absence is the finding, not a gap in the experiment:
it is what a predictive engine would have to make up for.

- **Uncalibrated** ("pre-cal"): `RefLoss1m` = free-space loss at the carrier the
  measurement reported, `n` = 3.0 (ITU-R P.1238 office), `TxPower` from the
  transmit power the survey project recorded for that radio. These are the
  numbers a planner starts from with no measurements. The pre-cal column
  inherits whatever the file says the power was — the six PLANNER walks are the
  same radio, and one of them records 10 mW where its siblings record 100,
  which is the whole of that walk's −10.22 dB bias.
- **Calibrated** ("cal"): least squares on `RSSI = a − 10n·log10 d` per AP, with
  `n` held inside the physical bounds 1.5–6.

Readings at or below −99 dBm are dropped: that is AirMagnet's scale bottoming
out rather than a measurement, and one voice walk writes 76 of them. Merged
exports are skipped, because a merge repeats the rows of the walks it was built
from and would weight that floor twice.

## The dataset, and why it is not the one the roadmap named

The roadmap says the Everett AirMapper walk hands the engine "73 measured
points + AP layout + floorplan". At the time this gate ran, no archive in the AirMapper
reference corpus appeared to carry a single AP placement — 48 archives, 0
placements — so the tripwire left behind here would say so if one ever did,
since an AirMapper archive brings a real metres-per-pixel scale with it and
would be the better ground truth.

> **Correction, 2026-09-07.** That tripwire fired, and it was right: the
> placements were in the archives all along. Three of the 48 carry 216 between
> them, under a `.serial` member the importer read from the wrong name
> (`docs/12-CROSS-PRODUCT-ORACLE.md`, defect 2). The paragraph below stands as
> the reason this gate ran on borrowed data, but the premise it rests on was a
> parser bug rather than a property of the corpus, and the channel of every
> imported observation was a band code as well (defect 1). **That
> re-measurement has since run (#362) and is recorded below; this section is
> kept as the record of the first run.**

The ground truth used instead is **AirMagnet Survey Pro demo projects**, where
the surveyor placed the APs on the plan by hand and the export carries those
placements. Reaching them needed one fix: an AP-placement row runs the BSSID
and the media type together in one column (`00:13:80:43:15:2F802.11a`), so
every placement in the corpus was stored under an address nothing could match.
That is fixed in this change.

Limits of this dataset, stated plainly:

- **Two buildings, three radios, 2013-era 802.11a/g gear.** Ten (walk, AP)
  results, but six of them are repeat walks of one floor with one AP.
- **The coordinate unit is not recorded.** An `.svd` carries positions and plan
  dimensions; the project file beside it says only `ScaleUnits=0`. Feet is the
  reading that makes the demo floors building-sized, and it is what the table
  below assumes. Reading them as metres instead moves the uncalibrated mean
  error to 12.44 dB and leaves the calibrated mean at 4.34 dB — because a
  constant scale error is a constant in log-distance and lands entirely in the
  fitted reference loss. Only the uncalibrated half depends on the unit.
- **AP height, antenna pattern and orientation are unknown**; distance is 2D.

## Result

`TRELLIS_SVD_CORPUS=… go test ./core/rf/ -run G1 -v`, error in dB:

| Walk / placed AP | pairs | pre-MAE | pre-bias | pre-p95 | cal-MAE | cal-p95 | n | med d (m) |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| PLANNER/PassiveSurvey2 | 40 | 10.22 | −10.22 | 15.13 | 3.24 | 4.97 | 2.82 | 22.8 |
| PLANNER/PassiveSurvey3 | 304 | 4.78 | 4.40 | 8.79 | 2.24 | 5.69 | 3.92 | 16.7 |
| PLANNER/PassiveSurvey4 | 84 | 3.84 | 2.67 | 10.38 | 2.89 | 7.37 | 3.54 | 10.4 |
| PLANNER/PassiveSurvey5 | 200 | 8.57 | 8.57 | 12.73 | 1.99 | 4.53 | 2.81 | 31.7 |
| PLANNER/PassiveSurvey6 | 100 | 6.52 | 6.52 | 10.41 | 1.46 | 3.42 | 6.00\* | 22.1 |
| PLANNER/PassiveSurvey7 | 141 | 6.59 | 6.59 | 9.41 | 1.67 | 4.25 | 4.46 | 27.4 |
| VOICE/PassiveSurvey1 `…A7:78:1F` | 368 | 20.09 | 14.65 | 37.89 | 12.48 | 24.94 | 1.50\* | 15.6 |
| VOICE/PassiveSurvey1 `…43:15:2F` | 410 | 18.53 | 18.53 | 31.81 | 5.03 | 12.16 | 1.50\* | 15.0 |
| VOICE/VoFiSurvey1 `…A7:78:1F` | 160 | 9.32 | −3.40 | 19.17 | 7.13 | 13.88 | 1.50\* | 25.2 |
| VOICE/VoFiSurvey1 `…43:15:2F` | 184 | 15.76 | 15.73 | 28.27 | 5.22 | 11.49 | 1.50\* | 9.7 |

`\*` the least-squares fit landed outside the physical exponent bounds and was
pulled back to them. Three of those five were **negative** unbounded — −2.72,
−3.49 and −0.07 — meaning the line through the measurements says the signal
gets stronger with distance. That is not a propagation model; on those walks
the model explains nothing. The sixth PLANNER walk fitted 8.15 and was pulled
back to 6.

**Mean over 10 walks: 10.42 dB uncalibrated, 4.34 dB calibrated.**

Against the roadmap thresholds (≤6 dB pre-cal, ≤3–4 dB post-cal): **both fail.**

## Reading the result

The two sites disagree completely, and that is the substance of the finding:

- The **PLANNER site** (one AP, six walks, one floor) is roughly where a
  wall-free model should be: 3.8–10.2 dB uncalibrated, 1.5–3.2 dB calibrated,
  fitted exponents 2.8–4.5. Calibration works there.
- The **VOICE site** is a rout: 9.3–20.1 dB uncalibrated, 5.0–12.5 dB
  calibrated, every fit unphysical. Dropping the censored readings changed it
  by about a decibel, so the sentinels are not the explanation — distance
  explains almost none of what those radios heard on that floor.

A model whose accuracy swings between "usable after calibration" and "no
relationship to the measurements" depending on the building is not a model a
predictive planner can be sold on. Adding walls is the obvious next move — and
is exactly the cathedral the gate exists to avoid starting on a hunch, since it
needs wall geometry, a material catalog, plan vectorisation and an editor before
it can be measured at all.

## Verdict and what would change it

**Fail. Phase 2, Phase 3 and Phase 4 are not opened; no planner rows are
filed.** Per `docs/06-ROADMAP.md` the alternative to a pass is to fix the model
or rethink the bet — and this repository does not have the ground truth to tell
those apart.

The measurement still to run before the gate is reconsidered is the one the
roadmap assumed it already had: **one floor walked with APs whose positions we placed
ourselves, at a scale we calibrated ourselves.** That is the same walk the
alpha's kill criterion needs (`T-KILL` in the plan of record), on the same
hardware. Until then, this result stands as a fail on borrowed data rather than
a verdict on the physics.

Re-running is one command per corpus; both corpora are the vendor's and neither
is committed:

```bash
TRELLIS_SVD_CORPUS=~/AirMagnet/DemoProjects go test ./core/rf/ -run G1 -v
TRELLIS_AMP_CORPUS=~/AirMapper/Survey go test ./core/rf/ -run G1AirMapper -v
```

## Re-measurement on AirMapper ground truth (2026-09-07)

The first run scored the model against AirMagnet demo projects because the
AirMapper reference corpus appeared to carry no AP placements. It did carry
them: three of the 48 archives hold 216 between them, under a `.serial` member
the importer read from the wrong name (#361). This is the same measurement on
that data.

It is the better ground truth on two counts the AirMagnet exports could not
offer. The archive states its own **metres per pixel**, so no coordinate unit
has to be assumed — the AirMagnet run had to read `ScaleUnits=0` as feet and
carry that as a stated limit. And the placements and the walk positions are in
the same pixel space by construction, so the join needs no registration step.

What it does not offer is transmit power: an AirMapper placement records none,
so every AP is scored uncalibrated at an assumed 20 dBm, which is what a
planner with this data would have to assume. That assumption is the largest
lever on the uncalibrated column, and it does not change the verdict — see the
sensitivity below.

### Dataset

| Archive | Walk positions | AP placements | Addressed | Joined | Scored (≥30 pairs) |
| --- | --- | --- | --- | --- | --- |
| `Utopia floor 1.amp` | 74 | 53 | 45 | 43 | 20 |
| `Utopia floor 2.amp` | 132 | 65 | 60 | 58 | 25 |
| `Link-Live_DIAMainHall_2021-12-09.amp` | 36 | 98 | 34 | 34 | 0 |

A placement labelled with an AP name rather than an address groups several
radios; joining it to one of them would score the wrong radio, so those are not
addressed and are not scored. The DIA Main Hall walk contributes nothing: 36
positions is too few for any one of its APs to reach 30 pairs. So the
re-measurement is **45 placed APs across two floors of one building** — a
narrower base than the AirMagnet run's two sites, and that is a limit of the
result, not a strength.

### Result

**Mean over 45 placed APs: 16.75 dB uncalibrated, 4.02 dB calibrated.**

Against the roadmap thresholds (≤6 dB pre-cal, ≤3–4 dB post-cal): **both fail.**

- **18 of the 45 fits (40%) landed outside the physical exponent bounds** and
  were pulled back to them. None were negative this time — the unbounded
  exponents ran 0.07 to 1.48, meaning the line through the measurements is
  nearly flat: over these walks, distance barely predicts what the radio heard.
  On the AirMagnet corpus the same symptom appeared as negative exponents at
  one site and reasonable ones at the other.
- **Calibration is uniformly large and uniformly helpful.** Every AP improved,
  the calibrated MAE spread is 2.2–7.5 dB, and the mean lands just outside the
  3–4 dB band the roadmap allows after calibration.
- **The uncalibrated column is dominated by the assumed transmit power**, as it
  must be when the file records none. Scoring the same data at 10, 15, 20 and
  23 dBm — `defaultTxPowerDBm` in `core/rf/g1_corpus_test.go` — gives
  pre-calibration means of 10.99, 13.45, 16.75 and 19.05 dB. The
  calibrated mean is 4.02 dB at every one of them, because the fit absorbs the
  intercept. **Even the most favourable assumption fails the ≤6 dB
  threshold by 5 dB.**

### What this changes

Nothing about the verdict, and one thing about its reasoning. The first run
called the fail "a fail on borrowed data rather than a verdict on the physics",
because the two AirMagnet sites disagreed completely. On AirMapper data with a
real scale the model is _consistently_ poor rather than site-dependent: 40% of
fits unphysically flat, and a calibrated error above the post-calibration
threshold on data where calibration is doing all the work.

The measurement that would settle it is unchanged and is still not this one:
one floor walked with APs whose positions we placed ourselves, at a scale we
calibrated ourselves, on our own hardware (`T-KILL` in the plan of record).
Two floors of one building, surveyed by someone else, with no recorded
transmit power and no AP heights, is a stronger fail than the first — not a
verdict on the physics.

## The owner's decision, 2026-09-08

The fail above exposed a wrong premise rather than settling the physics: the
ground truth the roadmap named did not exist where it said it did, and the
re-measurement ran on two floors of one building surveyed by someone else, with
no recorded transmit power and no AP heights. The choice put to the owner was
to re-run the gate against better ground truth, re-scope the target, or stop
the planner for good.

**The decision is to re-scope and re-run.** In the owner's terms:

- **Target: 10 dB mean error against the Link-Live oracle** — NetAlly's own
  decode of the same AirMapper archives, described in
  `docs/12-CROSS-PRODUCT-ORACLE.md`. The oracle supplies the AP placements, so
  the gate no longer rests on this repo's reader of a schema-less format being
  right about where the APs were.
- **Time box: four weeks from the first run.** The first run was 2026-09-07, so
  the box closes **2026-10-05**.
- **Pass → the Phase C planner rows are filed** (floorplan walls, materials, AP
  placement, predictive layers, calibration, GPU) and the predictive engine is
  built.
- **Fail → the planner stops for good. There is no third re-scope.**

Two things this decision leaves stated rather than settled, recorded here so a
reader does not have to guess and so the owner can correct either in one line:

1. **The original gate named two thresholds** — ≤6 dB mean error before
   calibration and ≤3–4 dB after. The re-scope names one number. Read plainly,
   10 dB replaces the 6 dB pre-calibration threshold and the post-calibration
   threshold is unchanged; that is how it is recorded here and in
   `docs/06-ROADMAP.md`. If a single combined bar was meant, the roadmap line is
   the place to correct it.
2. **"From the first run"** is read here as the first run of this gate,
   2026-09-07, rather than the first run under the re-scope, which has not
   happened.

This supersedes the paragraph above on one point only. That paragraph says the
measurement that would settle the gate is a floor walked with APs we placed
ourselves — `T-KILL` in the plan of record. The owner chose the oracle route
instead, and the re-scoped run is the one that decides. `T-KILL` remains worth
having as the alpha's kill criterion; it is no longer what G1 waits on.

The re-scoped run was made on 2026-10-14 and corrected on 2026-10-15; both are recorded next, the correction first.

## The corrected run, 2026-10-15

**Verdict: PASS.** Scored against the APs the surveyor placed, mean error is
**8.30 dB** uncalibrated at the most favourable power tried (10 dBm) and
**8.74 dB** at 15 dBm, under the 10 dB bar. At 20 dBm it is 11.15 dB. The
calibrated mean is 3.55 dB. This is the same decision rule the 2026-10-14 run
applied (the most favourable power across 10–23 dBm), now on the right APs.

### What was wrong with the 2026-10-14 run

It scored the wrong APs. Link-Live puts a placement on the floor plan for every
BSS it hears, neighbours' routers included, and labels each estimate with the
BSS address ("RuckusWi:58b633-c652d9"). The harness joined only those
address-labelled estimates and skipped every placement labelled with a name,
which is how the surveyor placed the site's own APs. At the Everett office the
four SonicWall APs (SSIDs `LRC`/`LRG`) are placed as `10.24.23.87`–`.90`, and
every radio of those APs reports that value as `apName`. At Time Square 9 the
site's 42 APs (SSIDs `FP Internal`/`FP Guest`) are placed by name, while 70 of
the 1,141 address estimates sit on one pixel. So the run measured distance to
neighbours' routers and to Link-Live's guesses. That is why 86% of its fits
pinned at the exponent floor: distance from a guessed position did not predict
what the tester heard.

The harness now joins a surveyor-named placement to every radio whose `apName`
equals the label, scores each radio separately, and ignores address-labelled
placements as estimates. Nothing else changed: the model, the 30-pair floor, the
calibration bounds and the uncalibrated defaults are those of the earlier runs.
Owner-confirmed site APs: Everett = SonicWall `LRC`/`LRG`; Time Square = the
dense, multi-tenant `FP` deployment.

```bash
TRELLIS_LL_ORACLE=~/data/linklive-oracle go test ./core/rf/ -run G1LinkLive -v
```

### Result

| Survey | Radios scored | Pre-cal MAE (20 dBm) | Calibrated MAE |
| --- | --- | --- | --- |
| Large retailer, survey 2 (`60957f54`) | 125 | 6.49 | 2.70 |
| MGM corporate (`62869b48`) | 77 | 15.56 | 3.11 |
| Everett office, AirCheck G2 (`60957e34`) | 70 | 12.18 | 2.80 |
| Utopia floor 2 (`628da44e`) | 60 | 9.55 | 6.40 |
| Utopia floor 1 (`628da423`) | 57 | 17.95 | 3.47 |
| Time Square, 9th floor (`65c694f8`) | 48 | 13.71 | 3.36 |
| Time Square, 10th floor (`65c69564`) | 32 | 10.44 | 4.11 |
| Everett office, 2021 (`60957e0f`) | 20 | 5.53 | 3.77 |
| Everett office, 2022 (`628da3f3`) | 12 | 4.62 | 3.50 |
| Home, floor 2 (`60957de1`) | 9 | 8.01 | 4.95 |
| **All, per radio** | **510** | **11.15** | **3.55** |

| Assumed transmit power | 10 dBm | 15 dBm | 20 dBm | 23 dBm |
| --- | --- | --- | --- | --- |
| Pre-cal mean MAE, all 510 radios | **8.30** | **8.74** | 11.15 | 13.38 |

201 of the 510 fits (39%, against 86% before) fell outside the physical
exponent bounds. The sites that miss most at 20 dBm are dense deployments whose
controllers run the radios low, which is what the power sweep shows. A planner
should take each AP's configured power rather than a flat 20 dBm.

## Through the product, 2026-10-15 (T-P5)

The predicted map now calibrates each placed radio from the floor's own walk
(ADR-0011). The same ten surveys were run through it: each became a floor
holding the walk and one placement per radio of every surveyor-named AP, and
`GenerateFloorHeatmap` drew the predicted map per band, as `GetHeatmap` does.
The product scores the same 510 radios as the run above. A radio with at least
30 readings is drawn calibrated. A fit whose exponent falls outside the
physical bounds is drawn at the nearest bound with its own reference loss
(owner decision 2026-10-15) and the response marks it clamped; fewer readings
fall back to the uncalibrated defaults (10 dBm, exponent 3) and the response
says why.

```bash
TRELLIS_LL_ORACLE=~/data/linklive-oracle go test ./core/survey/ -run G1LinkLiveProduct -v
```

| Survey | Radios | Clamped | Unclamped MAE | Drawn-map MAE |
| --- | --- | --- | --- | --- |
| Large retailer, survey 2 (`60957f54`) | 125 | 45 | 2.77 | 2.70 |
| MGM corporate (`62869b48`) | 77 | 42 | 3.23 | 3.11 |
| Everett office, AirCheck G2 (`60957e34`) | 70 | 24 | 3.09 | 2.80 |
| Utopia floor 2 (`628da44e`) | 60 | 14 | 6.65 | 6.40 |
| Utopia floor 1 (`628da423`) | 57 | 45 | 4.59 | 3.47 |
| Time Square, 9th floor (`65c694f8`) | 48 | 18 | 3.83 | 3.36 |
| Time Square, 10th floor (`65c69564`) | 32 | 5 | 4.02 | 4.11 |
| Everett office, 2021 (`60957e0f`) | 20 | 0 | 3.78 | 3.78 |
| Everett office, 2022 (`628da3f3`) | 12 | 0 | 3.50 | 3.50 |
| Home, floor 2 (`60957de1`) | 9 | 8 | 3.38 | 4.95 |
| **All, per radio** | **510** | **201** | **3.83** | **3.55** |

**Kill criterion: not tripped.** The map the product draws is **3.55 dB** from
the walk, under the phase's 6 dB line, and equal to the run above, which
scored the same clamped fits at their bound. The 309 radios whose exponent was
physical score 3.83 dB on their own.

The first cut of T-P5 drew a clamped fit on the defaults, which put the drawn
map at 6.28 dB: those 201 radios averaged about 10.0 dB there. A clamped
exponent is not a propagation model, but its reference loss tracks the walk's
level, and that is most of the error.

## The re-scoped run, 2026-10-14 (superseded by the correction above)

**Recorded verdict, since corrected: FAIL.** Mean error against the Link-Live oracle's AP placements is
**20.24 dB uncalibrated** at the 20 dBm transmit power every earlier run
assumed, and **12.61 dB** at the most favourable power tried. No assumption
reaches 10 dB. Per the 2026-09-08 decision, a fail stops the planner for good.

### What was measured

Everything is Link-Live's: the AP placements, the walk positions and readings
in the same floor-plan pixel space, and the scale (`floorPlanScalePpf` in its
`heatmap.json`). The gate no longer depends on this repo's reader of an
AirMapper archive, which is what the re-scope asked for. The owner fetched the
oracle with `scripts/linklive-fetch-oracle.sh` on 2026-10-14: 17 surveys from
the Gotlabs org. The data is the vendor's and is not committed.

```bash
TRELLIS_LL_ORACLE=~/data/linklive-oracle go test ./core/rf/ -run G1LinkLive -v
```

The model, the 30-pair floor, the calibration bounds and the uncalibrated
defaults (free-space loss at the measured carrier, n = 3.0) are unchanged from
the earlier runs. A Link-Live placement records no transmit power, so the
uncalibrated number is reported across 10–23 dBm. A placement labelled with an
AP name rather than an address is not joined, as before.

Ten of the 17 surveys score. The four active walks carry no placements. Survey
3 of the large retailer carries none either. The DIA Main Hall walk and the
second Resorts World tent are too short for any AP to reach 30 pairs.

### Result

| Survey | Placed APs scored | Pre-cal MAE (20 dBm) | Calibrated MAE |
| --- | --- | --- | --- |
| Time Square, 9th floor (`65c694f8`) | 276 | 23.61 | 3.96 |
| Time Square, 10th floor (`65c69564`) | 124 | 22.78 | 3.66 |
| Large retailer, survey 2 (`60957f54`) | 44 | 7.82 | 2.63 |
| Resorts World tent, survey 3 (`628da344`) | 27 | 7.04 | 3.37 |
| Utopia floor 2 (`628da44e`) | 25 | 19.00 | 4.05 |
| Utopia floor 1 (`628da423`) | 20 | 13.95 | 3.98 |
| Everett office, 2022 (`628da3f3`) | 18 | 14.37 | 3.18 |
| Everett office, 2021 (`60957e0f`) | 16 | 17.20 | 3.18 |
| Home, floor 2 (`60957de1`) | 6 | 13.65 | 3.26 |
| MGM corporate (`62869b48`) | 1 | 28.79 | 3.17 |
| **All, per placed AP** | **557** | **20.24** | **3.71** |

| Assumed transmit power | 10 dBm | 15 dBm | 20 dBm | 23 dBm |
| --- | --- | --- | --- | --- |
| Pre-cal mean MAE, all 557 APs | 12.61 | 16.04 | 20.24 | 22.98 |
| Pre-cal mean MAE, the two COS surveys (50 APs) | 6.43 | 5.94 | 8.52 | 11.08 |

- **The harness reproduces the earlier run.** The two Utopia floors are the
  archives the 2026-09-07 re-measurement scored from this repo's own reader.
  Scored from Link-Live's placements, their 45 APs average 16.76 dB against
  16.75 dB then.
- **The Time Square floors carry 400 of the 557 APs.** Weighting each survey
  equally instead gives 16.72 dB, which also fails.
- **The calibrated mean, 3.71 dB, is inside the 3–4 dB band, but it is not a
  propagation model.** 480 of the 557 fits (86%) fell below the physical
  exponent bound and were pulled back to 1.5. Over these walks, distance from
  the placed AP barely predicts what the tester heard, so the fit is a constant
  offset. That is the September finding, on twelve times the APs.

### The COS surveys

The owner named the two Colorado Springs surveys as the good ones when the
data was delivered: the home floor and the large retailer's survey 2. Survey 3
of the retailer has no placements. On those two, the uncalibrated mean is 5.9
to 11.1 dB across the power range, and 8.52 dB at 20 dBm, which is under the
bar.

This record does not take that as the verdict. Fifty APs on two floors, 44 of
them in one walk, is the narrowest base of any run. The selection was named
before this run, but the gate as decided reads "against the Link-Live oracle",
not a subset of it. If the owner meant the COS surveys to be the gate, the
verdict line above is the one place to change.

### What else the run found

`go run ./tools/linklive-oracle` against the delivered corpus now pairs 12
surveys and agrees on every position, signal and noise reading. It disagrees
on 400 of 113,263 channels, and those 400 are exactly the 6 GHz readings: the
archive reader returns a 6 GHz channel offset by 190, with no frequency.
That was filed as #812 and is fixed by #814. It does not touch the gate number, because the harness
takes the carrier from Link-Live. Before this run, the tool stopped at the
first 6 GHz record, because Link-Live writes those channels as `"97E"`.

The delivered archives are a different export from the September corpus and
carry no placements in `.serial`. So `TestG1AirMapperCorpusCarriesAPPlacements`
fails on them by design: there is nothing for the importer to read, and the
importer has not regressed.
