# Cross-product oracle: Link-Live and the EtherScope nXG

**Run 2026-09-07.** Plan of record row T-B10.

Trellis reads AirMapper archives that a NetAlly tester wrote. NetAlly reads the
same archives too, and keeps the result: every survey uploaded to Link-Live is
decoded there and the decoded measurements are served back. That makes
Link-Live an oracle in the strict sense — one input, two independent analyzers
— so a disagreement is a defect in one of them rather than a difference of
opinion.

This matters more here than a comparison usually would. The `.SurveyResult`
member of an AirMapper archive is protobuf with no schema shipped alongside it;
its field numbers were recovered by inspection and checked for plausibility.
Plausibility is a weak check, and one of the fields it cleared was wrong.

## What was compared

Eleven surveys — 776 walk positions and 85,690 readings — paired by floor-plan
name and point count between the Link-Live records and the AirMapper reference
corpus. The corpus is the
vendor's own material and is not committed to this repository; the comparison
is reproducible from the analysis ids below.

```bash
scripts/linklive-fetch-oracle.sh /tmp/ll-oracle
go run ./tools/linklive-oracle \
    -index /tmp/ll-oracle/index.json \
    -processed /tmp/ll-oracle \
    -corpus "$TRELLIS_AMP_CORPUS"
```

## Result after the two defects below were fixed

| Link-Live analysis | Unit | Mode | Points LL/TR | Placements LL/TR | Positions agree | BSSIDs common/disjoint | Signal agree | Noise agree | Channel agree |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `60957e0fadf95e0035140a35` | EtherScopeXG | passive | 73/73 | 40/0 | 73/73 | 116/0 | 4789/4789 | 4789/4789 | 4789/4789 |
| `60957e5eadf95e0035140a3f` | EtherScopeXG | active | 76/76 | 0/0 | 76/76 | 0/0 | 0/0 | 0/0 | 0/0 |
| `60957f54adf95e0035140a55` | EtherScopeXG | passive | 107/107 | 90/0 | 107/107 | 282/0 | 14810/14810 | 14810/14810 | 14810/14810 |
| `60957f7cadf95e0035140a59` | EtherScopeXG | passive | 64/64 | 0/0 | 64/64 | 356/0 | 10873/10873 | 10873/10873 | 10873/10873 |
| `615ddd08c32eda00368954ac` | EtherScopeXG | passive | 36/36 | 39/0 | 36/36 | 436/0 | 2260/2260 | 2260/2260 | 2260/2260 |
| `62869b48bd386100367f0354` | EtherScopeXG | passive | 70/70 | 29/0 | 70/70 | 147/0 | 5378/5378 | 5378/5378 | 5378/5378 |
| `628da3028d578e0036ae9620` | EtherScopeXG | passive | 21/21 | 41/0 | 21/21 | 343/0 | 5819/5819 | 5819/5819 | 5819/5819 |
| `628da34488af45003628f883` | EtherScopeXG | passive | 36/36 | 42/0 | 36/36 | 341/0 | 9544/9544 | 9544/9544 | 9544/9544 |
| `628da3f38d578e0036aeaf0d` | EtherScopeXG | passive | 87/87 | 40/40 | 87/87 | 111/0 | 5297/5297 | 5297/5297 | 5297/5297 |
| `628da4238d578e0036aeb3f3` | EtherScopeXG | passive | 74/74 | 53/53 | 74/74 | 262/0 | 10240/10240 | 10240/10240 | 10240/10240 |
| `628da44e8d578e0036aeb89d` | EtherScopeXG | passive | 132/132 | 65/65 | 132/132 | 288/0 | 16680/16680 | 16680/16680 | 16680/16680 |
| **11 surveys** | | | 776/776 | 439/158 | 776/776 | 2682/0 | 85690/85690 | 85690/85690 | 85690/85690 |

Every walk position, every BSSID, every signal, noise and channel reading
agrees. The totals row is the tool's own arithmetic, not a hand sum. The
active walk (`60957e5e`) records an association rather than an observation
list, so it contributes no readings — that is the archive's shape, not a miss,
and it means the fields this reader uses for an active sample are not covered
by this oracle at all.

Placements are reported side by side and read by a person rather than asserted
equal. Link-Live groups an access point's radios into one placement where the
archive lists one per BSS, and an operator can add placements in Link-Live's
own web UI after the upload, so its count is legitimately not the archive's.
Where the paired archive is the one carrying placements the two agree exactly
(40/40, 53/53, 65/65). The DIA row reads 39/0 because two archives of that
floor share a plan name and point count and the pairing takes the first: the
one it took carries no placements, while its sibling carries 98 against
Link-Live's 39 grouped ones.

## The same comparison before the fixes

