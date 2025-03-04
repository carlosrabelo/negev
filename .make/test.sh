#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

cd "$ROOT_DIR"
GOCACHE="${GOCACHE:-$(pwd)/.gocache}"
export GOCACHE
mkdir -p "$GOCACHE"

go test -v ./...
echo "Tests completed"
