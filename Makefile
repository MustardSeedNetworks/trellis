.PHONY: generate generate-ts ui build build-e2e ui-build-hash lint vet test fmt-check check-stale-tests packages

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

generate:
	buf generate

generate-ts:
	buf generate --template buf.gen.ui.yaml

# Compiles the whole tree, then produces the daemon with the contract's ldflags.
build:
	go build ./...
	go build $(GOFLAGS) -o bin/trellisd ./cmd/trellisd

# The daemon the Playwright suite drives: the same recipe with the radio swapped
# for a scripted scanner (cmd/trellisd/scanner_e2e.go).
build-e2e:
	go build -tags e2e $(GOFLAGS) -o bin/trellisd-e2e ./cmd/trellisd

ui-build-hash:
	@./scripts/ui-build-hash.sh

# The host platform's build tags decide which files golangci-lint even sees, so
# a macOS run never inspects capture_linux.go or capture_windows.go — three
# gosec findings reached CI that way. GOOS is enough to fix it for the pure-Go
# backends and for the daemon's per-platform bind-error check; the darwin
# backend needs cgo and so only lints on a Mac.
lint:
	golangci-lint run ./core/... ./internal/... ./cmd/...
	GOOS=linux golangci-lint run ./internal/capture/... ./cmd/trellisd/...
	GOOS=windows golangci-lint run ./internal/capture/... ./cmd/trellisd/...
	golangci-lint run --build-tags e2e ./cmd/trellisd/...
	buf lint

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
ui:
	cd ui && npm ci && npm run build

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
