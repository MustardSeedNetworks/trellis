<!-- SPDX-License-Identifier: BUSL-1.1 -->

# Wi-Fi capture capabilities

What Trellis can actually measure on each host OS, and what it cannot. Written
from measurements on real hardware, not from vendor documentation.

## Two tiers, and why the distinction matters

Wi-Fi capture splits into two capabilities with very different costs. Conflating
them is how survey products end up promising data they cannot collect.

**Tier 1 — BSS scanning.** The OS scans and hands back a list of visible BSSs:
SSID, BSSID, RSSI, noise, channel, width, band, security. This is what a
coverage survey needs, and it is available on every platform without special
privilege.

**Tier 2 — frame capture (monitor mode).** The radio is put into monitor mode
and every 802.11 frame is captured. This is the only way to get channel
utilisation, retry rates, airtime, and per-frame rather than per-scan RSSI. It
requires elevated privilege everywhere and takes the interface off the network
while it runs.

Trellis implements Tier 1 today. Tier 2 is not implemented on any platform.

## Tier 1 — what is implemented

### macOS

Implemented, via CoreWLAN (`internal/capture/capture_darwin.go`).

**Cadence is the binding constraint.** Measured on macOS 27.0, Apple Silicon:

| | |
|---|---|
| Active scan (`scanForNetworks`) | 3–4 s |
| Repeat call inside the cache window | 0.04 s, **returns the same values** |
| Interval between genuinely fresh samples | ~2–6 s |

Calling faster does not sample faster — it re-reads a cache. The effective
ceiling is roughly **0.25–0.3 Hz**, set by the radio.

Continuous operation is possible and does not require polling:
`CWEventTypeScanCacheUpdated` fires whenever the scan cache updates, and
`cachedScanResults` then costs nothing. Measured caveat: the event fires only
when a scan actually occurs. Over 40 s with nothing scanning, zero events
arrived. So a continuous capture drives its own scan on a timer and treats the
event as a way to pick up samples other processes trigger, not as free
background telemetry.

**Practical consequence for survey UX:** at walking pace a fresh sample arrives
every few metres. Stop-and-measure is comfortably within the platform's
ability; fast continuous walking is not, and no amount of code changes that.

**Permission.** macOS hides SSID and BSSID from any process without Location
Services authorization, and a scan without it **does not fail** — it returns the
right number of networks, with correct RSSI and channel, and every identifier
emptied. Three conditions must all hold:

1. The process runs inside a signed `.app` bundle.
2. The bundle carries `com.apple.security.personal-information.location`.
   Without it, `locationd` registers the client and then declines to show the
   prompt at all, which is indistinguishable from macOS refusing to prompt.
3. The process is launched through LaunchServices. A directly executed inner
   binary registers as `com.apple.locationd.executable-<path>` — a *different*
   client from its bundle, holding no grant. Notarization does not change this;
   a notarized, stapled, authorized bundle still returned 0 of 11 networks with
   a BSSID under direct execution.

Any of the three missing produces the same silent result: a successful scan
naming nothing. `foundation/pkg/corewlan` treats a scan that found networks and
named none of them as `ErrLocationDenied`, because a real observation always
carries a BSSID.

Check a bundle's grant with `deploy/macos/location-status.py`.

### Linux

Not implemented. `capture.New()` returns `ErrUnsupported`.

The intended backend is nl80211 via netlink: a scan request plus a
scan-results notification, which is the same shape as the macOS event and has
no equivalent cache-window restriction. Linux also has the strongest monitor
mode support of the three platforms, so it is the natural first host for Tier 2.

### Windows

Not implemented. `capture.New()` returns `ErrUnsupported`.

The intended backend is the Native Wifi API: `WlanScan` plus
`WLAN_NOTIFICATION_ACM_SCAN_COMPLETE`. Windows historically rate-limits scans
more aggressively than Linux, so the cadence question needs measuring there
rather than assuming it matches macOS.

## Tier 2 — not implemented anywhere

Channel utilisation, retry rates and airtime are **not obtainable** from any
Tier 1 API on any platform. They require monitor mode.

- **macOS:** CoreWLAN exposes no monitor-mode API. Capture goes through BPF
  (`/dev/bpf*`, owned `root:access_bpf`), so it needs root or membership of
  `access_bpf` — the group Wireshark's ChmodBPF installer creates. Unverified on
  Apple Silicon hardware.
- **Linux:** a monitor-mode virtual interface via mac80211. Requires
  `CAP_NET_ADMIN` / `CAP_NET_RAW`.
- **Windows:** Npcap, with the weakest monitor-mode support of the three.

Note the architectural consequence. Tier 1 needs **no** privilege — on macOS the
gate is TCC, not root, and a root daemon gets nothing a user-session bundle does
not. Tier 2 needs privilege on every platform. A privileged capture process is
therefore justified by Tier 2 and not by Tier 1, which is why capture is linked
into `trellisd` today rather than split out.

## What is honest to claim

- Coverage survey (RSSI, SNR, channel, band, width, security per BSS): **yes**,
  macOS today, Linux and Windows once their backends land.
- Continuous capture while walking: **yes on macOS, at ~3–4 s per sample.** Not
  a fast continuous walk.
- Channel utilisation, retries, airtime: **no**, on any platform, until Tier 2
  exists.
- Anything requiring the radio to leave the network: **no**, and it would need
  to be an explicit mode rather than a background capability.
