.PHONY: generate generate-ts build lint vet test fmt-check check-stale-tests

generate:
	buf generate

generate-ts:
	buf generate --template buf.gen.ui.yaml

build:
	go build ./...

# The host platform's build tags decide which files golangci-lint even sees, so
# a macOS run never inspects capture_linux.go or capture_windows.go — three
# gosec findings reached CI that way. GOOS is enough to fix it for the pure-Go
# backends; the darwin backend needs cgo and so only lints on a Mac.
lint:
	golangci-lint run ./core/... ./internal/... ./cmd/...
	GOOS=linux golangci-lint run ./internal/capture/...
	GOOS=windows golangci-lint run ./internal/capture/...
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
