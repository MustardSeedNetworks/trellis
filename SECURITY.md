# Security Policy

## Supported versions

Until Trellis reaches 1.0, only the **latest released version** receives
security fixes. Older 0.x versions stay on the repo for reference but are not
patched — upgrade to the current minor.

| Version         | Supported          |
| --------------- | ------------------ |
| Latest (`main`) | :white_check_mark: |
| Older 0.x       | :x:                |
| Future 1.x      | :white_check_mark: |

## Read this first: `trellisd` has no authentication and no TLS

This is the most important thing on the page, so it is not buried in a
hardening section at the bottom.

`trellisd` binds `127.0.0.1:8446` and nothing else. There is no login, no API
token, and no TLS listener. Loopback **is** the security boundary — the only
one there is. From `cmd/trellisd/main.go`:

```go
// defaultAddr binds loopback only. This server currently has no
// authentication or TLS (both are tracked follow-ups before any
// non-local deployment), so it must not be exposed on all interfaces by
// default.
defaultAddr = "127.0.0.1:8446"
```

`TRELLIS_ADDR` can override that bind. **Do not point it at a non-loopback
address.** Doing so publishes an unauthenticated API — including survey data
and floor plans — to everything that can route to the host. An operator who
overrides it owns putting authentication and TLS in front of it.

This is a deliberate state for a pre-1.0 tool, not an oversight, and it is why
Trellis is not deployed as a shared service today.

## Reporting a vulnerability

**Please do not open a public issue for a security vulnerability.**

Use one of these private channels:

1. **GitHub Security Advisories (preferred):**
   <https://github.com/MustardSeedNetworks/trellis/security/advisories/new>.
   Creates a private advisory visible only to maintainers and you, with a
   built-in audit trail and CVE coordination workflow.
2. **Email:** `kris.armstrong@icloud.com` with subject `[TRELLIS SECURITY]`.

Include in your report:

- A description of the vulnerability and the affected component(s).
- Steps to reproduce, ideally with a minimal proof-of-concept.
- The version / commit you tested against.
- The potential impact.
- A suggested fix or mitigation, if you have one.

## What to expect

- **Acknowledgment** within 2 business days.
- **Triage** with a severity assessment within 7 business days.
- **Fix or mitigation** within the target window for the severity tier below.
  Disclosure timing is coordinated with you for high-impact issues.
- **Credit** in the resulting advisory and release notes, if you would like it.

### Severity levels

| Level    | Description                         | Target Resolution |
| -------- | ----------------------------------- | ----------------- |
| Critical | Remote code execution, auth bypass  | 24-48 hours       |
| High     | Data exposure, privilege escalation | 7 days            |
| Medium   | Limited impact vulnerabilities      | 30 days           |
| Low      | Minor issues, hardening             | Next release      |

## Scope

In scope:

- Code in this repository — the Go daemon, the embedded React UI, the CI
  workflows and the release pipeline.
- Built artifacts published as part of a tagged GitHub release, verifiable via
  the `cosign` bundle and CycloneDX SBOM shipped with each one.

Out of scope:

- **Reaching `trellisd` over the network after overriding `TRELLIS_ADDR`.**
  That is documented above as unauthenticated by design; exposing it is an
  operator decision, not a vulnerability in this code.
- Vulnerabilities in third-party dependencies — report those upstream. They are
  tracked here by Renovate and `govulncheck` and patched on the next release.
- Denial of service requiring sustained external traffic.
- Social engineering or physical access attacks.

## Hardening notes for operators

- Leave the listener on loopback. If you need it elsewhere, put an
  authenticating reverse proxy with TLS in front of it and firewall the port.
- Treat survey projects as sensitive: floor plans and site names describe a
  physical building, and the API serves them without authentication.
- Verify release artifacts before installing:
  `cosign verify-blob` against the `<file>.cosign.bundle` shipped with each
  release. Every archive also ships an SBOM.

## Acknowledgments

Security researchers who help keep Trellis secure are credited in the resulting
advisory and release notes, unless they prefer to remain anonymous.
