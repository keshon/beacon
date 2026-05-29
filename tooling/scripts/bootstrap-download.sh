#!/usr/bin/env bash
set -eu

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

BS_VERSION="5.3.3"
VENDOR_DIR="$ROOT_DIR/uikit/vendor/bootstrap"

if [ -d "$VENDOR_DIR/scss" ]; then
  echo "Bootstrap $BS_VERSION SCSS already present in $VENDOR_DIR/scss"
  exit 0
fi

echo "Downloading Bootstrap $BS_VERSION SCSS source..."

mkdir -p "$VENDOR_DIR"
curl -sL "https://github.com/twbs/bootstrap/archive/refs/tags/v${BS_VERSION}.tar.gz" \
  | tar xz --strip-components=1 -C "$VENDOR_DIR" "bootstrap-${BS_VERSION}/scss"

echo "Done: $VENDOR_DIR/scss"
