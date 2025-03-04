#!/bin/bash
set -euo pipefail

BINARY_NAME="${BINARY_NAME:-negev}"
CMD_PATH="./negev/cmd/negev"
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${VERSION:-dev}"
BUILD_TIME="${BUILD_TIME:-unknown}"

STATIC="${STATIC:-false}"

GOCACHE="${GOCACHE:-$(pwd)/.gocache}"
export GOCACHE
mkdir -p "$GOCACHE" "$ROOT_DIR/bin"

cd "$ROOT_DIR"

LDFLAGS="-X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}"
if [ "$STATIC" = "true" ]; then
    export CGO_ENABLED=0
    LDFLAGS="-extldflags -static -s -w $LDFLAGS"
fi

go build -ldflags "$LDFLAGS" -o "$ROOT_DIR/bin/$BINARY_NAME" "$CMD_PATH"
echo "Binary ready at: $ROOT_DIR/bin/$BINARY_NAME"
