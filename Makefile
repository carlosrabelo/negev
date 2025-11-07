

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
GOTOOLCHAIN ?= go1.25.4
GO_BIN ?= $(shell if [ -x "$(HOME)/go/bin/$(GOTOOLCHAIN)" ]; then echo "$(HOME)/go/bin/$(GOTOOLCHAIN)"; else echo "go"; fi)
GOOS ?= $(shell $(GO_BIN) env GOOS)
GOARCH ?= $(shell $(GO_BIN) env GOARCH)

.PHONY: build clean help info run test lint fmt deps install

# Build targets
build: ## Build binary for current platform
	@GO_BIN=$(GO_BIN) GOTOOLCHAIN=$(GOTOOLCHAIN) ./scripts/build.sh

# Development targets
run: build ## Run compiled binary
	@./scripts/run.sh

# Testing targets
test: ## Run project tests
	GO_BIN=$(GO_BIN) GOTOOLCHAIN=$(GOTOOLCHAIN) $(MAKE) -C $(CORE_DIR) test

# Code quality targets
lint: ## Check code quality
	GO_BIN=$(GO_BIN) GOTOOLCHAIN=$(GOTOOLCHAIN) $(MAKE) -C $(CORE_DIR) lint

fmt: ## Format source code
	GO_BIN=$(GO_BIN) GOTOOLCHAIN=$(GOTOOLCHAIN) $(MAKE) -C $(CORE_DIR) fmt

# Dependency management
deps: ## Download Go dependencies
	GO_BIN=$(GO_BIN) GOTOOLCHAIN=$(GOTOOLCHAIN) $(MAKE) -C $(CORE_DIR) deps

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
	@echo "Go Version: $(shell $(GO_BIN) version)"
	@echo "Platform: $(GOOS)/$(GOARCH)"
	@echo "Binary Dir: $(BIN_DIR)"

version: ## Show current version
	@echo "$(BIN) version $(VERSION) (built $(BUILD_TIME))"

help: ## Show this help
	@echo "NEGEV - VLAN Automation Tool for Cisco Switches"
	@echo "=============================================="
	@echo ""
	@echo " Build & Install:"
	@echo "   build           Build CLI binary"
	@echo "   install         Install CLI system-wide"
	@echo "   run             Run compiled binary"
	@echo ""
	@echo " Quality:"
	@echo "   fmt             Format Go sources with gofmt"
	@echo "   lint            Run golangci-lint via core/"
	@echo ""
	@echo " Testing:"
	@echo "   test            Run go test ./... in core/"
	@echo ""
	@echo "CLI usage:"
	@echo "  $(BIN) -t <ip> [-v level] [-y config] [-w] [-c] [-s]"
	@echo " Utilities:"
	@echo "   clean           Clean build artifacts from core/"
	@echo "   deps            Download Go dependencies"
	@echo "   info            Show project information"
	@echo "   version         Show current version"
	@echo "   help            Show this help"
