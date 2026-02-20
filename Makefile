BINARY    := xero
CMD       := ./cmd/xero
BUILD_DIR := ./build
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS   := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build install clean tidy run test lint fmt vet check help release

## help: Show available make targets
help:
	@echo "Usage: make <target>"
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## build: Build binary to ./build/xero
build:
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) $(CMD)

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

## fmt: Format all Go source files
fmt:
	go fmt ./...

## vet: Run go vet
vet:
	go vet ./...

## lint: Run golangci-lint (installs if missing)
lint:
	@which golangci-lint > /dev/null 2>&1 || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run ./...

## check: Run fmt, vet, and lint (pre-commit gate)
check: fmt vet lint

## release: Build release binaries for all platforms
release:
	@chmod +x scripts/release.sh
	@./scripts/release.sh $(VERSION)

## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR) dist coverage.out
