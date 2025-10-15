

.DEFAULT_GOAL := help

# Configurable variables
BIN ?= negev
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS ?= -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

# Directories
BIN_DIR := bin
CORE_DIR := core

# Go settings
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

.PHONY: build clean help info run test lint fmt deps install

# Build targets
build: ## Build binary for current platform
	@./scripts/build.sh

# Development targets
run: build ## Run compiled binary
	@./scripts/run.sh

# Testing targets
test: ## Run project tests
	$(MAKE) -C $(CORE_DIR) test

# Code quality targets
lint: ## Check code quality
	$(MAKE) -C $(CORE_DIR) lint

fmt: ## Format source code
	$(MAKE) -C $(CORE_DIR) fmt

# Dependency management
deps: ## Download Go dependencies
	$(MAKE) -C $(CORE_DIR) deps

# Installation targets
install: build ## Install binary (root: /usr/local/bin, user: ~/.local/bin)
	@./scripts/install.sh

# Maintenance targets
clean: ## Clean build artifacts
	@./scripts/clean.sh

# Information targets
info: ## Show project information
	@echo "Negev - VLAN Automation Tool for Cisco Switches"
	@echo "=================================================="
	@echo "Binary: $(BIN)"
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Go Version: $(shell go version)"
	@echo "Platform: $(GOOS)/$(GOARCH)"
	@echo "Binary Dir: $(BIN_DIR)"

version: ## Show current version
	@echo "$(BIN) version $(VERSION) (built $(BUILD_TIME))"

help: ## Show this help
	@echo "Negev - VLAN Automation Tool for Cisco Switches"
	@echo ""
	@echo "Usage: make [TARGET]"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | sort | awk -F':|## ' '{printf " %-20s %s\n", $$1, $$3}'
	@echo ""
	@echo "Examples:"
	@echo "  make build            # Build the CLI"
	@echo "  make run              # Run the compiled CLI"
	@echo "  make install          # Install the CLI"
	@echo ""
	@echo "CLI usage:"
	@echo "  $(BIN) -t <ip> [-v level] [-y config] [-w] [-c] [-s]"