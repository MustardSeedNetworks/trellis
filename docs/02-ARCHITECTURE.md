# Trellis — System Architecture

Status: draft for review · Owner: Kris Armstrong / MSN

## 1. The keystone decision: process-isolated, local-first, transport-agnostic core

Trellis is **not** a monolith. It is three cooperating OS processes, with the Go core
written so the **same binary** runs embedded in the desktop app *or* as a cloud
server. This one choice buys us:

- **Fault isolation** — a C++/GPU crash or a hung GPU driver can't take down the app.
- **Clean builds** — C++ builds independently; cgo is confined to
  `internal/capture` and CI proves it (docs/07-RISKS R5). linux and windows
  builds are cgo-free static binaries; darwin links system frameworks.
- **Fast big-buffer path** — heatmap grids move between Go and the engine by
  **shared memory**, never JSON.
- **One codebase, desktop *and* cloud** — the core's API is transport-agnostic.

**Capture is not one of the processes** (ADR-0006). It was, on a privilege-isolation
argument that turned out not to apply: Tier 1 scanning is unprivileged on every
platform, and on macOS the gate is TCC — which grants to a *signed bundle*, so the
process reading the radio has to be the one that is bundled. `internal/capture` is
therefore linked into the core, and **trellisd itself ships as the signed
`Trellis.app`**. The `capture.Scanner` interface still hides host-NIC from external
hardware; only the process boundary is gone.

```
┌────────────────────────────────────────────────────────────────────┐
│ UI process            React + TypeScript  (WebGL/WebGPU canvas)      │
│   ▲ control: Connect/gRPC            ▲ bulk: binary grids / tiles     │
│   │ (Wails bindings on desktop / gRPC-web+WS in cloud)               │
├───┼──────────────────────────────────────────────────────────────── │
│ Core service (Go)   domain · SQLite/Parquet · orchestration · API ·  │
│                     internal/capture (per-OS, cgo) · licensing ·     │
│                     reporting trigger                                │
│   │ ctrl(UDS)+shmem                                   │ exec        │
│   ▼                                                    ▼            │
│ RF Engine (C++/GPU)                                  Reporter (Go)   │
│ pure: scene → grids                                  + headless PDF  │
├──────────────────────────────────────────────────────────────────── │
│ Store:  project.sqlite   +   surveys/*.parquet   +   assets/         │
└────────────────────────────────────────────────────────────────────┘
```

## 2. Components

### 2.1 Predictive RF Engine — C++20 + GPU (the only greenfield heavy-math component)
A **pure function**: `compute(scene, params) → predicted grids`. No I/O, no DB, no app
logic ⇒ deterministic ⇒ golden-file testable. Holds a scene handle for *incremental*
recompute. **Measured-survey interpolation is NOT here** — it's reused Go in the core
(see 2.2 + `09-SEED-MIGRATION.md`); the engine only does *prediction*.
- Geometry, tiered propagation (fast MWF/ITU for interactivity; ray-trace for final),
  derived layers (SNR, data-rate, interference, coverage, roaming), and **calibration**
  (fit the predictive model to measured points). Detail in [04-RF-ENGINE.md](04-RF-ENGINE.md).
- **Interface:** flat C ABI; scene + grids in shared memory (Arrow/flatbuffer).
- **GPU:** kernels written once against **wgpu-native / Dawn** → Vulkan/Metal/D3D12.
  **CPU/SIMD fallback** for headless CI and no-GPU hosts.

### 2.2 Core Service — Go (the majority)
Everything that isn't *predictive* math or pixels.
- **Domain + persistence:** buildings/floors/scenes/APs/antennas/materials/
  requirements/surveys. SQLite (relational) via `sqlc`; Parquet (measurement clouds).
  Pure-Go `modernc.org/sqlite` to keep the core cgo-free.
- **Measured survey (migrated from Seed, `09-SEED-MIGRATION.md`):** AirMapper `.amp`
  import, interpolation, heatmap/colorscale, analysis, multi-floor, reports — proven Go,
  reused. Produces **measured** grids in the *same* `GridDescriptor` format as the
  engine's **predicted** grids, so UI/reports treat them identically.
- **Orchestration:** AP move → assemble scene *delta* → request compute (shmem) →
  cache grid → notify UI. **Incremental** (only the changed AP/region).
