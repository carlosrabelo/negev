#!/bin/bash
# Clean script for Negev CLI

set -e

# Variables
BIN_DIR="${BIN_DIR:-bin}"

echo "Cleaning build artifacts..."

# Remove binary files but keep directory structure
if [ -d "${BIN_DIR}" ]; then
    find "${BIN_DIR}" -type f ! -name ".gitkeep" -delete 2>/dev/null || true
    echo "Cleaned ${BIN_DIR}/"
fi

# Remove Go artifacts
rm -f *.test *.out *.prof 2>/dev/null || true

echo "Clean completed"