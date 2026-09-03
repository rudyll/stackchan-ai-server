#!/bin/bash
set -eu

# Keep the web and Supervisor images derived from the macOS icon master.
ROOT=$(cd "$(dirname "$0")/.." && pwd)
ASSETS="$ROOT/stackchan-server/server/internal/service/ai/assets"
SOURCE="$ASSETS/stackchan-icon.png"
command -v sips >/dev/null 2>&1 || { echo "ERROR: macOS sips is required" >&2; exit 1; }
sips -z 256 256 "$SOURCE" --out "$ASSETS/stackchan-mark.png" >/dev/null
cp "$ASSETS/stackchan-mark.png" "$ROOT/stackchan-server/logo.png"
sips -z 128 128 "$SOURCE" --out "$ROOT/stackchan-server/icon.png" >/dev/null
echo "Updated GUI mark, HA logo (256px) and HA icon (128px)."
