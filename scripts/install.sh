#!/bin/bash
# Installation script for Negev CLI

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

# Install based on user
if [ "$(id -u)" = "0" ]; then
    echo "Installing ${BIN} to /usr/local/bin (root)"
    mkdir -p /usr/local/bin
    cp "${BIN_DIR}/${BIN}" /usr/local/bin/
    echo "Installed to /usr/local/bin/${BIN}"
else
    echo "Installing ${BIN} to \$HOME/.local/bin (user)"
    mkdir -p "$HOME/.local/bin"
    cp "${BIN_DIR}/${BIN}" "$HOME/.local/bin/"
    echo "Installed to $HOME/.local/bin/${BIN}"
    
    # Check PATH
    if ! echo "$PATH" | grep -q "$HOME/.local/bin"; then
        echo "Warning: Add $HOME/.local/bin to your PATH:"
        echo "   export PATH=\"\$HOME/.local/bin:\$PATH\""
        echo "   Add this to your ~/.bashrc or ~/.zshrc"
    fi
fi