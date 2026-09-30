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
IMAGE="${2:-tink:latest}"
PLATFORM="${3:-}"
BUILD_ID="${4:-$(git -C "$DIR" rev-parse --short=7 HEAD)}"

PLATFORM_ARGS=()
if [ -n "$PLATFORM" ]; then
  PLATFORM_ARGS+=(--platform "$PLATFORM")
fi

echo "==> Building Docker image $IMAGE (version $VERSION, build $BUILD_ID)..."
docker buildx build \
  "${PLATFORM_ARGS[@]}" \
  --build-arg "VERSION=$VERSION" \
  --build-arg "BUILD_ID=$BUILD_ID" \
  --load \
  -t "$IMAGE" \
  "$DIR"

echo "✓ Docker image created: $IMAGE"
