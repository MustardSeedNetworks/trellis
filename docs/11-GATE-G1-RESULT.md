# Gate G1 result — engine credibility

**First run:** 2026-09-07 on AirMagnet demo projects · **re-measured the same
day** on the AirMapper walks that carry AP placements (#362).

**Verdict: FAIL, confirmed on both datasets. The predictive engine is not
built.** 10.42 dB uncalibrated / 4.34 dB calibrated on AirMagnet; **16.75 dB
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
