# Trellis

**MSN's Wi-Fi survey and planning product**, also carrying Wi-Fi troubleshooting
alongside Seed. Glance at live signal/SNR/neighbor-APs, or open a project: floorplan +
walls, AP placement, coverage/SNR/data-rate/interference heatmaps, walk-surveys, and
calibration. A modern, cross-platform successor to AirMagnet Survey/Planner (EOL) and a
source-available alternative to Ekahau / Hamina / iBwave. **Both Seed and Trellis carry
Wi-Fi analysis; Trellis owns survey/planning** (decided 2026-09-03, superseding the
2026-06-20 "Seed exits Wi-Fi" decision).

> Status (2026-09-14): pre-alpha, one binary (`trellisd`). **Exists:** the measured-
> survey engine migrated from Seed (`core/survey`, ~92% covered), the survey Connect/gRPC
> API (`proto/trellis/survey/v1`), a five-page UI (Surveys, Import, Coverage, Live, Reports)
> wired to the live measured-survey workflow, and per-OS scan capture (macOS CoreWLAN,
> Linux nl80211, Windows Native WiFi — passive/active scan on all three, plus opt-in
> monitor-mode acquisition on Linux 2.4 GHz only; see `docs/10-WIFI-CAPTURE.md`).
> **Does not exist:** the predictive C++/GPU RF engine — Gate G1 ran 2026-09-07 and
> failed its original thresholds, and the owner re-scoped it on 2026-09-08 to 10 dB mean
> error against the Link-Live oracle, which the corrected run of 2026-10-15 passes
> (`docs/11-GATE-G1-RESULT.md`) — licensing, and the desktop shell (the
> macOS build is a hand-rolled bundle serving loopback HTTP, not Wails). The product/
> architecture plan lives in `docs/`; see `docs/06-ROADMAP.md` for a phase-by-phase
> status table.

## Why "Trellis"

A trellis is a structure you **deliberately design before growth** — the right
metaphor for site design and AP layout. Fits the MSN botanical fleet (Seed, Stem).

## The shape (one breath)

The target shape: a pure-function **C++/GPU RF engine** (scene → grids, golden-tested)
behind a **Go** core that owns domain/DB/orchestration/capture/licensing and serves one
protobuf API to a **React/TypeScript** WebGL UI — four isolated processes, schema-first
seams, shared-memory for big buffers, same Go core running desktop (Wails) or cloud.
Today it is one binary (`trellisd`): Go core + linked-in capture + embedded UI, no
engine process, no shared memory, no Wails.

```text
React+TS UI ──Connect/gRPC + binary grids──► Go core ──shmem/Arrow──► C++/GPU RF engine (planned)
                                               │
                                               ├─ SQLite (project) — Parquet (planned)
                                               ├─ internal/capture (Go, per-OS Wi-Fi radio)
                                               └─ Reporter (Go, pure-Go PDF via fpdf; headless Chromium planned)
```

## Install

Linux packages carry the daemon and a systemd **user** unit that ships
disabled — Trellis serves the one operator who runs it, on loopback, storing
surveys in their own data directory, so there is no system service and no
service user ([ADR-0007](docs/adr/ADR-0007-linux-packaging-and-service-contract.md)).

```bash
sudo apt install ./trellis_<version>_amd64.deb     # or: sudo dnf install ./trellis-<version>-1.x86_64.rpm
trellisd                                            # or, to have it come back after a reboot:
systemctl --user enable --now trellisd
```

Then open <http://127.0.0.1:8446>. Triggering a Wi-Fi scan on Linux needs one
more opt-in step — see [docs/10-WIFI-CAPTURE.md](docs/10-WIFI-CAPTURE.md).

### macOS (Apple silicon only)

Trellis for macOS runs on Apple silicon (arm64) only; there is no Intel build.
Install the notarized package from the release page:

```bash
sudo installer -pkg trellis-<version>-darwin-arm64.pkg -target /     # or double-click it
open -a Trellis
```

The package installs `/Applications/Trellis.app` and nothing else: no launchd
job, no postinstall, no open port, mirroring the Linux contract above. Start it
with `open -a Trellis` and open <http://127.0.0.1:8446>. To have it start at
login, add Trellis under System Settings > General > Login Items. Always launch
it as the app: macOS gives Wi-Fi network names only to the signed bundle, and
only after you allow its Location Services prompt
([docs/10-WIFI-CAPTURE.md](docs/10-WIFI-CAPTURE.md)). The bare `trellisd` in
the `.tar.gz` serves imported surveys but cannot name the networks it scans.

Uninstall with `sudo /Applications/Trellis.app/Contents/Resources/uninstall.sh`.
Surveys in `~/Library/Application Support/Trellis` are never deleted; remove
that folder yourself if you want them gone.