- **Capture:** `internal/capture` reads the host radio in-process (ADR-0006);
  the core attaches position, persists, and triggers interpolation. External
  hardware, when it lands, stays a separate process behind the same interface.
- **API:** one protobuf/Connect surface (streaming for capture + progress), served
  in-process (desktop) or over the network (cloud).
- **Also:** import/export, report-model assembly, **Ed25519** license verification.

### 2.3 UI — TypeScript + React (presentation only)
No domain logic, no math.
- **Canvas:** WebGL/WebGPU floorplan + heatmap overlay. The engine grid arrives as a
  **binary blob**; UI uploads to a texture and shades client-side (instant recolor/
  threshold, no round-trip). Tiled for huge grids.
- **State:** TanStack Query over the API; Zustand for view state.
- **Transport:** Connect/gRPC control + binary channel (Wails or WebSocket) for grids.

### 2.4 Capture — `internal/capture`, linked into the core (ADR-0006)
Turns radios into `wifi.ScannedNetwork` values behind one `Scanner` interface.
- Backends (build tags): macOS CoreWLAN (**implemented**); Linux `nl80211`,
  Windows Native Wifi. Monitor mode is dead on modern macOS.
- **Tier 1 scanning is unprivileged everywhere** — see
  [10-WIFI-CAPTURE.md](10-WIFI-CAPTURE.md). macOS instead requires a signed,
  entitled, LaunchServices-launched bundle, which is why trellisd ships as
  `Trellis.app`. Tier 2 (monitor mode) needs privilege on every platform and is
  not implemented; it will want its own process when it lands.
- **External hardware is first-class** (supported USB radio / NetAlly appliance over
  USB-IP) — a separate process behind the same `Scanner`, unaffected by ADR-0006.
- **The one place cgo is allowed**, enforced by `scripts/check-cgo-confinement.py`.

### 2.5 Reporter — Go + headless renderer
Go builds a report model → Typst or HTML template → headless Chromium → PDF.
Templates versioned; heatmap images rendered offscreen by the engine.

### 2.6 Licensing — Go, Ed25519, offline
Signed tokens, optional node-lock, feature flags, offline-verifiable with an embedded
public key. (Spec reuse from MSN Ed25519 license work. No MD5/rotor/pinned-key.)

## 3. The seams (schema-first; this is the real engineering)
All three contracts defined in **protobuf**, codegen for Go/TS/C++ → one source of
truth. See [contracts/](contracts/).
- **Go ↔ Engine** — control: length-prefixed protobuf over UDS; bulk: shared-memory
  ring (Arrow/flatbuffer scene + grids). Contract = "give scene/delta, get grids."
- **Go ↔ UI** — Connect/gRPC control + binary grid channel.
- **Go ↔ Capture** — streaming protobuf measurements over local socket; radio-source
  abstraction hides host-NIC vs external HW.

## 4. Two hot loops (design to these budgets)
- **Plan loop (< 100 ms):** AP drag → delta to Go → engine recomputes *only the
  affected region* on GPU → grid via shmem → binary to UI → texture + shader.
  Incremental + GPU + binary end-to-end. (Invalidation scope, not GPU FLOPS, is the
  make-or-break.)
- **Survey loop (streaming, backpressured):** daemon → measurements → Go geotags →
  Parquet → periodic interpolation → measured heatmap → UI.

## 5. Deployment modes (same Go core)
- **Desktop:** Wails binary = Go core + React UI, with capture linked in; bundles
  the engine as a helper process. Per-OS installers + code signing/notarization —
  on macOS the signed bundle is not optional packaging, it is what makes Wi-Fi
  network names readable at all (`deploy/macos/build-app.sh`).
- **Cloud/team (later):** Go core as a server, engine as a GPU service, UI as a web
  planner (no capture); projects sync via object storage.
- **CI/headless:** engine CPU fallback so tests run without a GPU.

## 6. Cross-cutting
- **Testing:** engine = golden-file + property tests (more walls ⇒ less signal); Go =
  unit/integration + protobuf contract tests; UI = Playwright. Cross-language contract
  tests on the seams.
- **Observability:** structured logs (Go), engine timing counters, per-compute spans.
- **Versioning:** protobuf contracts are versioned; project format carries a schema
  version in `manifest.json`.
