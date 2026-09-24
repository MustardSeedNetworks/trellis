# Cross-product oracle: Link-Live and the EtherScope nXG

**Runs 2026-09-07 (Link-Live) and 2026-09-24 (EtherScope live view).** Plan
of record row T-B10.

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

## The EtherScope nXG live Wi-Fi view, 2026-09-24

The Live page is checked against the tester the same way: one airspace, two
independent radios reading it in the same minute. The EtherScope (Wi-Fi app,
BSSIDs view) and `trellisd` built from `65fc41d`
(v0.2.91-2), running on `dev-srv-ubuntu` with the Edimax EW-7611ULB
(`rtl8xxxu`, 2.4 GHz only, unassociated), were read between 10:55 and
11:00 UTC. Trellis was read through its `Scan` RPC — the call the Live page
polls — and through the Live page itself.

Only 2.4 GHz is compared. The Edimax cannot tune 5 GHz, so the EtherScope's
thirteen 5 GHz BSSIDs are out of reach for this adapter rather than missed by
Trellis.

Identifiers are redacted. This repository is public and a BSSID locates the
site it was heard at, so the screenshots, the raw `Scan` responses and the
unredacted table live in the private plans repository under
`artifacts/t-b10-etherscope-2026-09-24/`. Below, `C` is the site gateway, `A`
and `B` are the two access points the EtherScope groups the other radios into,
and the digit is the BSS on that radio.

| BSS | Channel ES / TR | SSID ES / TR | Signal ES / TR (dBm) | Width ES / TR | Security ES / TR | Utilization ES / TR |
| --- | --- | --- | --- | --- | --- | --- |
| C | 1 / 1 | named / same | −13 / −8 | 40 / 40 | WPA3-P + WPA2-P / WPA3 | 12 % / 12 % |
| A1 | 6 / 6 | hidden / hidden | −18 / −18 | not shown / 20 | WPA2-P / WPA2 | 18 % / 17 % |
| A2 | 6 / 6 | named / same | −18 / −18 | — / 20 | — / WPA2 | — / 17 % |
| A3 | 6 / 6 | named / same | −19 / −18 | — / 20 | — / WPA2 | — / 17 % |
| A4 | 6 / 6 | hidden / hidden | −19 / −18 | — / 20 | — / WPA2 | — / 17 % |
| A6 | 6 / 6 | named / same | −18 / −18 | — / 20 | — / WPA3 | — / 17 % |
| B1 | 11 / 11 | named / same | −72 / −64 | — / 20 | — / WPA2 | — / 18 % |
| B2 | 11 / 11 | hidden / hidden | −72 / −64 | not shown / 20 | WPA2-P / WPA2 | 18 % / 18 % |
| B3 | 11 / 11 | named / same | −72 / −62 | — / 20 | — / WPA2 | — / 18 % |
| B4 | 11 / 11 | hidden / hidden | −70 / −64 | — / 20 | — / WPA2 | — / 18 % |
| B6 | 11 / 11 | named / same | −71 / −68 | — / 20 | — / WPA3 | — / 18 % |

Signal is the EtherScope's BSSID list at 10:56 against the `Scan` at
10:56:06. Width, security and utilization come from the EtherScope's
per-BSSID detail page, opened for C, A1 and B2 (a dash means that page was not
opened). Only C's page prints a width, `1 (40 MHz, 1 - 5)`; the A1 and B2 pages
print the channel alone, so no EtherScope width is recorded for them.
Trellis's values are from its scans at 10:58:32 and 10:59:42, whose
utilization ranged 12–13 % on channel 1 and 17–20 % on channels 6 and 11.

**Agreement.** The EtherScope's channel filter counts 1, 5 and 5 BSSIDs on
channels 1, 6 and 11; Trellis heard the same eleven and no others. Every
channel, SSID and hidden-SSID flag agrees. The one width the EtherScope
printed (C, 40 MHz) agrees, and so does the QBSS channel utilization on
every BSS whose detail was opened. Signal agrees
to within 1 dB on channel 6. On channels 1 and 11 Trellis reads 3–10 dB
stronger. The two radios were not side by side and their antennas differ,
and the gap follows the transmitting AP rather than any one field of the
decode, so it reads as position and antenna. This run did not establish that.

**Disagreements.**

- **SNR — filed as #600.** nl80211 reports no noise figure with a scan, so
  the Linux backend assumes −95 dBm for every BSS. The EtherScope measured −91
  dBm for C and A1 and −89 dBm for B2, so Trellis's SNR is 4–6 dB high on
  every row (C 87 against 79 dB, A1 77 against 73, B2 27 against 18). B2
  crosses the Live page's 20 dB weak-link threshold. The tester calls that
  link weak, while Trellis would report a host joined to it as healthy. Nothing
  on the wire says the floor was assumed.
- **Security naming for transition mode — by design, not filed.** C offers
  SAE and PSK. The EtherScope lists both (`WPA3-P, WPA2-P`) and Trellis
  reports the strongest (`WPA3`), as `internal/capture/ie.go` documents and
  `ie_test.go` pins. That is a vocabulary choice. It is recorded here because
  an operator comparing the two screens will see it.
- **AP grouping — not a defect.** The EtherScope groups BSSIDs into seven
  APs by radio (B2 is listed under B1's AP). The Live page lists BSSes and has
  no AP concept, so the two do not disagree on anything the Live page claims.

Not covered: 5 GHz and 6 GHz (no adapter), an associated host (the Edimax was
not joined, so the Live page's connected-link verdict was read as "not joined"
and the SNR consequence above is derived, not observed), and the macOS and
Windows backends. The first needs the MT7925U named in `T-HW`.

## What the Link-Live run could not cover

- **NIAC's authored Wi-Fi model.** niac W1 has not been built, so there is
  nothing to read over the wire yet.
- **The active-walk fields.** `frAssocRSSI`, `frAssocChan` and the rest were
  chosen the same way the channel was, and the one active survey here yields no
  rows to compare. They remain unchecked.
- **Six Link-Live records** have no counterpart in the corpus: two Everett
  walks and the four Time Square walks. They are listed by the tool rather than
  quietly skipped.
