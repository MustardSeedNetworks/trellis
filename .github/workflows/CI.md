# CI/CD Pipeline

The CI pipeline runs on every push and PR. **All checks must pass.**

## GitHub Actions Workflows

### ci.yml - Main CI Pipeline

| Job | Description | Checks |
| --- | --- | --- |
| `backend` | Go checks | build, vet, test -race, gofmt |
| `govulncheck` | Go vulnerability scan | `govulncheck` (hard gate) |
| `frontend` | React/TS checks | Biome lint, Vitest, tsc + Vite build |
| `semgrep` | SAST | Fleet-shared Semgrep rules (`MustardSeedNetworks/.github`) |

Trellis has no C dataplane, no Playwright/E2E, no i18n, no Storybook, and no
path-filtered `changes`/`ci-complete` dispatcher — it is a much smaller
surface than seed/stem/niac today. Every job in `ci.yml` is a required
status check directly; there is no aggregator to keep in sync.

### Other Workflows

| Workflow | Purpose |
| --- | --- |
| `codeql.yml` | CodeQL security analysis (Go, JS/TS) |
| `dead-code.yml` | Weekly dead code detection |
| `docs-link-check.yml` | Weekly external link check |
| `label-sync.yml` | Sync label definitions |
| `labeler.yml` | Auto-label PRs and issues |
| `license-check.yml` | Verify dependency licenses |
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

Every workflow that needs Node uses `./.github/actions/setup-node`; none pin
a `node-version:` literal. The composite is the single place the Node and
npm versions are declared, and it must stay in step with `.nvmrc` and the
`engines` / `packageManager` fields in `ui/package.json`.

## CI Must Pass Before Merge

`main` is protected. Push a feature branch, open a PR, and let CI gate it:

```bash
gh pr create --fill
gh pr merge --auto --squash --delete-branch
```

Fix issues locally first:

```bash
go build ./... && go vet ./... && go test -race ./... && gofmt -l .
cd ui && npm run lint && npm run test && npm run build
```

## The Universal Build Contract

`internal/version` + `internal/api/embed_ui.go` implement the same
`/__version` contract as seed/stem/niac: `go build ./...` compiles against
the tracked `internal/api/ui/.gitkeep` stub (so bare CI builds don't need a
UI build first), but a real release always runs `ui/` through Vite first —
Vite writes straight into `internal/api/ui/` (see `ui/vite.config.ts`'s
`outDir`) — so the embedded `uiBuildHash` is non-empty and provable.
`release.yml`'s `build-ui` job does this before `goreleaser` runs.
