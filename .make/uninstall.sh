#!/bin/bash
set -euo pipefail

BINARY_NAME="negev"
INSTALL_DIR="${HOME}/.local/bin"

rm -f "$INSTALL_DIR/$BINARY_NAME"
echo "Removed: $INSTALL_DIR/$BINARY_NAME"
