# CI/CD Pipeline

The CI pipeline runs on every push and PR. **All checks must pass.**

## GitHub Actions Workflows

### ci.yml - Main CI Pipeline

| Job | Description | Checks |
| --- | --- | --- |
| `backend` | Go checks | build, vet, test -race, gofmt |
| `backend-darwin` | Go checks (macOS) | Builds and tests on `macos-latest`; only compiler for `*_darwin.go` |
| `backend-windows` | Go checks (Windows) | Builds and tests on `windows-latest` |
| `govulncheck` | Go vulnerability scan | `govulncheck` (hard gate) |
| `frontend` | React/TS checks | Biome lint, Vitest, tsc + Vite build |
| `e2e` | Browser tests | Playwright, chromium + webkit |
| `i18n` | Internationalization | Fleet-shared i18n validator |
| `quality` | Code quality gates | banned vocabulary, file-size ratchet, theme contract |
| `security` | Security scans | npm audit, gitleaks, Trivy |
| `semgrep` | SAST | Fleet-shared Semgrep rules (`MustardSeedNetworks/.github`) |
| `ci-conformance` | Fleet CI conformance | Reusable, from `MustardSeedNetworks/.github` |
| `codeql-alert-gate` | CodeQL alert gate | Fails on open High/Critical alerts |
| `ci-complete` | Aggregate gate | The required status check |

Trellis has no C dataplane and no Storybook, so it carries no `c-lint` or
Storybook job. It does have E2E, i18n and a `ci-complete` aggregate — this
section previously said it had none of those, which stopped being true as the
jobs were added and nobody updated the page.

### Other Workflows

| Workflow | Purpose |
| --- | --- |
| `codeql.yml` | CodeQL security analysis (Go, JS/TS) |
| `dead-code.yml` | Weekly dead code detection |
| `docs-link-check.yml` | Weekly external link check |
| `label-sync.yml` | Sync label definitions |
| `labeler.yml` | Auto-label PRs and issues |
| `license-check.yml` | Verify dependency licenses |
| `main-retry.yml`      | Retry a failed main run once (see the file header)  |
| `pr-body-lint.yml` | Enforce the PR body template |
| `release-please.yml` | Automated version management and release PRs |
| `release.yml` | goreleaser release builds, signing, provenance |
| `scorecard.yml` | OpenSSF Scorecard |
| `title-lint.yml` | Lint PR and issue titles |
| `todo-tracker.yml` | Weekly TODO tracking |

## Workflow security

Two scanners cover `.github/workflows/` itself; run them locally before
pushing workflow changes:

```bash
actionlint -ignore 'SC2129' .github/workflows/*.yml
zizmor --min-severity high .github/workflows/
```

- **actionlint** — syntax, expression and shell errors inside `run:` blocks.
  `SC2129` is ignored as a pure style preference; every correctness rule
  stays on.
- **zizmor** (pinned 1.29.0) — Actions security scanner. **Blocks on High
  findings.** `release-please.yml` carries one `# zizmor: ignore[...]`
  comment (workflow_run trigger, justified inline) mirroring seed/stem/niac.

Permissions follow least privilege: workflows declare `permissions: {}` (or
`contents: read`) at the top level and grant scopes per job. `release.yml`
deliberately runs without npm caching, because its output is published and
attested and a restored cache entry could land inside a signed artifact; it
opts out by passing `cache: ""` to the `setup-node` composite action.

## The Node.js pin lives in one file

`.nvmrc` is the single source of truth for the Node version. Every workflow that
needs Node uses `./.github/actions/setup-node`, and that composite reads
`.nvmrc` via `node-version-file` — it has **no `node-version` input**, so no
caller can override it and no second copy of the version can exist.

That input used to default to a literal, and it drifted: Renovate bumps the
manifests it can see and cannot see a default buried inside a composite, so CI
ran 26.7.0 against manifests demanding 26.8.1 and logged EBADENGINE on every
job for weeks. "Must stay in step" was the previous rule here, and a rule that
depends on someone remembering is not a mechanism.

The remaining pair that can disagree is `.nvmrc` and the `engines` field in
`package.json`. Making that a hard failure (`engine-strict=true`) is the obvious
next step and is deliberately **not** taken yet: Homebrew's newest `node` is
26.7.0, so 26.8.1 is not installable through the fleet's normal channel, and
turning the mismatch fatal would block local development in all four repos. See
the linked issue.

The npm version is still declared in the composite; `packageManager` in
`package.json` is what `engines` checks it against.

## CI Must Pass Before Merge

`main` is protected. Push a feature branch, open a PR, and let CI gate it:

```bash
gh pr create --fill
gh pr merge --auto
```

`main` uses a **merge queue**, which rejects `--squash` and `--delete-branch`
on `gh pr merge`: the queue owns the merge method. A queued PR reports
`BLOCKED` with an entry under `mergeQueue`, not `CLEAN`.

Fix issues locally first:

```bash
go build ./... && go vet ./... && go test -race ./... && gofmt -l .
cd ui && npm run lint && npm run test && npm run build && npm run test:e2e
```

## The Universal Build Contract

`internal/version` + `internal/api/embed_ui.go` implement the same
`/__version` contract as seed/stem/niac: `go build ./...` compiles against
the tracked `internal/api/ui/.gitkeep` stub (so bare CI builds don't need a
UI build first), but a real release always runs `ui/` through Vite first —
Vite writes straight into `internal/api/ui/` (see `ui/vite.config.ts`'s
`outDir`) — so the embedded `uiBuildHash` is non-empty and provable.
`release.yml`'s `build-ui` job does this before `goreleaser` runs.
