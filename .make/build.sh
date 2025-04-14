#!/bin/bash
set -euo pipefail

BINARY_NAME="${BINARY_NAME:-negev}"
CMD_PATH="./negev/cmd/negev"
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${VERSION:-dev}"
BUILD_TIME="${BUILD_TIME:-unknown}"

GOCACHE="${GOCACHE:-$(pwd)/.gocache}"
export GOCACHE
mkdir -p "$GOCACHE" "$ROOT_DIR/bin"

cd "$ROOT_DIR"
go build -ldflags "-X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}" -o "$ROOT_DIR/bin/$BINARY_NAME" "$CMD_PATH"
echo "Binary ready at: $ROOT_DIR/bin/$BINARY_NAME"
