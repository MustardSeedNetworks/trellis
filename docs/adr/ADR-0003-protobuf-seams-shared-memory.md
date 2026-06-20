# ADR-0003 — Schema-first protobuf seams; shared memory for grids

Status: Accepted · Date: 2026-06-20

## Context
Process isolation (ADR-0001) means three cross-language seams: UI↔Go, Go↔Engine,
Go↔Capture. These are the integration surface and the most likely source of pain.

## Decision
- Define **every seam in protobuf**, in `/contracts`, as the single source of truth;
  generate Go / TS / C++ with **`buf`**. No hand-written wire types.
- **Control messages** travel as protobuf (gRPC/Connect, or length-prefixed over UDS).
- **Bulk grid/scene buffers** travel by **shared memory**, referenced from protobuf by
  handle + descriptor. **Grids never serialize to JSON.**
- Freeze the contracts in **Phase 0**, before component code.

## Why
- Schema-first catches breaking changes (`buf breaking`) and keeps three languages in
  sync mechanically.
- A 2M-cell float grid as JSON would dominate the latency budget; shared memory makes
  the Go↔engine round-trip cheap, and a binary channel does the same Go↔UI.
- Freezing contracts first lets the engine, core, and UI proceed in parallel (UI can
  mock the API from the `.proto`).

## Consequences
- Up-front contract design effort (the keystone `engine.proto` exists in draft).
- Shared-memory lifecycle (allocation, handles, cleanup) must be designed carefully;
  use Arrow/flatbuffer layouts for self-describing buffers.
