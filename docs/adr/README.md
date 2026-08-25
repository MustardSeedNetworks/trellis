# Architecture Decision Records

Short, dated records of the load-bearing decisions and *why*. Supersede by adding a new
ADR that references the old one — don't edit history.

| ADR | Decision | Status |
|---|---|---|
| [0001](ADR-0001-process-isolated-architecture.md) | Process-isolated, local-first, transport-agnostic core | Accepted |
| [0002](ADR-0002-language-per-component.md) | Go core · C++/GPU engine · TS/React UI (not all-Rust) | Accepted |
| [0003](ADR-0003-protobuf-seams-shared-memory.md) | Schema-first protobuf seams; shared-memory for grids | Accepted |
| [0004](ADR-0004-gpu-via-wgpu-dawn.md) | Portable GPU via wgpu-native/Dawn + CPU fallback | Accepted |
| [0005](ADR-0005-ed25519-offline-licensing.md) | Ed25519 offline-verifiable licensing | Accepted |
| [0006](ADR-0006-capture-linked-into-core.md) | Wi-Fi capture linked into trellisd, not a separate daemon (amends 0001, 0002) | Accepted |
