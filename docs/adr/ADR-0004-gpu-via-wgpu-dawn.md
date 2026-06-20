# ADR-0004 — Portable GPU via wgpu-native/Dawn + CPU fallback

Status: Accepted · Date: 2026-06-20

## Context
Heatmap propagation, ray-tracing, and interpolation are massively parallel and belong
on the GPU. But Trellis targets Windows, macOS, and Linux — three GPU APIs (D3D12,
Metal, Vulkan).

## Decision
Write GPU compute **once** against a portable layer — **wgpu-native** or **Google
Dawn** (WebGPU/WGSL → D3D12/Metal/Vulkan) — and maintain a **CPU/SIMD fallback** of the
same kernels (ISPC or intrinsics).

## Why
- One kernel codebase across all OSes; no per-API hand-rolling (R1).
- WGSL/WebGPU is the same model the UI uses, so knowledge transfers and a future
  browser planner can share kernels.
- The CPU fallback means **nothing requires a GPU**: CI runs headless, low-end hosts
  still work, and golden tests run deterministically on the CPU path.

## Consequences
- Tie to the maturity of wgpu-native/Dawn; acceptable and improving.
- Must keep CPU and GPU kernels in agreement (shared test vectors).

## Alternatives rejected
- **Per-API native** (D3D/Metal/Vulkan by hand): 3× the GPU code, 3× the bugs.
- **CUDA**: NVIDIA-only — unacceptable for a cross-platform desktop tool.
