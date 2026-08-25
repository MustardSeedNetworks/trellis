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

Tier 1 is *mostly* unprivileged, with one measured exception — Linux needs
`CAP_NET_ADMIN` to trigger a scan, though not to read the cache. That is why the
privilege claim below is stated per platform rather than as a blanket.

Trellis implements Tier 1 on macOS, Linux and Windows. Tier 2 is not
implemented on any platform.

## Tier 1 — what is implemented

### macOS

Implemented, via CoreWLAN (`internal/capture/capture_darwin.go`), linked
directly into trellisd (ADR-0006).

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

Check a bundle's grant with `deploy/macos/location-status.py` (exit 0 authorized,
1 registered but not authorized, 2 unknown to locationd).

**Two consequences of launching through LaunchServices**, both measured, both
handled in `internal/apppaths`:

| | |
|---|---|
| Working directory | `/` — a relative data directory would resolve under the filesystem root |
| stdout and stderr | `/dev/null` — a bundled daemon logging to stdout is silent |

So the bundled daemon keeps its survey store at
`~/Library/Application Support/Trellis` and its log at
`~/Library/Logs/Trellis/trellisd.log`. That log is the only output it has; the
capture-readiness line at startup is where an operator sees whether this launch
can read network names.

Environment variables *do* reach the app when it is launched with `open` from a
shell (`TRELLIS_ADDR=… open -a Trellis.app` works), but not when it is started
from the Finder or the Dock, which inherit launchd's environment instead.

**Building and running it:**

```
npm --prefix ui ci && npm --prefix ui run build
./deploy/macos/build-app.sh 0.1.30
open -a dist/macos-app/Trellis.app
python3 deploy/macos/location-status.py
```

### Linux

Implemented, via nl80211 over generic netlink
(`internal/capture/capture_linux.go`). Pure Go through `mdlayher/genetlink`, so
R5's cgo boundary costs Linux nothing.

The backend is a scan request plus a scan-results notification, which is the
same shape as the macOS event and has no equivalent cache-window restriction. Linux also has the strongest monitor
mode support of the three platforms, so it is the natural first host for Tier 2.

**Privilege, measured** (RTL8723BU USB adapter):

| | |
|---|---|
| root, trigger scan | 11 BSSes |
| unprivileged, trigger scan | `Operation not permitted` (EPERM) |
| unprivileged, `scan dump` (cached) | 11 BSSes |

Triggering needs `CAP_NET_ADMIN`; reading the cache does not. Unlike macOS this
is a real privilege boundary, and where the capability comes from — a file
capability on trellisd, a minimal helper that only triggers, or cached-only
results — is open (ADR-0006, "Open question for the Linux backend").
Cached-only is not a free option: the cache is stale, and stays empty if nothing
else on the host ever scans. This backend therefore reports `ErrPermission`
rather than quietly serving the cache.

**One implementation note that cost real time.** The scan-complete notification
must be received on a *second* netlink socket. Multicast notifications carry
sequence number 0, so a group joined on the socket that sent the request
interleaves them with its own reply and netlink rejects the exchange with
`mismatched sequence in netlink reply`.

**Signal accuracy is the adapter's, not the backend's.** The RTL8723BU reports
−6 dBm for an AP a metre away. `iw` prints exactly the same value from the same
kernel data, so the backend is faithful; the adapter's absolute calibration is
not. A survey is a comparison between points, but a miscalibrated adapter still
makes absolute thresholds meaningless.

### Windows

Implemented, via the Native Wifi API (`internal/capture/capture_windows.go`):
`WlanScan`, `WLAN_NOTIFICATION_ACM_SCAN_COMPLETE`, then
`WlanGetNetworkBssList`. Pure Go through `golang.org/x/sys/windows` — no cgo.

**Windows 11 gates scanning on Location Services, like macOS.** This was not
expected: every summary of the Native Wifi API describes scanning as
unprivileged, and the Linux experience above sets you up to look for a
capability. Measured on Windows 11 build 26200, in a process running as
`nt authority\system` **holding administrator rights**:

| | |
|---|---|
| Location consent `Deny` | `WlanScan` → `ERROR_ACCESS_DENIED` |
| `netsh wlan show networks` | "Network shell commands need location permission to access WLAN information." |
| Machine consent set to `Allow`, `lfsvc` running, rebooted | still denied |
| Session state | `console` session present, **no user logged in** |

Elevation is not the missing piece — the process already had it. The consent is
granted **per user in an interactive session**, which a headless service session
cannot satisfy. That is structurally the same gate as macOS's TCC, reached by a
different mechanism, and it means trellisd must run in the surveyor's own
session on Windows too.

**Struct layout is a contract with the OS.** `WLAN_BSS_ENTRY` is read straight
out of memory `wlanapi` allocated. One wrong offset does not fail loudly: every
field after the mistake is read from the wrong bytes and the survey records
plausible nonsense. `TestWLANBSSEntryLayout` pins all sixteen offsets and the
360-byte size, and is run on real Windows rather than inferred.

**Cadence is unmeasured.** Windows historically rate-limits scans more
aggressively than Linux. That needs measuring on a host with a logged-in user,
which is the same thing the live scan needs.

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

Note the architectural consequence, which is the whole of ADR-0006. On macOS
Tier 1 needs **no** privilege — the gate is TCC, not root, and a root daemon gets
*less* than a user-session bundle, because TCC grants to a signed bundle
identity. Tier 2 needs privilege on every platform, and Linux needs it for Tier 1
scan triggering too. A separate, privileged capture process is therefore
justified by Tier 2, and arguably by Linux Tier 1 — but not by macOS Tier 1,
where it costs a working permission model and buys nothing. That is why capture
is linked into `trellisd` today rather than split out.

## What is honest to claim

- Coverage survey (RSSI, SNR, channel, band, width, security per BSS): **yes**,
  macOS today, Linux and Windows once their backends land.
- Continuous capture while walking: **yes on macOS, at ~3–4 s per sample.** Not
  a fast continuous walk.
- Channel utilisation, retries, airtime: **no**, on any platform, until Tier 2
  exists.
- Anything requiring the radio to leave the network: **no**, and it would need
  to be an explicit mode rather than a background capability.
