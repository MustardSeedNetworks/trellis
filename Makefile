.PHONY: generate generate-ts ui build build-e2e ui-build-hash lint lint-md golangci-lint buf vet test fmt-check check-stale-tests packages

include mk/lint.mk

# Universal Build Contract: every binary carries version, commit, build time
# and the md5 of the embedded UI, injected into internal/version. The hash is
# the one value with no VCS fallback, so a binary built without this recipe
# reports uiBuildHash "unknown" and deployment validation catches it. The same
# script computes it for release.yml, the macOS bundle and the E2E daemon, so
# there is one recipe rather than four that can drift.
VERSION_PKG := github.com/MustardSeedNetworks/trellis/internal/version
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME  ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
UI_BUILD_HASH = $(shell ./scripts/ui-build-hash.sh)
LDFLAGS = -s -w \
	-X $(VERSION_PKG).Version=$(VERSION) \
	-X $(VERSION_PKG).Commit=$(COMMIT) \
	-X $(VERSION_PKG).BuildTime=$(BUILD_TIME) \
	-X $(VERSION_PKG).UIBuildHash=$(UI_BUILD_HASH)
GOFLAGS = -trimpath -buildvcs=false -ldflags "$(LDFLAGS)"

# Vite's entry output, and the file goreleaser's before hook already treats as
# proof the UI was built. Used here as the make target standing for the bundle.
UI_BUNDLE := internal/api/ui/index.html
# What a rebuild must react to. Deliberately not all of ui/: e2e specs, stories
# and the Playwright config are not inputs to the bundle, and listing them would
# rebuild it for changes that cannot affect it.
UI_SOURCES := $(shell find ui/src ui/public -type f 2>/dev/null) \
	ui/index.html ui/package.json ui/vite.config.ts \
	ui/tsconfig.json ui/tsconfig.app.json ui/postcss.config.js

# Must match the golangci-lint pin in .github/workflows/ci.yml. A copy on PATH
# of any other version is a false clear: it passes what CI rejects or rejects
# what CI passes.
GOLANGCI_LINT_VERSION := v2.13.2
GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint

# Must match the buf pin in .github/workflows/ci.yml. Bare `buf` is the same
# false clear as a stale golangci-lint: this machine had 1.71.0 while CI pinned
# 1.72.0, so `make lint` and CI were linting the protos with different rule sets.
#
# The version is derived FROM the install line, not the other way round. The org
# Renovate preset has dedicated managers for GOLANGCI_LINT_VERSION and
# MARKDOWNLINT_CLI2_VERSION and none for buf, so only the literal
# `go install <module>@<version>` form is tracked; `@$(BUF_VERSION)` would be an
# invisible pin, which the preset's own description calls worse than a floating
# one. One literal means a Renovate bump cannot leave a stale copy behind.
BUF_INSTALL := go install github.com/bufbuild/buf/cmd/buf@v1.72.0
BUF_VERSION := $(lastword $(subst @, ,$(BUF_INSTALL)))
BUF := $(shell go env GOPATH)/bin/buf

generate: buf
	$(BUF) generate

generate-ts: buf
	$(BUF) generate --template buf.gen.ui.yaml

# Compiles the whole tree, then produces the daemon with the contract's ldflags.
# The UI bundle is a prerequisite, not a separate target a caller must remember:
# without it UI_BUILD_HASH expands to "unknown" and the binary proves nothing
# about its UI (Universal Build Contract rules 1 and 3). UI_BUILD_HASH is
# deliberately `=` and not `:=` — it expands when the recipe line below runs,
# which is after $(UI_BUNDLE) has been made, so the hash covers the bundle this
# build actually embeds.
build: $(UI_BUNDLE)
	go build ./...
	go build $(GOFLAGS) -o bin/trellisd ./cmd/trellisd

# The daemon the Playwright suite drives: the same recipe with the radio swapped
# for a scripted scanner (cmd/trellisd/scanner_e2e.go).
# Same UI prerequisite as `build`: Playwright's webServer builds through this
# target and handshake.spec.ts asserts the uiBuildHash in /__version, so on a
# clean tree without it the suite fails on the hash rather than on a defect.
build-e2e: $(UI_BUNDLE)
	go build -tags e2e $(GOFLAGS) -o bin/trellisd-e2e ./cmd/trellisd

ui-build-hash:
	@./scripts/ui-build-hash.sh

