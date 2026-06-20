# Trellis

**The MSN Wi-Fi product** — live troubleshooting **and** survey + predictive planning &
design. Glance at live signal/SNR/neighbor-APs, or open a project: floorplan + walls,
AP placement, coverage/SNR/data-rate/interference heatmaps, walk-surveys, and
calibration. A modern, cross-platform successor to AirMagnet Survey/Planner (EOL) and an
open alternative to Ekahau / Hamina / iBwave. **All MSN Wi-Fi lives here — Seed exits
Wi-Fi** (decided 2026-06-20).

> Status: **design phase — docs first, no code yet.** This repo currently holds the
> product/architecture plan. Code starts only when the plan in `docs/` is settled.

## Why "Trellis"
A trellis is a structure you **deliberately design before growth** — the right
metaphor for site design and AP layout. Fits the MSN botanical fleet (Seed, Stem).

## The shape (one breath)
A pure-function **C++/GPU RF engine** (scene → grids, golden-tested) behind a **Go**
core that owns domain/DB/orchestration/capture/licensing and serves one protobuf API
to a **React/TypeScript** WebGL UI — four isolated processes, schema-first seams,
shared-memory for big buffers, same Go core running desktop (Wails) or cloud.

```
React+TS UI ──Connect/gRPC + binary grids──► Go core ──shmem/Arrow──► C++/GPU RF engine
                                               │
                                               ├─ SQLite (project) + Parquet (survey pts)
                                               ├─ Capture daemon (Go, per-OS / external HW)
                                               └─ Reporter (Go + headless Chromium)
```

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
| [docs/08-SEED-TRELLIS-BOUNDARY.md](docs/08-SEED-TRELLIS-BOUNDARY.md) | Wi-Fi boundary — all Wi-Fi → Trellis (decided) |
| [docs/09-SEED-MIGRATION.md](docs/09-SEED-MIGRATION.md) | Seed survey subsystem migration inventory (measured = reuse; engine = new) |
| [docs/adr/](docs/adr/) | Architecture Decision Records |
| [docs/contracts/](docs/contracts/) | Protobuf seam definitions (the keystone) |

## Planned repo layout (once code starts)
```
/engine    C++20 + GPU compute (RF math). Clean C ABI. CPU fallback.
/core      Go. Domain, SQLite/Parquet, orchestration, API, licensing.
/ui        TypeScript + React + WebGL. Desktop (Wails) and web.
/capture   Go, per-OS backends + external-HW. The only cgo component.
/contracts .proto (single source of truth) → codegen Go/TS/C++.
/reporter  Go + headless renderer → PDF.
/docs      this.
```
