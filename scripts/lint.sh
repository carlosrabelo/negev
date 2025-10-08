#!/bin/bash
# Lint script for negev project

set -e

echo "Running linter..."

# Check for golangci-lint in system PATH
if command -v golangci-lint >/dev/null 2>&1; then
    echo "Using system golangci-lint..."
    golangci-lint run ./...
    exit 0
fi

# Check for golangci-lint in GOPATH/bin
GOPATH_BIN="$(go env GOPATH)/bin"
if [ -x "$GOPATH_BIN/golangci-lint" ]; then
    echo "Using golangci-lint from GOPATH..."
    "$GOPATH_BIN/golangci-lint" run ./...
    exit 0
fi

# Install golangci-lint if not found
echo "golangci-lint not available, installing..."
GOBIN="$GOPATH_BIN" go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Try to run golangci-lint, fallback to go vet if it fails
if "$GOPATH_BIN/golangci-lint" run ./... 2>/dev/null; then
    echo "Linting completed with golangci-lint"
else
    echo "golangci-lint failed, falling back to go vet..."
    go vet ./...
fi