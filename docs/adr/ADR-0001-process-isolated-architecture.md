# ADR-0001 — Process-isolated, local-first, transport-agnostic core

Status: Accepted · Date: 2026-06-20

## Context
Trellis spans heavy native math (RF/GPU), a lot of orchestration/data plumbing, a rich
UI, and privileged capture. The dead predecessor (AirMagnet) welded all of it into one
Windows-only C++/.NET monolith — fragile, unportable, untestable.

## Decision
Build as **four cooperating OS processes** — UI, Go core, C++/GPU engine, capture
daemon — and write the **Go core to be transport-agnostic** so the *same binary* runs
embedded in the desktop app or as a cloud server.

## Why
- **Fault isolation**: a GPU driver hang or C++ crash can't take down the app or lose data.
- **Build hygiene**: the Go core stays cgo-free (single static binary); native code
  builds independently.
- **Performance**: big grid buffers move by **shared memory**, not JSON.
- **Privilege isolation**: capture runs separately with the rights it needs.
- **One codebase, desktop + cloud**.

## Consequences
- Need well-defined IPC seams (see ADR-0003) — more upfront contract work.
- Slightly more processes to supervise/package; worth it for isolation + reuse.
- Enables a headless engine + CLI (scriptability) as a side benefit.

## Alternatives rejected
- **Monolith** (one process, FFI everywhere): fastest to start, but no fault isolation,
  cgo everywhere, and no desktop/cloud reuse — the AirMagnet trap.
