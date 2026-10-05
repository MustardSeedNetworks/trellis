<!-- SPDX-License-Identifier: BUSL-1.1 -->

# T-KILL walk script

The alpha's kill criterion, in the plan of record: a Trellis walk of a floor
must produce a heatmap that matches an AirMapper walk of the same floor. No
fixture can stand in for that. One operator walks one floor twice, once with
each tool, and `tools/walk-compare` scores one walk against the other.

Owner's walk: one hotel floor, week of 2026-10-05.

## What you need

- **The AirMapper device** (AirCheck G3, EtherScope nXG or CyberScope) with a
  Link-Live account to pull the survey back as an `.amp` archive.
- **The Mac** running Trellis as a signed `.app`. The release tarball's bare
  binary will not do: macOS hides every SSID and BSSID from a process that is
  not a signed bundle launched through LaunchServices, and the scan still
  succeeds with every name blank (`docs/10-WIFI-CAPTURE.md`, "Permission").
  The Edimax adapter is in pvm01, and nothing else Trellis drives is portable.
- **A floor plan image** of the floor (PNG or JPEG), with one known distance on
  it: a corridor length, or a wall you can measure with a tape.
- **Floor tape** and a printed copy of the plan.

## 1. Floor prep (before walking)

1. Choose 8 to 12 waypoints the walk passes through: corridor ends,
   intersections, lift lobby, stair doors. Each one must be a spot you can
   find on the plan to within a metre.
2. Put a numbered tape mark on the floor at each waypoint. Mark the same
   numbers on the printed plan.
3. Draw one route on the printed plan through every waypoint in order, along
   corridors and through any open public areas. Both walks follow this route.
4. Note the calibration distance: two points on the plan and the measured
   metres between them.

## 2. Build and check Trellis on the Mac

From a worktree of `main` (the build needs the UI first):

```bash
npm --prefix ui ci && npm --prefix ui run build
./deploy/macos/build-app.sh <version>
open -a dist/macos-app/Trellis.app
python3 deploy/macos/location-status.py   # must exit 0
```

`~/Library/Logs/Trellis/trellisd.log` must show `capture ready` with a
non-zero `networks` count, not `capture permission incomplete`. Do this at
home first: a Location prompt on the hotel floor wastes the walk.

## 3. AirMapper walk (first)

AirMapper goes first because its archive carries the floor plan, and Trellis
must use that same image (step 4).

1. New AirMapper passive survey, using the floor plan image. Calibrate with the
   distance from step 1.4.
2. Start at waypoint 1. Walk the route at a slow, steady pace, tapping your
   position on the plan at every waypoint as you reach it.
3. Finish at the last waypoint and save. Note the start and finish times.
4. Upload to Link-Live and download the survey as an `.amp` archive.

## 4. Trellis walk (straight after)

1. Take the plan image out of the archive, so both walks share one pixel
   space. AirMapper may resize what it was given; this copy is what its
   positions are measured in:

   ```bash
   unzip -l floor.amp                        # find the .png or .jpg member
   unzip -j floor.amp '<plan member>' -d .
   ```

2. In Trellis: **New survey**, upload that image as the floor plan, and
   **Calibrate** it with the same two points and distance.
3. **Start walk**, then **Start walking**. Stand at waypoint 1 and click it on
   the plan. Walk the same route at the same pace, and click each waypoint on
   the plan as you reach it. The readings between two clicks are placed along
   the line between them, so a missed click bends the walk.
4. The Mac yields a fresh reading about every 6.6 s. Pause about 10 s at each
   waypoint so each one gets a reading of its own.
5. **Stop walking** at the last waypoint, then **Complete survey**. Note the times.

Walk the two back to back, within the same hour: the comparison treats the
air as unchanged between them.

## 5. Store both walks

The corpus is not committed: it names the hotel's networks, and the reference
corpora stay outside the repository for the same reason
(`docs/11-GATE-G1-RESULT.md`). Quit Trellis first, so the store is not being
written, then copy the store and the archive into one directory per walk:

```bash
walk=~/Trellis-Corpus/t-kill/2026-10-0X-<hotel>-floor<N>
mkdir -p "$walk"
cp floor.amp "$walk/"
cp -R ~/Library/Application\ Support/Trellis "$walk/store"
```

Add a `notes.md` beside them: date, start and finish times of each walk, the
route, the calibration distance, the AirMapper device and the Mac model.

## 6. Compare

```bash
go run ./tools/walk-compare -amp "$walk/floor.amp" -store "$walk/store" \
  -survey "<Trellis survey name>"
```

It refuses two walks drawn on different images, and an AirMapper plan with no
scale. Each Trellis reading is paired with the nearest AirMapper reading within
2 m, and every BSSID both heard there is one pair. Per band, it prints:

| Column | Meaning |
| --- | --- |
| Pairs | BSSID readings both tools took at the same spot |
| Bias | Mean of Trellis minus AirMapper: the two radios' calibration offset |
| MAE | Mean absolute difference |
| MAE less bias | The same with the offset removed: what the capture path adds |
| p95 less bias | 95th percentile of the offset-removed difference |
| Missed | BSSIDs AirMapper heard at -75 dBm or stronger that Trellis did not report at that spot |

## Reading the result

The plan says "within corpus tolerances" and never defined them. **Proposed
bar, for the owner to confirm or change before the walk:** in every band with
at least 100 pairs, MAE less bias ≤ 5 dB, p95 less bias ≤ 10 dB, and Missed
≤ 10%.

Bias is reported, but it does not fail the walk. Two radios disagree by a
constant (the Edimax reads -6 dBm a metre from an AP). A constant offset is a
calibration matter. Scatter around the offset, or APs Trellis cannot see, means
the capture path is wrong.

A fail stops the alpha: report it before any further survey work, per the
kill criterion. A pass, with both walks stored as above, meets T-KILL's
acceptance.

If the AP positions on the floor are also marked on the plan, the same walk is
the self-placed ground truth Gate G1 named (`docs/11-GATE-G1-RESULT.md`).
