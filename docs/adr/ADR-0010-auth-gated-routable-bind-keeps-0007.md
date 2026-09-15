# ADR-0010 — Auth, TLS and CSRF gate a routable bind; ADR-0007's packaging shape stands

Status: Proposed · Date: 2026-09-14 · Becomes Accepted when
[#440](https://github.com/MustardSeedNetworks/trellis/pull/440) merges · Amends
[ADR-0007](ADR-0007-linux-packaging-and-service-contract.md); answers the auth
half of [#160](https://github.com/MustardSeedNetworks/trellis/issues/160) and
[#439](https://github.com/MustardSeedNetworks/trellis/issues/439)

## Context

ADR-0007 closed with a trigger: "if #160's tablet workflow is ever built —
auth, TLS, CSRF, then a routable bind — that feature brings a _system_ unit
with it, and supersedes this ADR … replaced wholesale rather than loosened
field by field." PR #440 is titled with that phrase. It adds an operator
credential (Argon2id), a signed session cookie, CSRF through
`foundation/pkg/csrf`, a bounded login rate limiter, TLS, and a bind gate that
allows a non-loopback address only once a credential is configured. It ships
no system unit, no service user, no `/etc` config, and changes no package.

Read literally, ADR-0007 says #440 supersedes it. Read for its reason, ADR-0007
was guarding against a machine-wide service owned by a service user whose data
the operator cannot reach. #440 builds none of that: the daemon still runs as
the surveyor, in their session, with their data in their home; the only change
is that the surveyor may reach it from a tablet on the same network after
logging in. Two readings of one sentence were in the repo (the ADR and the PR
body). The owner decided on 2026-09-14: **#440 is narrower than the workflow
ADR-0007 anticipated, and 0007's packaging shape stands.**

## Decision

1. **A routable bind is gated on credential + TLS + CSRF.** The default stays
   `127.0.0.1:8446`, plain HTTP, no login. `TRELLIS_ADDR` on a non-loopback
   address is refused until an operator credential exists and TLS is on;
   then the daemon serves HTTPS with the session cookie and CSRF token the
   fleet's other three products use (`foundation/pkg/csrf`).
2. **ADR-0007 is amended, not superseded.** The `.deb`/`.rpm` still install
   the binary and a not-enabled _user_ unit; no service user, no `/etc`
   conffile, no `setcap`, no port opened by a package. A system unit remains
   a separate, future decision, taken only if a deployment appears where more
   than one person shares one Trellis on one host.
3. **The fleet's port table reads conditionally**: "plain HTTP on loopback
   only until an operator credential is configured", replacing the flat "do
   not fix it to HTTPS".

## Consequences

- Two run modes, one binary: local (today's) and credentialed-routable. The
  bind gate is the invariant that keeps them honest, as it was in ADR-0007.
- The shared auth core follow-up (foundation#29) is where the login, session
  and rate-limit pieces converge with seed and stem; until it lands, the
  Trellis copy in `internal/auth` is the implementation.
- The loopback default means an install still has no network exposure until
  a person asks for one, so ADR-0007's `apt purge` and no-firewall-rule
  consequences hold unchanged.
- If a later feature does need a system unit, that ADR supersedes 0007 and
  this one together; this ADR records that #440 alone did not.
