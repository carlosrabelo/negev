MAKEFLAGS += --no-print-directory

.DEFAULT_GOAL := help

.PHONY: build build-static clean deps fmt help info install install-system lint quality run test uninstall uninstall-system version

BINARY_NAME := negev
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

export BINARY_NAME

help: ## Show available targets
	@echo "NEGEV - VLAN Automation Tool"
	@echo ""
	@grep -hE '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) \
		| sort \
		| awk 'BEGIN {FS = ":.*## "} {printf "  %-15s %s\n", $$1, $$2}'

build: ## Build the binary
	@BINARY_NAME=$(BINARY_NAME) VERSION=$(VERSION) BUILD_TIME=$(BUILD_TIME) ./.make/build.sh

build-static: ## Build statically linked binary
	@BINARY_NAME=$(BINARY_NAME) VERSION=$(VERSION) BUILD_TIME=$(BUILD_TIME) STATIC=true ./.make/build.sh

test: ## Run tests
	@./.make/test.sh

lint: ## Run linter
	@go vet ./...

fmt: ## Format code
	@go fmt ./...

quality: fmt lint ## Run all quality checks

clean: ## Remove build artifacts
	@./.make/clean.sh

deps: ## Download Go dependencies
	@go mod download

info: ## Show project information
	@echo "Negev - VLAN Automation Tool"
	@echo "===================="
	@echo "Binary: $(BINARY_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Go Version: $(shell go version)"
	@echo "Platform: $(shell go env GOOS)/$(shell go env GOARCH)"

install: ## Install to ~/.local/bin (requires prior make build)
	@./.make/install.sh

install-system: ## Install to /usr/local/bin (requires prior make build; sudo only for copy)
	@SYSTEM=1 ./.make/install.sh

uninstall: ## Remove from ~/.local/bin (sudo only if needed)
	@./.make/uninstall.sh

uninstall-system: ## Remove from /usr/local/bin (sudo only if needed)
	@SYSTEM=1 ./.make/uninstall.sh

run: ## Run compiled binary (requires prior make build)
	@BINARY_NAME=$(BINARY_NAME) ./.make/run.sh

version: ## Show current version
	@echo "$(BINARY_NAME) version $(VERSION) (built $(BUILD_TIME))"
