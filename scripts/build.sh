#!/bin/bash
# Build script for Negev CLI

set -e

export GOTOOLCHAIN="${GOTOOLCHAIN:-go1.24.7}"
GO_BIN="${GO_BIN:-go}"

if ! command -v "${GO_BIN}" >/dev/null 2>&1; then
    echo "Error: Go binary '${GO_BIN}' not found."
    echo "Install go1.24.7 with 'go install golang.org/dl/go1.24.7@latest' and 'go1.24.7 download', or adjust GO_BIN."
    exit 1
fi

# Variables
BIN="${BIN:-negev}"
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}"
BUILD_TIME="${BUILD_TIME:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"
BIN_DIR="${BIN_DIR:-bin}"
GOOS="${GOOS:-$("${GO_BIN}" env GOOS)}"
GOARCH="${GOARCH:-$("${GO_BIN}" env GOARCH)}"

echo "Building ${BIN} for ${GOOS}/${GOARCH}..."
echo "Version: ${VERSION}"
echo "Build Time: ${BUILD_TIME}"

# Create output directory
mkdir -p "${BIN_DIR}"

# Build from core directory
cd core
GOCACHE="${GOCACHE:-${PWD}/.gocache}"
mkdir -p "${GOCACHE}"
GOCACHE="${GOCACHE}" GOTOOLCHAIN="${GOTOOLCHAIN}" "${GO_BIN}" build -ldflags "-X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}" -o "../${BIN_DIR}/${BIN}" ./cmd/${BIN}

echo "Build completed: ${BIN_DIR}/${BIN}"
