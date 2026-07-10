.PHONY: generate build lint vet test fmt-check

generate:
	buf generate

build:
	go build ./...

lint:
	golangci-lint run ./internal/... ./cmd/...
	buf lint

vet:
	go vet ./...

test:
	go test ./...

fmt-check:
	@fmt_out="$$(gofmt -l .)"; \
	if [ -n "$$fmt_out" ]; then \
		echo "$$fmt_out"; \
		exit 1; \
	fi
