#!/bin/bash
set -euo pipefail

BINARY_NAME="${BINARY_NAME:-negev}"
BIN_DIR="${BIN_DIR:-bin}"
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

if [ ! -f "$ROOT_DIR/$BIN_DIR/$BINARY_NAME" ]; then
    echo "Error: Binary not found. Run 'make build' or 'make build-static' first."
    exit 1
fi

if [ "$(id -u)" = "0" ]; then
    INSTALL_DIR="/usr/local/bin"
else
    INSTALL_DIR="${HOME}/.local/bin"
fi

mkdir -p "$INSTALL_DIR"
cp "$ROOT_DIR/$BIN_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
echo "Installed: $INSTALL_DIR/$BINARY_NAME"

if [ "$(id -u)" != "0" ] && ! echo "$PATH" | grep -q "$HOME/.local/bin"; then
    echo "Warning: Add $HOME/.local/bin to your PATH"
fi