# The host platform's build tags decide which files golangci-lint even sees, so
# a macOS run never inspects capture_linux.go or capture_windows.go — three
# gosec findings reached CI that way. GOOS is enough to fix it for the pure-Go
# backends and for the daemon's per-platform bind-error check; the darwin
# backend needs cgo and so only lints on a Mac.
lint: golangci-lint buf
	$(GOLANGCI_LINT) run ./core/... ./internal/... ./cmd/... ./tools/...
	GOOS=linux $(GOLANGCI_LINT) run ./internal/capture/... ./cmd/trellisd/...
	GOOS=windows $(GOLANGCI_LINT) run ./internal/capture/... ./cmd/trellisd/...
	$(GOLANGCI_LINT) run --build-tags e2e ./cmd/trellisd/...
	$(BUF) lint

vet:
	go vet ./...

# check-stale-tests refuses to start while orphaned test binaries from an
# earlier run are still holding the machine. Go's -test.timeout cannot kill a
# binary stuck in a cgo call, so they accumulate silently and make every
# subsequent timing meaningless — see the script for what that cost once.
check-stale-tests:
	@./scripts/check-stale-tests.sh

test: check-stale-tests
	go test ./...

fmt-check:
	@fmt_out="$$(gofmt -l .)"; \
	if [ -n "$$fmt_out" ]; then \
		echo "$$fmt_out"; \
		exit 1; \
	fi

# Vite writes straight into internal/api/ui/, which Go embeds — no copy step
# (Universal Build Contract). `packages` needs it because goreleaser's before
# hook refuses to build a binary with an empty UI.
#
# Split into two file targets rather than one phony recipe, because `build` now
# depends on this: a phony `ui` would re-run `npm ci` on every `make build`,
# which is a slow default for a backend-only change. `npm ci` re-runs only when
# the lockfile moves; Vite re-runs only when a UI source moves.
ui: $(UI_BUNDLE)

ui/node_modules: ui/package-lock.json
	cd ui && npm ci
	@touch ui/node_modules

# The output directory is emptied first, keeping .gitkeep. Vite cannot do this
# itself: emptyOutDir must stay false because outDir is outside the Vite project
# root and Vite would wipe the tracked .gitkeep (Universal Build Contract). With
# it left uncleaned, Rollup's content-hashed filenames mean every rebuild ADDS a
# chunk instead of replacing one, so internal/api/ui/ accumulates orphans that
# `go:embed` ships inside the binary — and UI_BUILD_HASH, an md5 over that whole
# directory, becomes a function of the machine's build history rather than of
# the source. Two clean checkouts at one commit produced different hashes before
# this line, which defeats the point of embedding the hash at all.
$(UI_BUNDLE): ui/node_modules $(UI_SOURCES)
	find internal/api/ui -mindepth 1 ! -name .gitkeep -delete
	cd ui && npm run build

# Local .deb/.rpm for validating an install on the dev servers. The published
# packages come from release.yml through goreleaser-cross; this is the same
# .goreleaser.yml, snapshot-versioned, with signing and SBOMs skipped because
# both need CI's OIDC identity and syft. Artifacts land in dist/.
# The darwin target is the only cgo one (CoreWLAN, ADR-0006). CI cross-compiles
# it with osxcross inside goreleaser-cross; a Mac has clang already, and a Linux
# host has neither, so there the darwin build is skipped and the deb/rpm this
# target exists for are still produced.
ifeq ($(shell uname -s),Darwin)
PACKAGE_ENV = TRELLIS_DARWIN_CC=clang TRELLIS_DARWIN_CXX=clang++
else
PACKAGE_ENV = TRELLIS_SKIP_DARWIN=true
endif

packages: ui
	$(PACKAGE_ENV) UI_BUILD_HASH=$(UI_BUILD_HASH) goreleaser release --snapshot --clean --skip=sign,sbom,publish

# Reinstall whenever the installed version does not match the pin.
golangci-lint:
	@if ! "$(GOLANGCI_LINT)" version 2>/dev/null | grep -q "$(GOLANGCI_LINT_VERSION:v%=%)"; then \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	fi

# Same shape, plus an assert the golangci-lint target does not have: an install
# that silently produces some other version would be a new false clear rather
# than the one this target removes, so the version is checked again after it.
buf:
	@if ! "$(BUF)" --version 2>/dev/null | grep -qx "$(BUF_VERSION:v%=%)"; then \
		$(BUF_INSTALL); \
	fi
	@got="$$("$(BUF)" --version 2>/dev/null)"; \
	if [ "$$got" != "$(BUF_VERSION:v%=%)" ]; then \
		echo "buf is $${got:-absent}, the Makefile pins $(BUF_VERSION:v%=%)" >&2; \
		exit 1; \
	fi
