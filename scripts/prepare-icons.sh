#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SOURCE_PNG="${1:-$ROOT_DIR/assets/icon-source.png}"
OUT_DIR="${2:-$ROOT_DIR/dist/icons}"
ICONSET_DIR="$OUT_DIR/jorro.iconset"
ICNS_OUT="$OUT_DIR/jorro.icns"
ICO_OUT="$OUT_DIR/jorro.ico"
ICON_MODE="${ICON_MODE:-all}"

if [[ ! -f "$SOURCE_PNG" ]]; then
  echo "Error: icon source not found: $SOURCE_PNG"
  echo "Place your image at assets/icon-source.png or pass a path to this script."
  exit 1
fi

if ! command -v magick >/dev/null 2>&1; then
  echo "Error: ImageMagick (magick) is required."
  exit 1
fi

if [[ "$ICON_MODE" != "all" && "$ICON_MODE" != "ico" && "$ICON_MODE" != "icns" ]]; then
  echo "Error: ICON_MODE must be one of: all, ico, icns"
  exit 1
fi

if [[ "$ICON_MODE" == "all" || "$ICON_MODE" == "icns" ]]; then
  if ! command -v iconutil >/dev/null 2>&1; then
    echo "Error: iconutil is required to build .icns."
    exit 1
  fi
fi

mkdir -p "$OUT_DIR"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT
BASE_PNG="$TMP_DIR/base-1024.png"

# Normalize to a square transparent canvas for stable icon results.
magick "$SOURCE_PNG" \
  -alpha on \
  -background none \
  -resize 1024x1024^ \
  -gravity center \
  -extent 1024x1024 \
  "$BASE_PNG"

if [[ "$ICON_MODE" == "all" || "$ICON_MODE" == "icns" ]]; then
  rm -rf "$ICONSET_DIR"
  mkdir -p "$ICONSET_DIR"
  for size in 16 32 128 256 512; do
    size2x=$((size * 2))
    magick "$BASE_PNG" -resize "${size}x${size}" "$ICONSET_DIR/icon_${size}x${size}.png"
    magick "$BASE_PNG" -resize "${size2x}x${size2x}" "$ICONSET_DIR/icon_${size}x${size}@2x.png"
  done
  iconutil -c icns "$ICONSET_DIR" -o "$ICNS_OUT"
  echo "Generated: $ICNS_OUT"
fi

if [[ "$ICON_MODE" == "all" || "$ICON_MODE" == "ico" ]]; then
  magick "$BASE_PNG" -define icon:auto-resize=16,24,32,48,64,128,256 "$ICO_OUT"
  echo "Generated: $ICO_OUT"
fi
