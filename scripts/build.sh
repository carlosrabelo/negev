#!/bin/bash
# Build script for Negev CLI

set -e

export GOTOOLCHAIN="${GOTOOLCHAIN:-go1.24.7}"

# Variables
BIN="${BIN:-negev}"
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}"
BUILD_TIME="${BUILD_TIME:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"
BIN_DIR="${BIN_DIR:-bin}"
GOOS="${GOOS:-$(go env GOOS)}"
GOARCH="${GOARCH:-$(go env GOARCH)}"

echo "Building ${BIN} for ${GOOS}/${GOARCH}..."
echo "Version: ${VERSION}"
echo "Build Time: ${BUILD_TIME}"

# Create output directory
mkdir -p "${BIN_DIR}"

# Build from core directory
cd core
GOTOOLCHAIN="${GOTOOLCHAIN}" go build -ldflags "-X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}" -o "../${BIN_DIR}/${BIN}" ./cmd/${BIN}

echo "Build completed: ${BIN_DIR}/${BIN}"
