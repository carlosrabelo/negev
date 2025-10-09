# Negev - VLAN automation tool
# Build automation for Go project

# Variables
GO		= go
BIN		= negev
SRC		= ./cmd/negev
BUILD_DIR	= ./bin
VERSION		= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BUILD_TIME	= $(shell date +%Y-%m-%dT%H:%M:%S%z)
LDFLAGS		= -s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)

# Default target - show help
.DEFAULT_GOAL := help

.PHONY: all build clean deps fmt help info install lint mod-tidy run test vet

help:	## Show this help
	@echo "Negev - Available targets:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'

all: clean fmt vet build	## Clean, format, vet and build

build:	## Build the project
	@echo "Building $(BIN)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -tags netgo -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BIN) $(SRC)

clean:	## Remove build artifacts and Go caches
	@echo "Cleaning build artifacts and caches..."
	@rm -rf $(BUILD_DIR)
	@$(GO) clean -cache -testcache -modcache 2>/dev/null || true

deps:	## Download and verify Go module dependencies
	$(GO) mod download
	$(GO) mod verify

fmt:	## Format source code
	$(GO) fmt ./...

info:	## Show project information
	@echo "Project:      Negev - VLAN automation tool for Cisco switches"
	@echo "Binary:       $(BIN)"
	@echo "Source:       $(SRC)"
	@echo "Build dir:    $(BUILD_DIR)"
	@echo "Version:      $(VERSION)"
	@echo "Build time:   $(BUILD_TIME)"
	@echo "Go version:   $$($(GO) version)"
	@echo "Dependencies: telnet, ssh, yaml parsing"
	@echo "Platforms:    Linux, macOS, Windows (amd64, arm64)"

install: build	## Install binary to user path
	@./scripts/install.sh

lint:	## Run golangci-lint if available, fallback to go vet
	@./scripts/lint.sh

mod-tidy:	## Tidy go.mod and go.sum
	$(GO) mod tidy

run:	## Run the application with current sources
	$(GO) run $(SRC)

test:	## Run unit tests
	$(GO) test ./...

vet:	## Run go vet
	$(GO) vet ./...
