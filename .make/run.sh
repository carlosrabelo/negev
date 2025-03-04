#!/bin/bash
set -euo pipefail

BINARY_NAME="${BINARY_NAME:-negev}"
BIN_DIR="${BIN_DIR:-bin}"
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

if [ ! -f "$ROOT_DIR/$BIN_DIR/$BINARY_NAME" ]; then
    echo "Error: Binary not found. Run 'make build' first."
    exit 1
fi

if [ $# -eq 0 ]; then
    echo "Usage: $ROOT_DIR/$BIN_DIR/$BINARY_NAME --target <switch_ip> [options]"
    exit 1
fi

exec "$ROOT_DIR/$BIN_DIR/$BINARY_NAME" "$@"
