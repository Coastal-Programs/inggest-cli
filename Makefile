BINARY    := inngest
CMD       := ./cmd/inngest
BUILD_DIR := ./build
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

LDFLAGS := -ldflags "-s -w \
	-X main.version=$(VERSION)"

.PHONY: build install clean tidy run test lint fmt fmt-check fix vet check hooks help release

## help: Show available make targets
help:
	@echo "Usage: make <target>"
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## build: Build binary to ./build/inngest
build:
	@mkdir -p $(BUILD_DIR)
	go build -trimpath $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) $(CMD)

## install: Install binary to GOPATH/bin
install:
	go install $(LDFLAGS) $(CMD)

## run: Run without installing (ARGS="..." to pass arguments)
run:
	go run $(LDFLAGS) $(CMD) $(ARGS)

## tidy: Tidy and verify go modules
tidy:
	go mod tidy
	go mod verify

## test: Run all tests with race detector and coverage
test:
	go test -v -race -coverprofile=coverage.out ./...

## coverage: Open test coverage report in browser
coverage: test
	go tool cover -html=coverage.out

## fmt: Format all Go source files (mutating)
fmt:
	go fmt ./...

## fmt-check: Verify formatting without modifying files
fmt-check:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "Unformatted files (run 'make fmt'):"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

## fix: Auto-fix formatting and lint issues (mutating)
fix:
	gofmt -w .
	@which golangci-lint > /dev/null 2>&1 || go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.3
	golangci-lint run --fix ./...

## vet: Run go vet
vet:
	go vet ./...

## lint: Run golangci-lint (installs if missing)
lint:
	@which golangci-lint > /dev/null 2>&1 || go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.3
	golangci-lint run ./...

## check: Run fmt-check, vet, and lint (non-mutating pre-commit gate)
check: fmt-check vet lint

## hooks: Install git hooks via lefthook (opt-in)
hooks:
	@which lefthook > /dev/null 2>&1 || { echo "lefthook not found — install: https://lefthook.dev"; exit 1; }
	lefthook install

## release: Build release binaries for all platforms
release:
	@chmod +x scripts/release.sh
	@./scripts/release.sh $(VERSION)

## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR) dist coverage.out
