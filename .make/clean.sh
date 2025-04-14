#!/bin/bash
set -euo pipefail

BINARY_NAME="${BINARY_NAME:-negev}"
BIN_DIR="${BIN_DIR:-bin}"
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

rm -f "$ROOT_DIR/$BIN_DIR/$BINARY_NAME"
rm -rf "$ROOT_DIR/.gocache"

# Clean test artifacts
rm -f "$ROOT_DIR"/*.test "$ROOT_DIR"/*.out "$ROOT_DIR"/*.prof 2>/dev/null || true

echo "Clean completed"
