# Trellis

**MSN's Wi-Fi survey and planning product**, also carrying Wi-Fi troubleshooting
alongside Seed. Glance at live signal/SNR/neighbor-APs, or open a project: floorplan +
walls, AP placement, coverage/SNR/data-rate/interference heatmaps, walk-surveys, and
calibration. A modern, cross-platform successor to AirMagnet Survey/Planner (EOL) and a
source-available alternative to Ekahau / Hamina / iBwave. **Both Seed and Trellis carry
Wi-Fi analysis; Trellis owns survey/planning** (decided 2026-09-03, superseding the
2026-06-20 "Seed exits Wi-Fi" decision).

> Status (2026-09-04): pre-alpha, one binary (`trellisd`). **Exists:** the measured-
> survey engine migrated from Seed (`core/survey`, ~92% covered), the survey Connect/gRPC
> API (`proto/trellis/survey/v1`), a five-page UI (Surveys, Import, Coverage, Live, Reports)
> wired to the live measured-survey workflow, and per-OS scan capture (macOS CoreWLAN,
> Linux nl80211, Windows Native WiFi — passive/active scan, no monitor mode). **Does
> not exist:** the predictive C++/GPU RF engine (Gate G1 has never run), licensing, and
> the desktop shell (the macOS build is a hand-rolled bundle serving loopback HTTP, not
> Wails). The product/architecture plan lives in `docs/`; see `docs/06-ROADMAP.md` for
> a phase-by-phase status table.

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

```
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

```
sudo apt install ./trellis_<version>_amd64.deb     # or: sudo dnf install ./trellis-<version>-1.x86_64.rpm
trellisd                                            # or, to have it come back after a reboot:
systemctl --user enable --now trellisd
```

Then open <http://127.0.0.1:8446>. Triggering a Wi-Fi scan on Linux needs one
more opt-in step — see [docs/10-WIFI-CAPTURE.md](docs/10-WIFI-CAPTURE.md).

macOS runs from the signed bundle built by `deploy/macos/build-app.sh`; only a
signed, entitled bundle can read Wi-Fi network names. Windows ships as an
archive.

## Docs (read in order)
| Doc | What |
|---|---|
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
```
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