### Windows

Extract `trellis-<version>-windows-amd64.zip` (or `-arm64`) and run `install.ps1`
from an elevated PowerShell. It installs `trellisd.exe` to
`C:\Program Files\Trellis` and registers a Windows service named `Trellis`
that runs as its own virtual account (`NT SERVICE\Trellis`, never LocalSystem),
keeps surveys and its log under `%ProgramData%\Trellis` (owned by Administrators,
writable only by SYSTEM, Administrators and the service), and is **neither
started nor set to start at boot**, the counterpart of the disabled Linux user
unit:

```powershell
.\install.ps1                  # install, or upgrade in place
trellisd service start         # then open http://127.0.0.1:8446
trellisd service status        # also: stop, uninstall
sc config Trellis start= auto  # optional: start with Windows
.\install.ps1 -Uninstall       # surveys under %ProgramData%\Trellis are kept
```

The service serves imported surveys and reports but **cannot scan Wi-Fi**:
Windows gates WLAN scanning on a per-user Location grant in an interactive
session, which a service never has (#152). To survey with a live adapter, run
`trellisd.exe` from your own session. `TRELLIS_ADDR`, `TRELLIS_DATA_DIR`,
`TRELLIS_CAPTURE_MODE` and `TRELLIS_TLS_*` set in the shell that runs
`trellisd service install` are copied into the service. `TRELLIS_AUTH_*` is
refused, not dropped: a service's environment is readable by every local user,
so `service install` fails if a credential is set, and a routable
`TRELLIS_ADDR` is refused with it. The service is therefore loopback-only
without a login. It will not start if its data directory is not owned by
SYSTEM or Administrators, grants write access to anyone else, or is a junction.

## Docs (read in order)

| Doc | What |
| --- | --- |
| [docs/00-VISION.md](docs/00-VISION.md) | Problem, positioning, what it is / isn't |
| [docs/01-PRD.md](docs/01-PRD.md) | Functional + non-functional requirements, MVP scope |
| [docs/02-ARCHITECTURE.md](docs/02-ARCHITECTURE.md) | System architecture — processes, components, seams, data flows |
| [docs/03-TECH-STACK.md](docs/03-TECH-STACK.md) | Language per component + libraries + rationale |
| [docs/04-RF-ENGINE.md](docs/04-RF-ENGINE.md) | Propagation models, derived layers, calibration, GPU |
| [docs/05-DATA-MODEL.md](docs/05-DATA-MODEL.md) | Project bundle format, schema, measurement storage |
| [docs/06-ROADMAP.md](docs/06-ROADMAP.md) | Phased build plan with exit criteria |
| [docs/07-RISKS.md](docs/07-RISKS.md) | Risks + mitigations |
| [docs/08-SEED-TRELLIS-BOUNDARY.md](docs/08-SEED-TRELLIS-BOUNDARY.md) | Wi-Fi boundary — both products carry analysis, Trellis owns survey/planning (decided 2026-09-03) |
| [docs/09-SEED-MIGRATION.md](docs/09-SEED-MIGRATION.md) | Seed survey subsystem migration inventory (measured = reuse; engine = new) |
| [docs/adr/](docs/adr/) | Architecture Decision Records |
| [docs/contracts/](docs/contracts/) | Protobuf seam definitions (the keystone) |

## Repo layout (as of 2026-09-03)

```text
cmd/trellisd     Go. Single binary entrypoint.
core/            Go. Domain: core/survey (measured-survey engine), core/wifi (scan model).
internal/api     Go. Connect/gRPC handlers for the survey API.
internal/capture Go, per-OS Wi-Fi scan backends (macOS/Linux/Windows). The only cgo bits.
internal/i18n    Go. UI string catalog.
proto/           .proto (single source of truth) → codegen Go/TS. trellis/survey/v1 implemented.
ui/              TypeScript + React. Four pages: Surveys, Import, Coverage, Reports.
docs/            this.
deploy/          Packaging (macOS app bundle, etc.).
```

`/engine` (C++/GPU RF engine) and `/reporter` (headless-Chromium reporter) from the
target architecture are not started — the reporter that exists today is pure-Go
(`core/survey/report.go`, fpdf).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Report vulnerabilities privately, per
[SECURITY.md](SECURITY.md).

## License

Trellis is source-available under the [Business Source License 1.1](LICENSE),
not an OSI-approved license. The Additional Use Grant permits any use except
offering the Licensed Work to third parties as a Wi-Fi Survey Service, as
defined in the LICENSE file. The work converts to Apache-2.0 on the Change Date
stated there (2030-08-01).

For commercial licensing inquiries: `kris.armstrong@icloud.com`.
