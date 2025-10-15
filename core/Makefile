# Load environment variables if exists
-include ../.env

.DEFAULT_GOAL := help

# Configurable variables
BIN ?= negev
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS ?= -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

# Directories
BIN_DIR := ../bin

# Go settings
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

.PHONY: build clean help info run test lint fmt deps

# Build targets
build: ## Build binary for current platform
	@echo "Building $(BIN) for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BIN_DIR)
	go build $(LDFLAGS) -o $(BIN_DIR)/$(BIN) ./cmd/$(BIN)

# Development targets
run: ## Run binary via go run
	@echo "Running $(BIN) via go run..."
	@echo "Usage: go run ./cmd/$(BIN) -t <switch_ip> [options]"
	@echo "Example: go run ./cmd/$(BIN) -t 192.168.1.1 -v 1 -y ../examples/config.yaml"

# Testing targets
test: ## Run module tests
	@echo "Running tests..."
	go test -v ./...

# Code quality targets
lint: ## Check code quality
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

fmt: ## Format source code
	@echo "Formatting code..."
	go fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	fi

# Dependency management
deps: ## Download Go dependencies
	@echo "Downloading dependencies..."
	go mod download

deps-tidy: ## Clean and organize dependencies
	@echo "Tidying dependencies..."
	go mod tidy

# Maintenance targets
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@if [ -d "$(BIN_DIR)" ]; then find $(BIN_DIR) -type f ! -name ".gitkeep" -delete 2>/dev/null || true; fi
	@rm -f *.test *.out *.prof

# Information targets
info: ## Show module information
	@echo "Core Module - Negev VLAN Automation"
	@echo "===================================="
	@echo "Module: $(shell go list -m)"
	@echo "Binary: $(BIN)"
	@echo "Version: $(VERSION)"
	@echo "Go Version: $(shell go version)"
	@echo "Platform: $(GOOS)/$(GOARCH)"

help: ## Show this help
	@echo "Core Module - Negev VLAN Automation"
	@echo ""
	@echo "Usage: make [TARGET]"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | sort | awk -F':|## ' '{printf " %-20s %s\n", $$1, $$3}'
	@echo ""
	@echo "Examples:"
	@echo "  make build            # Build binary"
	@echo "  make run              # Run via go run"
	@echo "  make test             # Run tests"
	@echo "  make lint             # Check code quality"