#!/bin/bash
# Run script for Negev CLI

set -e

# Variables
BIN="${BIN:-negev}"
BIN_DIR="${BIN_DIR:-bin}"

# Check if binary exists
if [ ! -f "${BIN_DIR}/${BIN}" ]; then
    echo "Error: Binary not found: ${BIN_DIR}/${BIN}"
    echo "Run 'make build' first"
    exit 1
fi

echo "Running ${BIN}..."
echo "Usage: ${BIN_DIR}/${BIN} -t <switch_ip> [options]"
echo "Example: ${BIN_DIR}/${BIN} -t 192.168.1.1 -v 1 -y examples/config.yaml"
echo ""

# Execute with all passed arguments
exec "${BIN_DIR}/${BIN}" "$@"