# Trellis — System Architecture

Status: draft for review · Owner: Kris Armstrong / MSN

## 1. The keystone decision: process-isolated, local-first, transport-agnostic core

Trellis is **not** a monolith. It is four cooperating OS processes, with the Go core
written so the **same binary** runs embedded in the desktop app *or* as a cloud
server. This one choice buys us:

- **Fault isolation** — a C++/GPU crash or a hung GPU driver can't take down the app.
- **Clean builds** — the Go core stays **cgo-free** (single static binary); C++ builds
  independently; cgo is confined to the capture daemon only.
- **Fast big-buffer path** — heatmap grids move between Go and the engine by
  **shared memory**, never JSON.
- **Privilege isolation** — capture runs as a separate privileged helper.
- **One codebase, desktop *and* cloud** — the core's API is transport-agnostic.

```
┌────────────────────────────────────────────────────────────────────┐
│ UI process            React + TypeScript  (WebGL/WebGPU canvas)      │
│   ▲ control: Connect/gRPC            ▲ bulk: binary grids / tiles     │
│   │ (Wails bindings on desktop / gRPC-web+WS in cloud)               │
├───┼──────────────────────────────────────────────────────────────── │
│ Core service (Go)   domain · SQLite/Parquet · orchestration · API ·  │
│                     capture coord · licensing · reporting trigger    │
│   │ ctrl(UDS)+shmem        │ local socket(stream)      │ exec        │
│   ▼                        ▼                            ▼            │
│ RF Engine (C++/GPU)     Capture daemon (Go,per-OS)   Reporter (Go)   │
│ pure: scene → grids     radios → measurement stream  + headless PDF  │
├──────────────────────────────────────────────────────────────────── │
│ Store:  project.sqlite   +   surveys/*.parquet   +   assets/         │
└────────────────────────────────────────────────────────────────────┘
```

## 2. Components

### 2.1 RF Engine — C++20 + GPU (the only heavy-math component)
A **pure function**: `compute(scene, params) → grids`. No I/O, no DB, no app logic ⇒
deterministic ⇒ golden-file testable. Holds a scene handle for *incremental* recompute.
- Geometry, tiered propagation (fast MWF/ITU for interactivity; ray-trace for final),
  derived layers (SNR, data-rate, interference, coverage, roaming), survey
  interpolation + calibration. Detail in [04-RF-ENGINE.md](04-RF-ENGINE.md).
- **Interface:** flat C ABI; scene + grids in shared memory (Arrow/flatbuffer).
- **GPU:** kernels written once against **wgpu-native / Dawn** → Vulkan/Metal/D3D12.
  **CPU/SIMD fallback** for headless CI and no-GPU hosts.

### 2.2 Core Service — Go (the majority)
Everything that isn't math or pixels.
- **Domain + persistence:** buildings/floors/scenes/APs/antennas/materials/
  requirements/surveys. SQLite (relational) via `sqlc`; Parquet (measurement clouds).
  Pure-Go `modernc.org/sqlite` to keep the core cgo-free.
- **Orchestration:** AP move → assemble scene *delta* → request compute (shmem) →
  cache grid → notify UI. **Incremental** (only the changed AP/region).
- **Capture coordination:** consume measurement stream, attach position, persist,
  trigger interpolation.
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

### 2.4 Capture Daemon — Go + per-OS + external HW (privileged helper)
Turns radios into a uniform measurement stream.
- Backends (build tags): Linux `nl80211`+`AF_PACKET`; Windows Npcap + Native-WiFi;
  macOS CoreWLAN scan (**monitor mode is dead on modern macOS**).
- **External hardware is first-class** (supported USB radio / NetAlly appliance over
  USB-IP) — pragmatic primary path; host-NIC is best-effort.
- **The one place cgo is allowed** (libpcap/Npcap), confined here.

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
- **Desktop:** Wails binary = Go core + React UI; bundles the engine + capture daemon
  as helper processes. Per-OS installers + code signing/notarization.
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
