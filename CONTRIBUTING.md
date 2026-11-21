# Contributing to Trellis

Thank you for your interest in contributing to Trellis. This document covers how
to set up a working tree, what the checks expect, and how a change reaches
`main`.

## Code of Conduct

Participation is governed by the [Code of Conduct](CODE_OF_CONDUCT.md).

## Getting Started

### Prerequisites

Versions are pinned by the repository; use these, not newer or older ones.

- Go 1.27.4 (`go.mod`)
- Node.js 26.13.0 and npm 12.2.1 (`.nvmrc`, `engines` in `ui/package.json`)
- Git
- GNU Make

`golangci-lint` (v2.15.0) and `buf` are installed at their pinned versions by
`make lint`; do not rely on a copy on your `PATH`. A copy of any other version
passes what CI rejects or rejects what CI passes.

### Development Setup

```bash
git clone https://github.com/MustardSeedNetworks/trellis.git
cd trellis

go mod download
make build        # UI bundle, then bin/trellisd with build metadata embedded
make test
make lint
```

`make build` builds the UI first because the daemon embeds it. Use `make build`,
not a bare `go build`, for anything you intend to run or validate: a binary
built outside make reports `uiBuildHash` as `unknown`.

## Make Targets

| Target | What it does |
| --- | --- |
| `make build` | Builds the UI and `bin/trellisd` with version, commit, build time and UI hash |
| `make test` | `go test ./...` |
| `make test-e2e` | The Playwright suite in `ui/` |
| `make lint` | golangci-lint on Linux, Windows and e2e build tags, plus `buf lint` |
| `make lint-md` | markdownlint over every `.md` file |
| `make fmt` / `make fmt-check` | Apply / verify `gofmt` |
| `make security` | `govulncheck`, the same invocation CI runs |
| `make generate generate-ts` | Regenerate `gen/` and `ui/src/gen/` from `proto/` |
| `make check-generated` | Fail if generated code differs from the proto |
| `make validate-touched` | Lint and test only what your branch changed, plus its reverse dependencies |
| `make packages` | Local `.deb` / `.rpm` snapshot through goreleaser |
| `make deploy-validate EXPECT=vX.Y.Z` | Assert a running daemon's `/__version` matches a release |

Between edits, `make validate-touched` is the fast loop. Run the full
`make test` and `make lint` once before opening the PR.

## Development Workflow

### Branches

`main` is protected. Work on a branch and open a pull request; never push to
`main`. Use a conventional prefix: `feat/`, `fix/`, `docs/`, `chore/`,
`refactor/`, `test/`, `ci/`.

### Commit Messages

Commits and PR titles follow [Conventional Commits](https://www.conventionalcommits.org/).
The subject starts lowercase and is at least three characters.

```text
feat(capture): add passive scan to the windows backend
fix: reject non-read methods on /__version
docs: describe the log file location
```

Allowed types: `build`, `chore`, `ci`, `docs`, `feat`, `fix`, `perf`,
`refactor`, `revert`, `test`. release-please builds the changelog and version
from these, so the type matters.

### Pull Request Process

1. **Open an issue first.** Issue titles start with a category (`Defect:`,
   `Feature:`, `Chore:`, `Docs:`, `Security:`, `Epic:`) or a conventional
   prefix.
2. **Branch from `main`.**
3. **Write tests.** Table-driven Go tests; Vitest for UI logic; Playwright for
   user flows.
4. **Run the checks** listed above.
5. **Fill in the PR template.** It needs `Fixes #N` (with the `#`) linking the
   issue, and your test commands and output in the Testing Evidence block.
6. **Wait for CI.** `CI Complete` gates the merge.

## Code Standards

### Go

- Follow [Effective Go](https://go.dev/doc/effective_go).
- `gofmt` and `golangci-lint` clean, with zero warnings. Fix the cause of a
  finding; do not add a `//nolint` directive.
- Table-driven tests that assert behavior.
- HTTP responses use the JSON encoder or `html/template`, never a raw format
  string. `scripts/check-output-escaping.sh` enforces this.
- cgo stays inside `internal/capture` (`scripts/check-cgo-confinement.py`).

### TypeScript and React

- TypeScript only, never JavaScript; no `any`.
- Biome for linting and formatting.
- Vitest for unit tests.

## Reporting Security Vulnerabilities

**Do not open a public issue for a security vulnerability.** See
[SECURITY.md](SECURITY.md) for the private disclosure channels.

## Questions

Check the existing issues and the documents under [docs/](docs/), then open an
issue for a question, a defect or a feature.
