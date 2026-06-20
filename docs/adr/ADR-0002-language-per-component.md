# ADR-0002 — Language per component (Go core · C++/GPU engine · TS/React UI)

Status: Accepted · Date: 2026-06-20

## Decision
- **Go** for the core service, capture daemon, reporter, licensing — the majority.
- **C++20 (+GPU)** for the RF engine only, behind a flat C ABI.
- **TypeScript + React** for the UI (WebGL/WebGPU), in Wails (desktop) and web (cloud).

## Why
- The perf-critical math is ~15% of the code but ~90% of the CPU, and it belongs on the
  **GPU** regardless of host language — so the host should optimize for **velocity**.
- **Go** is the productive spine: concurrency for the capture/stream pipeline, clean
  SQL, trivial deploys, one binary for desktop+cloud.
- **C++** owns the math/GPU: mature numerics (Eigen), geometry, SIMD, GPU interop, and
  a large body of portable RF code to draw on.
- **TS/React** is the strongest UI ecosystem and gives desktop+web from one codebase.

## Why not all-Rust (explicitly considered)
Rust's safety/perf wins land only in the engine, while its friction is paid across the
85% that's UI/orchestration/glue — the wrong trade for an iterate-heavy product. Its
genuine fits here (optional native capture, WASM engine for a browser planner) are
narrow and don't justify adopting it fleet-wide.

## Consequences
- Three toolchains in CI; managed with `buf` (contracts) + per-component pipelines.
- A strict rule: **C++ never does I/O/DB/orchestration**; **Go core stays cgo-free**
  (engine is a separate process). See ADR-0003.
