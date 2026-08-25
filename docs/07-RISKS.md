# Trellis — Risks & Mitigations

| # | Risk | Impact | Mitigation |
|---|---|---|---|
| R1 | **GPU portability** (Vulkan/Metal/D3D12 divergence) | High | Commit to **wgpu-native/Dawn** early; write kernels once; CPU/SIMD fallback so nothing *requires* a GPU. |
| R2 | **macOS monitor-mode capture is effectively dead** | High | Bake in **external-HW-first** capture; treat host-NIC as best-effort; don't discover this late. |
| R3 | **Incremental invalidation** harder than the math | High | This (not GPU FLOPS) is the interactivity make-or-break. Design scene-cache + region/AP-scoped recompute from Phase 1; budget-test in Phase 4. |
| R4 | **RF model accuracy** (predictions don't match reality) | High | Tiered models + **calibration against measurements**; golden cases from published data; validate measured-vs-predicted early. |
| R5 | **cgo creep** erodes the clean Go core | Med | Engine is a *separate process*, not linked. cgo confined to `internal/capture` behind `darwin && cgo` (ADR-0006 — capture is linked into trellisd, so this is a package boundary, not a process one); prefer pure-Go (`modernc.org/sqlite`). Enforced by `scripts/check-cgo-confinement.py` in CI, plus `CGO_ENABLED=0 go build ./...`. |
| R6 | **Big-buffer data path** becomes the bottleneck | Med | Binary everywhere for grids (shmem Go↔engine, binary channel Go↔UI); tile large grids; never JSON a grid. |
| R7 | **Scope creep** (auto-design, spectrum, cloud pulled into MVP) | Med | PRD tags [MVP]/[Later]; phase gates; non-goals are explicit. |
| R8 | **CAD import** (DXF/DWG) is a tar pit | Med | Image/PDF floorplans for MVP; CAD later; consider ODA/`dxf` libs + ML wall-extraction as a separate workstream. |
| R9 | **Strategy contradiction** with locked MSN docs | Med | Recorded in `00-VISION.md`; update `LICENSE_STRATEGY.md` to sanction Trellis (owner decision) so docs stop conflicting. |
| R10 | **Code signing / notarization** across 3 OSes | Med | Reuse MSN build-platform work (Apple Dev acct available, Win cert); plan in Phase 6, not at the end. |
| R11 | **Three-language build/CI** complexity | Med | `buf` for contracts; per-component CI; protobuf contract tests across seams; reproducible toolchains pinned (MSN always-latest policy). |
