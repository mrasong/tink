#!/usr/bin/env bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

VERSION_FILE="$DIR/VERSION"
if [ ! -f "$VERSION_FILE" ]; then
  echo "✗ Version file not found: $VERSION_FILE" >&2
  exit 1
fi
DEFAULT_VERSION="$(tr -d '[:space:]' < "$VERSION_FILE")"
if [ -z "$DEFAULT_VERSION" ]; then
  echo "✗ Version file is empty: $VERSION_FILE" >&2
  exit 1
fi

VERSION="${1:-$DEFAULT_VERSION}"
BUILD_ID_OVERRIDE="${2:-}"

BUILD_ID="${BUILD_ID_OVERRIDE:-$(git -C "$DIR" rev-parse --short=7 HEAD)}"

echo "==> Version: $VERSION, build: $BUILD_ID"

echo "==> Building Tink Web Dashboard (Svelte)..."
cd "$DIR/server/web"
if [ ! -d "node_modules" ]; then
  pnpm install
fi
VITE_APP_VERSION="$VERSION" VITE_APP_BUILD="$BUILD_ID" pnpm build

echo "==> Building Tink server executable (embedding Web Dashboard)..."
mkdir -p "$DIR/build"
cd "$DIR/server"
go build -buildvcs=false \
  -ldflags="-w -s -X main.Version=$VERSION -X main.Build=$BUILD_ID -X github.com/mrasong/tink/server/internal/api.Version=$VERSION -X github.com/mrasong/tink/server/internal/api.Build=$BUILD_ID" \
  -o "$DIR/build/tink-server" ./cmd/server

echo "✓ Server binary created successfully at $DIR/build/tink-server"
