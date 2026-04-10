#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MAIN_PKG="$SCRIPT_DIR/cmd/beacon"
OUTPUT="$SCRIPT_DIR/beacon"

if [ ! -f "$MAIN_PKG/main.go" ]; then
    echo "ERROR: main.go not found in $MAIN_PKG"
    exit 1
fi

echo "[1/3] Gathering build info..."

BUILD_DATE="$(date -u +"%Y-%m-%dT%H-%M-%SZ")"

GIT_COMMIT="$(git -C "$SCRIPT_DIR" rev-parse --short HEAD 2>/dev/null || true)"
if [ -z "$GIT_COMMIT" ]; then
    GIT_COMMIT="none"
fi

GIT_TAG="$(git -C "$SCRIPT_DIR" describe --tags --abbrev=0 2>/dev/null || true)"
if [ -z "$GIT_TAG" ]; then
    GIT_TAG="dev"
fi

GIT_DIRTY=""
if ! git -C "$SCRIPT_DIR" diff --quiet 2>/dev/null; then
    GIT_DIRTY="-dirty"
fi

VERSION="${GIT_TAG}${GIT_DIRTY}"

echo "[2/3] Building..."
echo "Version: $VERSION"
echo "Commit:  $GIT_COMMIT"
echo "Date:    $BUILD_DATE"

go build -o "$OUTPUT" -ldflags "\
-X=github.com/keshon/buildinfo.Version=$VERSION \
-X=github.com/keshon/buildinfo.Commit=$GIT_COMMIT \
-X=github.com/keshon/buildinfo.BuildTime=$BUILD_DATE \
-X=github.com/keshon/buildinfo.Project=Beacon \
-X=github.com/keshon/buildinfo.Description=Beacon-uptime-monitoring-service-in-Go \
" "$MAIN_PKG"

echo "[3/3] Running $OUTPUT..."
"$OUTPUT"
EXIT_CODE=$?

echo
echo "Process exited with code $EXIT_CODE."
read -n 1 -s -r -p "Press any key to close..."
echo

exit $EXIT_CODE