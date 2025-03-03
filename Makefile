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
	@mkdir -p bin
	@go build -o bin/$(BINARY_NAME) ./negev/cmd/negev
	@echo "Binary ready at: bin/$(BINARY_NAME)"

build-static: ## Build statically linked binary
	@mkdir -p bin
	@CGO_ENABLED=0 go build -o bin/$(BINARY_NAME) ./negev/cmd/negev

test: ## Run tests
	@go test -v ./...
	@echo "Tests completed"

lint: ## Run linter
	@go vet ./...

fmt: ## Format code
	@go fmt ./...

quality: fmt lint ## Run all quality checks

clean: ## Remove build artifacts
	@rm -f bin/$(BINARY_NAME)
	@echo "Clean completed"

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
	@install -D -m 755 bin/$(BINARY_NAME) $(HOME)/.local/bin/$(BINARY_NAME)
	@echo "Installed: $(HOME)/.local/bin/$(BINARY_NAME)"

install-system: ## Install to /usr/local/bin (requires prior make build; sudo only for copy)
	@sudo install -D -m 755 bin/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@echo "Installed: /usr/local/bin/$(BINARY_NAME)"

uninstall: ## Remove from ~/.local/bin (sudo only if needed)
	@rm -f $(HOME)/.local/bin/$(BINARY_NAME)
	@echo "Removed: $(HOME)/.local/bin/$(BINARY_NAME)"

uninstall-system: ## Remove from /usr/local/bin (sudo only if needed)
	@sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Removed: /usr/local/bin/$(BINARY_NAME)"

run: ## Run compiled binary (requires prior make build)
	@./bin/$(BINARY_NAME)

version: ## Show current version
	@echo "$(BINARY_NAME) version $(VERSION) (built $(BUILD_TIME))"
