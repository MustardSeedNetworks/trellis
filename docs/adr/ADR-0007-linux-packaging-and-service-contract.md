# ADR-0007 — Linux packages install a user-session daemon, not a system service

Status: Accepted · Date: 2026-09-05 · Answers [#39](https://github.com/MustardSeedNetworks/trellis/issues/39)
and the packaging half of [#160](https://github.com/MustardSeedNetworks/trellis/issues/160)

## Context

Trellis published archives only: three `.tar.gz` and two `.zip`. seed, stem and
niac all publish `.deb` and `.rpm`, so Trellis was the one product in the fleet
that could not be installed with apt or dnf. #39 asked for parity, and then
answered itself with "decide the service contract first", because packaging a
daemon means shipping a unit file, and a unit file is a statement about who runs
the process, as whom, listening where.

Everything that makes that decision easy is already written down:

- `trellisd` binds `127.0.0.1:8446` and, since #270, **refuses to start on a
  non-loopback address**. There is no authentication, no TLS, no CSRF; the bind
  gate is what keeps that honest rather than a comment.
- Its data lives in the *user's* directory — `$XDG_DATA_HOME/trellis`,
  `~/Library/Application Support/Trellis`, `%AppData%\Trellis`
  (`internal/apppaths`). There is no `/etc/trellis`, no `/var/lib/trellis`, and
  nothing on the machine that a second user would share.
- ADR-0006 linked capture into the daemon and recorded why: on macOS the gate is
  TCC and only a user-launched signed bundle passes it. Trellis is a tool a
  surveyor runs while walking a floor, not a service a machine offers.

The fleet's shape does not transfer. seed, stem and niac are system services:
`niac.service` under `/usr/lib/systemd/system`, a `niac` service user, a
`/etc/niac` config marked as a conffile, directories owned by that user, and
postinstall scripts that enable and start the unit. Copying that shape here
would create a machine-wide service, owned by a service user, whose data
directory the actual operator cannot reach, and would then need
`systemctl disable` to stop it doing nothing useful in the background — for a
daemon that by design serves exactly one person on exactly one host.

## Decision

**The `.deb` and `.rpm` install the binary and a *user* unit. They install no
system service, no service user, no `/etc` config, and open no port.**

Concretely:

1. `/usr/bin/trellisd` — the binary, plus the licence and docs under
   `/usr/share/doc/trellis/`.
2. `/usr/lib/systemd/user/trellisd.service` — a **systemd user unit**, shipped
   **not enabled**. The operator opts in with
   `systemctl --user enable --now trellisd`; it then runs as them, with their
   `XDG_DATA_HOME`, and dies with their session. Nothing starts at install time,
   so an install has no effect until a person asks for one.
3. **No service user and no state directory.** Surveys are the operator's data
   in the operator's home, which is also what makes `apt purge` uncontroversial:
   the packages own nothing outside `/usr`, so remove and purge are identical
   and no survey is ever deleted by a package operation.
4. **No conffile**, because there is no config file. `trellisd` is configured by
   environment (`TRELLIS_ADDR`, `TRELLIS_DATA_DIR`), which the user unit exposes
   through the conventional `~/.config/environment.d/` drop-ins rather than a
   root-owned file in `/etc`. An upgrade therefore cannot clobber local edits.
5. **No firewall rule and no port to open**, following the 2026-05-29 fleet
   change that dropped the HTTP redirectors. The bind gate already refuses any
   address that a firewall rule would be needed for.
6. **The package does not `setcap`.** On Linux, *triggering* a scan needs
   `CAP_NET_ADMIN` (ADR-0006 measured it: unprivileged trigger is EPERM, reading
   the cache is not). `setcap cap_net_admin+ep /usr/bin/trellisd` in a
   postinstall would hand every local user the ability to reconfigure the host's
   network interfaces, silently, as a side effect of installing a survey tool.
   It stays an explicit operator step, documented in `docs/10-WIFI-CAPTURE.md`,
   and a Trellis without it still imports, analyses and reports — it reads the
   scan cache and cannot trigger a sweep.

## Consequences

- Trellis reaches artifact parity with the fleet — deb and rpm, arch-mapped
  file names, SBOMs on the packages and not only the archives — without
  acquiring a network service it has no auth story for.
- `dnf install` then `trellisd` works for a single user immediately; the user
  unit is for people who want it back after a reboot.
- If #160's tablet workflow is ever built — auth, TLS, CSRF, then a routable
  bind — that feature brings a *system* unit with it, and supersedes this ADR.
  The shape above is deliberately the one that has to be replaced wholesale
  rather than loosened field by field.
- The macOS `.app` bundle (`deploy/macos/build-app.sh`) and the Windows archive
  are unchanged: both are already user-session shapes, and this ADR brings Linux
  into line with them rather than the other way round.
