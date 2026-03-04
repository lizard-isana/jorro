#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SRC_DIR="$ROOT_DIR/src"
DIST_DIR="$ROOT_DIR/dist"
ICONS_DIR="$DIST_DIR/icons"
PREPARE_ICONS_SCRIPT="$ROOT_DIR/scripts/prepare-icons.sh"
TMP_SYSO="$SRC_DIR/zz_jorro_windows_amd64.syso"
RSRC_BIN=""

mkdir -p "$DIST_DIR"
ICON_MODE=ico "$PREPARE_ICONS_SCRIPT"
cp "$ICONS_DIR/jorro.ico" "$DIST_DIR/jorro.ico"

if command -v rsrc >/dev/null 2>&1; then
  RSRC_BIN="$(command -v rsrc)"
elif [[ -x "$HOME/go/bin/rsrc" ]]; then
  RSRC_BIN="$HOME/go/bin/rsrc"
else
  GOPATH_BIN="$(go env GOPATH 2>/dev/null)/bin/rsrc"
  if [[ -x "$GOPATH_BIN" ]]; then
    RSRC_BIN="$GOPATH_BIN"
  fi
fi

if [[ -n "$RSRC_BIN" ]]; then
  "$RSRC_BIN" -ico "$ICONS_DIR/jorro.ico" -o "$TMP_SYSO"
  trap 'rm -f "$TMP_SYSO"' EXIT
else
  echo "Warning: rsrc not found, building without embedded EXE icon."
  echo "         Use dist/jorro.ico for shortcuts or install rsrc to embed icon."
fi

GOOS=windows GOARCH=amd64 go build \
  -trimpath \
  -ldflags "-H=windowsgui -s -w" \
  -o "$DIST_DIR/jorro.exe" \
  "$SRC_DIR"

echo "Built: $DIST_DIR/jorro.exe"
