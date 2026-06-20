# ADR-0005 — Ed25519 offline-verifiable licensing

Status: Accepted · Date: 2026-06-20

## Context
We just reverse-engineered AirMagnet's licensing: MD5-digest PKCS#7 over INI, an RSA
key pinned in a DLL, and a fragile MAC host-lock. It was forgeable-adjacent, opaque,
and a maintenance nightmare. Trellis must not repeat that.

## Decision
Use **Ed25519-signed, offline-verifiable license tokens**, reusing the MSN Ed25519
license spec. The Go core embeds the public key and verifies signatures locally;
optional node-lock and feature flags; optional (not required) online activation.

## Why
- Modern, small, fast, hard to forge; no phone-home requirement (works air-gapped).
- Aligns with the rest of the MSN fleet (Seed/Stem/NIAC license direction).
- Avoids the AirMagnet failure modes: no MD5, no rotor cipher, no DLL-pinned RSA, no
  brittle host-lock semantics.

## Consequences
- Need a keygen/issuer workflow (private key custody, signing) — kept off any product
  binary; lessons from the AirMagnet recovery toolkit apply.
- Feature gating wires through the Go core (single enforcement point), not scattered.

## Notes
- Decide host-lock policy deliberately (none / soft / hard) — AirMagnet's per-build
  divergence was a self-inflicted wound; pick one model and document it.