| Link-Live analysis | Unit | Mode | Points LL/TR | Placements LL/TR | Positions agree | BSSIDs common/disjoint | Signal agree | Noise agree | Channel agree |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `60957e0fadf95e0035140a35` | EtherScopeXG | passive | 73/73 | 40/0 | 73/73 | 116/0 | 4789/4789 | 4789/4789 | 0/4789 |
| `60957e5eadf95e0035140a3f` | EtherScopeXG | active | 76/76 | 0/0 | 76/76 | 0/0 | 0/0 | 0/0 | 0/0 |
| `60957f54adf95e0035140a55` | EtherScopeXG | passive | 107/107 | 90/0 | 107/107 | 282/0 | 14810/14810 | 14810/14810 | 0/14810 |
| `60957f7cadf95e0035140a59` | EtherScopeXG | passive | 64/64 | 0/0 | 64/64 | 356/0 | 10873/10873 | 10873/10873 | 0/10873 |
| `615ddd08c32eda00368954ac` | EtherScopeXG | passive | 36/36 | 39/0 | 36/36 | 436/0 | 2260/2260 | 2260/2260 | 0/2260 |
| `62869b48bd386100367f0354` | EtherScopeXG | passive | 70/70 | 29/0 | 70/70 | 147/0 | 5378/5378 | 5378/5378 | 0/5378 |
| `628da3028d578e0036ae9620` | EtherScopeXG | passive | 21/21 | 41/0 | 21/21 | 343/0 | 5819/5819 | 5819/5819 | 0/5819 |
| `628da34488af45003628f883` | EtherScopeXG | passive | 36/36 | 42/0 | 36/36 | 341/0 | 9544/9544 | 9544/9544 | 0/9544 |
| `628da3f38d578e0036aeaf0d` | EtherScopeXG | passive | 87/87 | 40/0 | 87/87 | 111/0 | 5297/5297 | 5297/5297 | 0/5297 |
| `628da4238d578e0036aeb3f3` | EtherScopeXG | passive | 74/74 | 53/0 | 74/74 | 262/0 | 10240/10240 | 10240/10240 | 0/10240 |
| `628da44e8d578e0036aeb89d` | EtherScopeXG | passive | 132/132 | 65/0 | 132/132 | 288/0 | 16680/16680 | 16680/16680 | 0/16680 |
| **11 surveys** | | | 776/776 | 439/0 | 776/776 | 2682/0 | 85690/85690 | 85690/85690 | 0/85690 |

### Defect 1 — every channel was a band code

`frObsChannel` read field 16, chosen because "its values are all real 802.11
channel numbers". They are not channel numbers at all: field 16 holds 24 under
a 2.4 GHz BSS and 50 under a 5 GHz one. A 2.4 GHz AP on channel 8 imported as
"channel 28", which is not a channel.

Field 6 carries the channel and agrees with Link-Live on all 85,690
observations; field 16 agrees on none. Every co-channel and adjacent-channel
figure computed from an imported AirMapper survey was computed from band codes.

### Defect 2 — every AP placement was dropped

AirMapper writes the operator's AP placements at the top level of the `.serial`
sidecar under `apLocations`. The parser read them from a nested `locations.aps`
member, which no archive in the corpus carries, so every placement was dropped
in silence — 216 of them across 48 archives.

This is the ground truth Gate G1 went looking for and recorded as absent
(`docs/11-GATE-G1-RESULT.md`), which is why the gate ran on borrowed AirMagnet
data with an unrecorded coordinate unit. The tripwire that gate left behind is
what caught it, and it is inverted rather than deleted. Re-measuring the gate
on this ground truth is #362.

A placement's address is folded into its label: `BelkinIn:58ef68-09f907` is one
BSS, while `HT_LV5_W_Plaza_` names an AP whose radios were grouped and gets no
address, since choosing one of its BSSIDs would join measurements to the wrong
radio.

## What this run could not cover

- **The EtherScope nXG live Wi-Fi view.** The tester is the oracle for the Live
  analysis page, and it was not on the lab network on 2026-09-07: a ping sweep
  of the management subnet from `pvm01` answered on five hosts, none of them
  the unit, and VNC on 5900 was closed on both addresses recorded for it. A
  same-room, same-minute comparison cannot be manufactured from an absent
  radio, so that half of T-B10 stays open.
- **NIAC's authored Wi-Fi model.** niac W1 has not been built, so there is
  nothing to read over the wire yet.
- **The active-walk fields.** `frAssocRSSI`, `frAssocChan` and the rest were
  chosen the same way the channel was, and the one active survey here yields no
  rows to compare. They remain unchecked.
- **Six Link-Live records** have no counterpart in the corpus: two Everett
  walks and the four Time Square walks. They are listed by the tool rather than
  quietly skipped.
