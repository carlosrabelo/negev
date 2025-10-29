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
GOTOOLCHAIN ?= go1.25.3
GO_BIN ?= $(shell if [ -x "$(HOME)/go/bin/$(GOTOOLCHAIN)" ]; then echo "$(HOME)/go/bin/$(GOTOOLCHAIN)"; else echo "go"; fi)
GOCACHE_DIR ?= $(CURDIR)/.gocache
GOOS ?= $(shell $(GO_BIN) env GOOS)
GOARCH ?= $(shell $(GO_BIN) env GOARCH)

.PHONY: build clean help info run test lint fmt deps

# Build targets
build: ## Build binary for current platform
	@echo "Building $(BIN) for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BIN_DIR)
	@mkdir -p $(GOCACHE_DIR)
	GOCACHE=$(GOCACHE_DIR) GOTOOLCHAIN=$(GOTOOLCHAIN) $(GO_BIN) build $(LDFLAGS) -o $(BIN_DIR)/$(BIN) ./cmd/$(BIN)

# Development targets
run: ## Run binary via go run
	@echo "Running $(BIN) via go run..."
	@echo "Usage: go run ./cmd/$(BIN) -t <switch_ip> [options]"
	@echo "Example: go run ./cmd/$(BIN) -t 192.168.1.1 -v 1 -y ../examples/config.yaml"

# Testing targets
test: ## Run module tests
	@echo "Running tests..."
	@mkdir -p $(GOCACHE_DIR)
	GOCACHE=$(GOCACHE_DIR) GOTOOLCHAIN=$(GOTOOLCHAIN) $(GO_BIN) test -v ./...

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
	@mkdir -p $(GOCACHE_DIR)
	GOCACHE=$(GOCACHE_DIR) GOTOOLCHAIN=$(GOTOOLCHAIN) $(GO_BIN) fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		GOCACHE=$(GOCACHE_DIR) GOTOOLCHAIN=$(GOTOOLCHAIN) goimports -w .; \
	fi

# Dependency management

deps: ## Download Go dependencies
	@echo "Downloading dependencies..."
	@mkdir -p $(GOCACHE_DIR)
	GOCACHE=$(GOCACHE_DIR) GOTOOLCHAIN=$(GOTOOLCHAIN) $(GO_BIN) mod download


deps-tidy: ## Clean and organize dependencies
	@echo "Tidying dependencies..."
	@mkdir -p $(GOCACHE_DIR)
	GOCACHE=$(GOCACHE_DIR) GOTOOLCHAIN=$(GOTOOLCHAIN) $(GO_BIN) mod tidy

# Maintenance targets
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@if [ -d "$(BIN_DIR)" ]; then find $(BIN_DIR) -type f ! -name ".gitkeep" -delete 2>/dev/null || true; fi
	@rm -f *.test *.out *.prof

# Information targets
info: ## Show module information
	@echo "Core Module - Negev VLAN Automation"
	@echo "===================================="
	@echo "Module: $(shell $(GO_BIN) list -m)"
	@echo "Binary: $(BIN)"
	@echo "Version: $(VERSION)"
	@echo "Go Version: $(shell $(GO_BIN) version)"
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
