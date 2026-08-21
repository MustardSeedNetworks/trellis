.PHONY: generate generate-ts build lint vet test fmt-check check-stale-tests

generate:
	buf generate

generate-ts:
	buf generate --template buf.gen.ui.yaml

build:
	go build ./...

lint:
	golangci-lint run ./internal/... ./cmd/...
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
