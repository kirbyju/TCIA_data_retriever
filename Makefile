# Makefile for NBIA Data Retriever CLI

# Variables
BINARY_NAME := nbia-data-retriever-cli
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GO_VERSION := $(shell go version | awk '{print $$3}')

# Build flags
LDFLAGS := -ldflags "-s -w \
	-X main.version=$(VERSION) \
	-X main.buildStamp=$(BUILD_TIME) \
	-X main.gitHash=$(GIT_COMMIT) \
	-X main.goVersion=$(GO_VERSION)"

# Directories
DIST_DIR := dist
TEST_DIR := tests

.PHONY: all build test clean install release release-snapshot help

# Default target
all: clean build

# Build for current platform
build:
	@echo "Building $(BINARY_NAME) $(VERSION) for current platform..."
	@go build $(LDFLAGS) -o $(BINARY_NAME) .
	@echo "Build complete: ./$(BINARY_NAME)"

# Run tests
test:
	@echo "Running Go tests..."
	@go test -v ./...
	@echo "Running integration tests..."
	@cd $(TEST_DIR) && ./run_all_tests.sh

# Run specific test suite
test-%:
	@cd $(TEST_DIR) && ./test_$*.sh

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(DIST_DIR) $(BINARY_NAME) $(BINARY_NAME)-* *.exe
	@echo "Clean complete"

# Install binary to system
install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	@sudo cp $(BINARY_NAME) /usr/local/bin/
	@echo "Installation complete"

# Uninstall binary from system
uninstall:
	@echo "Removing $(BINARY_NAME) from /usr/local/bin..."
	@sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Uninstall complete"

# Create a release using goreleaser
release:
	@echo "Creating release $(VERSION)..."
	@if ! command -v goreleaser &> /dev/null; then \
		echo "Error: goreleaser not found. Install it from https://goreleaser.com"; \
		exit 1; \
	fi
	@GO_VERSION=$(GO_VERSION) goreleaser release --clean

# Create a snapshot release (doesn't publish)
release-snapshot:
	@echo "Creating snapshot release..."
	@if ! command -v goreleaser &> /dev/null; then \
		echo "Error: goreleaser not found. Install it from https://goreleaser.com"; \
		exit 1; \
	fi
	@GO_VERSION=$(GO_VERSION) goreleaser release --snapshot --clean

# Development build with race detector
dev:
	@echo "Building development version with race detector..."
	@go build -race $(LDFLAGS) -o $(BINARY_NAME) .
	@echo "Development build complete: ./$(BINARY_NAME)"

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@gofmt -w *.go

# Run linters
lint:
	@echo "Running linters..."
	@if command -v golangci-lint &> /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found, running go vet instead"; \
		go vet ./...; \
	fi

# Update dependencies
deps:
	@echo "Updating dependencies..."
	@go mod download
	@go mod tidy
	@go mod verify

# Run the tool with test manifest
run: build
	@./$(BINARY_NAME) -i test.tcia

# Show help
help:
	@echo "NBIA Data Retriever CLI - Build Targets"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all              Clean and build (default)"
	@echo "  build            Build binary for current platform"
	@echo "  test             Run all tests"
	@echo "  test-<name>      Run specific test suite (e.g., test-smoke)"
	@echo "  clean            Remove build artifacts"
	@echo "  install          Install binary to /usr/local/bin"
	@echo "  uninstall        Remove binary from /usr/local/bin"
	@echo "  release          Create a release with goreleaser"
	@echo "  release-snapshot Create a snapshot release (no publish)"
	@echo "  dev              Build with race detector"
	@echo "  fmt              Format code"
	@echo "  lint             Run linters"
	@echo "  deps             Update dependencies"
	@echo "  run              Build and run with test manifest"
	@echo "  help             Show this help message"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION:  $(VERSION)"
	@echo "  COMMIT:   $(GIT_COMMIT)"
	@echo "  GO:       $(GO_VERSION)"