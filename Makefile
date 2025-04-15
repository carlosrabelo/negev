MAKEFLAGS += --no-print-directory

.DEFAULT_GOAL := help

.PHONY: build build-static clean deps fmt help info install lint quality run test uninstall version

BINARY_NAME := negev
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

build: ## Build binary for current platform
	@BINARY_NAME=$(BINARY_NAME) VERSION=$(VERSION) BUILD_TIME=$(BUILD_TIME) ./.make/build.sh

build-static: ## Build statically linked binary
	@BINARY_NAME=$(BINARY_NAME) VERSION=$(VERSION) BUILD_TIME=$(BUILD_TIME) STATIC=true ./.make/build.sh

clean: ## Clean build artifacts
	@./.make/clean.sh

deps: ## Download Go dependencies
	@go mod download

fmt: ## Format source code
	@go fmt ./...

help: ## Show available targets
	@echo "NEGEV - VLAN Automation Tool"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*## "} {printf "  %-15s %s\n", $$1, $$2}'

info: ## Show project information
	@echo "Negev - VLAN Automation Tool"
	@echo "===================="
	@echo "Binary: $(BINARY_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Go Version: $(shell go version)"
	@echo "Platform: $(shell go env GOOS)/$(shell go env GOARCH)"

install: build-static ## Install binary
	@BINARY_NAME=$(BINARY_NAME) ./.make/install.sh

lint: ## Check code quality
	@go vet ./...

quality: fmt lint ## Run all quality checks

run: build ## Run compiled binary
	@BINARY_NAME=$(BINARY_NAME) ./.make/run.sh

test: ## Run project tests
	@./.make/test.sh

uninstall: ## Uninstall binary
	@BINARY_NAME=$(BINARY_NAME) ./.make/uninstall.sh

version: ## Show current version
	@echo "$(BINARY_NAME) version $(VERSION) (built $(BUILD_TIME))"
