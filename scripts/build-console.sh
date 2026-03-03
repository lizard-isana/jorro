#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SRC_DIR="$ROOT_DIR/src"
DIST_DIR="$ROOT_DIR/dist"
TARGET_OS="${GOOS:-$(go env GOOS)}"
BIN_NAME="jorro-cli"

if [[ "$TARGET_OS" == "windows" ]]; then
  BIN_NAME="${BIN_NAME}.exe"
fi

mkdir -p "$DIST_DIR"

go build -o "$DIST_DIR/$BIN_NAME" "$SRC_DIR"

echo "Built: $DIST_DIR/$BIN_NAME"
