#!/bin/bash
# Install script for negev binary

set -e

BINARY_NAME="negev"
BUILD_DIR="./bin"

if [ ! -f "$BUILD_DIR/$BINARY_NAME" ]; then
    echo "Error: Binary $BUILD_DIR/$BINARY_NAME not found. Run 'make build' first."
    exit 1
fi

# Install based on user permissions
if [ "$(id -u)" -eq 0 ]; then
    echo "Installing $BINARY_NAME to /usr/local/bin"
    cp "$BUILD_DIR/$BINARY_NAME" /usr/local/bin/
    echo "Installation completed successfully"
else
    echo "Installing $BINARY_NAME to \$HOME/.local/bin"
    mkdir -p "$HOME/.local/bin"
    cp "$BUILD_DIR/$BINARY_NAME" "$HOME/.local/bin/"
    echo "Installation completed successfully"
    echo "Note: Make sure \$HOME/.local/bin is in your PATH"
fi