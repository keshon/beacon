#!/usr/bin/env bash
set -eu

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

INPUT="$ROOT_DIR/uikit/scss/uikit.scss"
OUTPUT="$ROOT_DIR/static/uikit.css"

if ! command -v sass >/dev/null 2>&1; then
  echo "ERROR: 'sass' not found in PATH. Install Dart Sass."
  exit 1
fi

mkdir -p "$(dirname "$OUTPUT")"

sass --watch "$INPUT":"$OUTPUT" --style=expanded --no-source-map

