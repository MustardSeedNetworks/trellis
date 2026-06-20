# Trellis — Tech Stack & Rationale

Language per component is deliberate: **Go is the spine, C/C++ is a tight math/GPU core
it calls, React/TS is the face.** Each is used where it's strongest and nowhere else.

| Component | Language | Key libs / tools | Why this language |
|---|---|---|---|
| **RF engine** | **C++20** (+ C ABI) | Eigen (linalg), CGAL/`parry`-style geometry, **wgpu-native or Dawn** (portable GPU), ISPC/SIMD (CPU fallback) | Mature numerics + geometry, SIMD, GPU interop, decades of portable RF code to draw on. Fenced behind a flat C ABI. |
| **Heatmap compute** | C++ + **WGSL/SPIR-V** compute shaders | wgpu-native/Dawn | Grid propagation, ray-trace, interpolation are massively parallel → GPU. |
| **Core service** | **Go 1.2x** | `connectrpc` (API), `sqlc` + `modernc.org/sqlite` (pure-Go, cgo-free), Apache Arrow Go, `protobuf` | Productive, great concurrency for the capture/stream pipeline, simple deploys, one binary serves desktop *and* cloud. |
| **Desktop shell** | Go + **Wails v2/v3** | webview2/WKWebView/webkitgtk | The Go-native Tauri analog: Go host + web UI in one binary, no Electron bloat. |
| **UI** | **TypeScript + React** | WebGL/WebGPU (custom or regl/deck.gl), TanStack Query, Zustand, Vite | Best UI ecosystem; one codebase serves desktop (Wails) and web (cloud planner). |
| **Capture daemon** | **Go** (+ contained cgo) | `nl80211`/`AF_PACKET` (Linux), Npcap + Native-WiFi (Windows), CoreWLAN (macOS), libpcap | Concurrency + per-OS build tags; cgo is acceptable *here* because it's isolated. |
| **Reporter** | Go | Typst **or** HTML + headless Chromium | Versionable templates; kills Crystal Reports. |
| **Contracts** | **protobuf** | `buf` (lint/breaking/codegen) → Go, TS (`connect-es`), C++ | Single source of truth for all three seams. |
| **Licensing** | Go | Ed25519 (`crypto/ed25519`) | Offline-verifiable signed tokens; reuse MSN spec. |
| **Project store** | — | **SQLite** (relational) + **Parquet/Arrow** (measurement clouds) | Right tool per data shape; millions of points belong in columnar, not SQLite. |

## Hard rules (the seams that keep this clean)
1. **The Go core is cgo-free.** The engine is a *separate process* (shmem + UDS), not
   linked in. cgo lives only in the capture daemon (and is avoided even there where a
   pure-Go path exists, e.g. `modernc.org/sqlite`).
2. **C++ stays behind a flat C ABI** and never does I/O, DB, or orchestration —
   "give scene/delta, get grids." Resist C++ creeping upward.
3. **GPU is written once** against wgpu-native/Dawn — no per-API hand-rolling — with a
   CPU/SIMD fallback so CI runs headless.
4. **Big buffers never serialize to JSON.** Grids move as binary (shmem Go↔engine,
   binary channel Go↔UI). JSON is for control messages only.
5. **Schema-first.** Every cross-language boundary is a `.proto` checked by `buf`;
   generated code, never hand-written wire types.

## Why not all-Rust (recorded, since it came up)
Rust's safety/perf wins land in ~15% of this app (the engine), but its friction is
paid across the 85% that's UI/orchestration/glue — wrong trade for an iterate-heavy
product. The perf-critical math goes to the GPU regardless of host language, so the
host should optimize for velocity. Rust's only real fits here (optional native capture,
WASM engine for a browser planner) are narrow and not worth adopting fleet-wide.
