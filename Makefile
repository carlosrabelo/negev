CORE_DIR := core

.DEFAULT_GOAL := help

# Configurable variables
BIN ?= negev
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS ?= -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

# Directories
BIN_DIR := bin

# Go settings
GOTOOLCHAIN ?= go1.25.4
GO_BIN ?= $(shell if [ -x "$(HOME)/go/bin/$(GOTOOLCHAIN)" ]; then echo "$(HOME)/go/bin/$(GOTOOLCHAIN)"; else echo "go"; fi)
GOOS ?= $(shell $(GO_BIN) env GOOS)
GOARCH ?= $(shell $(GO_BIN) env GOARCH)

.PHONY: build clean deps fmt help info install lint run test version

build: ## Build binary for current platform
	@GO_BIN=$(GO_BIN) GOTOOLCHAIN=$(GOTOOLCHAIN) ./scripts/build.sh

clean: ## Clean build artifacts
	@./scripts/clean.sh

deps: ## Download Go dependencies
	@GO_BIN=$(GO_BIN) GOTOOLCHAIN=$(GOTOOLCHAIN) $(MAKE) -C $(CORE_DIR) deps

fmt: ## Format source code
	@GO_BIN=$(GO_BIN) GOTOOLCHAIN=$(GOTOOLCHAIN) $(MAKE) -C $(CORE_DIR) fmt

help: ## Show available targets
	@echo "NEGEV - VLAN Automation Tool for Cisco Switches"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) \
		| sort \
		| awk 'BEGIN {FS = ":.*## "} {printf "  %-15s %s\n", $$1, $$2}'

info: ## Show project information
	@echo "Negev - VLAN Automation Tool for Cisco Switches"
	@echo "=================================================="
	@echo "Binary: $(BIN)"
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Go Version: $(shell $(GO_BIN) version)"
	@echo "Platform: $(GOOS)/$(GOARCH)"
	@echo "Binary Dir: $(BIN_DIR)"

install: build ## Install binary (root: /usr/local/bin, user: ~/.local/bin)
	@./scripts/install.sh

lint: ## Check code quality
	@GO_BIN=$(GO_BIN) GOTOOLCHAIN=$(GOTOOLCHAIN) $(MAKE) -C $(CORE_DIR) lint

run: build ## Run compiled binary
	@./scripts/run.sh

test: ## Run project tests
	@GO_BIN=$(GO_BIN) GOTOOLCHAIN=$(GOTOOLCHAIN) $(MAKE) -C $(CORE_DIR) test

version: ## Show current version
	@echo "$(BIN) version $(VERSION) (built $(BUILD_TIME))"
