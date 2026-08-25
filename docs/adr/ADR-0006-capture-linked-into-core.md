# ADR-0006 — Wi-Fi capture is linked into trellisd, not a separate daemon

Status: Accepted · Date: 2026-08-24 · Amends [ADR-0001](ADR-0001-process-isolated-architecture.md)
and [ADR-0002](ADR-0002-language-per-component.md); rewrites R5 in
[07-RISKS.md](../07-RISKS.md)

## Context

ADR-0001 made capture one of four cooperating processes, for two stated
reasons: **privilege isolation** ("capture runs as a separate privileged
helper") and **build hygiene** (cgo stays out of the Go core). `docs/07-RISKS`
R5 restated the second as "cgo confined to capture daemon".

Building the macOS backend (`internal/capture`, shipped in 0.1.29) showed the
first reason does not apply to what Trellis actually does, and that the split
carries a specific cost.

**On macOS, privilege is not the gate and cannot be.** The gate is TCC, and it
is *stricter* than root: macOS gives Wi-Fi network names only to a signed,
entitled bundle that LaunchServices attributes to that bundle identity. A root
daemon gets nothing a user-session bundle does not — it gets less. No amount of
privilege separation buys a single named BSSID.

**On Linux, privilege is real, and this ADR does not pretend otherwise.**
Measured on an RTL8723BU adapter:

| | |
|---|---|
| root, trigger scan | 11 BSSes |
| unprivileged, trigger scan | `Operation not permitted` (EPERM) |
| unprivileged, read cached results | 11 BSSes |

So *triggering* a scan needs `CAP_NET_ADMIN`; *reading* the cache does not. That
is a genuine argument for a privileged helper — **on Linux**. It is not an
argument for a process split on macOS, where the same split actively costs a
working permission model, nor on Windows, where Native Wifi scanning is
unprivileged.

Privilege also becomes real at Tier 2 (monitor mode) on every platform. Tier 2
is not implemented anywhere and, when it lands, is a different code path with a
different lifecycle.

**The split makes the macOS permission model harder, not easier.** A grant
belongs to a bundle. With capture split out, trellisd would have to launch the
capture bundle through LaunchServices — `open -W -a Bundle.app --args …` — for
every scan, because a direct exec of the inner binary registers as
`com.apple.locationd.executable-<path>`, a different client holding no grant.
That was verified against a notarized, stapled, authorized bundle: direct
execution returned 0 of 11 networks with a BSSID. Piping a measurement stream
back out of an `open`-launched process, at 0.25 Hz, is machinery whose only
purpose is to preserve the split.

**Nothing else about the split was being used.** trellisd is a loopback-only,
single-user process. There is no fault domain worth isolating between "serve the
survey API" and "read the radio", and no second consumer of the capture process.

## Decision

Link `internal/capture` directly into **trellisd**. Delete `cmd/trellis-capture`.
Ship **trellisd itself** as the signed, entitled macOS application bundle
(`deploy/macos/build-app.sh`, bundle id `net.mustardseed.trellis`), because the
process that reads the radio is the process that must hold the grant.

Confine cgo to `internal/capture`, behind `//go:build darwin && cgo`, and
enforce that with `scripts/check-cgo-confinement.py` in CI rather than by
convention.

This changes R5's *mechanism* (a process boundary → a package boundary plus a
CI check) and keeps its *goal* (cgo does not spread through the Go core).

## Why

- **The privilege argument does not hold where it was being applied.** It is
  false on macOS and on Windows, and where it *is* true — Linux scan triggering —
  it argues for one small platform-specific helper, not for every platform paying
  an IPC seam. Splitting all three because one might need it is the wrong default.
- **One process, one bundle, one grant.** The permission model becomes a
  property of how trellisd is launched, not a subprocess protocol to get right.
- **cgo hygiene survives as a package boundary.** `CGO_ENABLED=0 go build ./...`
  still compiles the whole tree, and CI now proves no package outside
  `internal/capture` touches a cgo dependency — which the old rule never
  actually checked.
- **The seam that mattered stays.** `capture.Scanner` is the abstraction the
  survey engine consumes; host-NIC and external hardware sit behind it either
  way. What is removed is a *process* boundary, not a *contract* boundary.

## Consequences

- The macOS release build sets `CGO_ENABLED=1` for darwin. A cgo-less macOS
  build compiles but reports `ErrUnsupported` and can never scan; `.goreleaser.yml`
  carries that constraint with the reason.
- trellisd must be launched as `Trellis.app` through LaunchServices to read
  network names. Launched from a terminal it still serves imported surveys and
  still scans — it just names nothing, and says so.
- Two consequences of a LaunchServices launch had to be handled in trellisd:
  the working directory is `/`, and stdout and stderr are `/dev/null`. Hence
  `internal/apppaths` — an absolute per-user data directory and a log file at
  `~/Library/Logs/Trellis/trellisd.log`.
- ADR-0001's "the Go core stays cgo-free (single static binary)" now holds for
  linux and windows only. The darwin binary is dynamically linked against system
  frameworks.
- The **engine** remains a separate process. ADR-0001's fault-isolation argument
  is untouched for C++/GPU code, which is where it was load-bearing.

## Alternatives rejected

- **Keep the split, launch the bundle per scan.** Preserves the diagram at the
  cost of an `open`-per-scan launch path, a stream to marshal back, and a
  failure mode (direct exec silently loses the grant) that has already cost real
  time once.
- **Keep the split, drop the bundle; run capture as root.** Does not work. Root
  is not what macOS checks; an unbundled root daemon sees nameless BSSIDs.
- **Wait for Tier 2 and split then.** Tier 2 needs privilege *and* takes the
  radio off the network, so it will want its own process with its own lifecycle
  regardless. Carrying an unused split until then is not a down payment on it.

## Open question for the Linux backend

`CAP_NET_ADMIN` has to come from somewhere: a file capability on trellisd, a
small capability-bearing helper that only triggers scans, or accepting
cached-only results — which are stale, and empty if nothing else on the host
ever scans. That choice belongs to the Linux backend's own ADR, decided against
a working implementation rather than in advance. `capture.Scanner` is the same
interface either way, which is the point: this ADR removes a *process* boundary,
not the *contract* boundary that would let Linux reintroduce a helper behind it.
