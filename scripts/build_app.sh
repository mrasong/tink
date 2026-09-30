#!/usr/bin/env bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$DIR/client"

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
BUILD_NUMBER_OVERRIDE="${2:-}"
BUILD_ID_OVERRIDE="${3:-}"
BUILD_ID="${BUILD_ID_OVERRIDE:-$(git -C "$DIR" rev-parse --short=7 HEAD)}"
APP_ICON="$DIR/client/Sources/Tink/Resources/Tink.icns"

if [ ! -f "$APP_ICON" ]; then
    echo "✗ App icon not found: $APP_ICON" >&2
    exit 1
fi

# Build number: (year - 2025) + day of year (3 digits) + hour (2 digits).
# For example, 2026-01-23 18:xx produces 102318.
YEAR="$(date +%Y)"
DAY_OF_YEAR="$(date +%j)"
HOUR="$(date +%H)"
YEAR_NUMBER=$((10#$YEAR - 2025))
BUILD_NUMBER="${BUILD_NUMBER_OVERRIDE:-${YEAR_NUMBER}${DAY_OF_YEAR}${HOUR}}"

echo "==> Version: $VERSION, build: $BUILD_NUMBER, commit: $BUILD_ID"

# macOS 26+ 新视觉（liquid glass）以"二进制是否由 26+ SDK 链接"为门控。
# SwiftBuild 会把 LC_BUILD_VERSION 的 sdk 字段记成 deployment target（15.0），
# 故用 Apple 自带的 vtool 将 sdk 改写为 26.0；minos 仍为 15.0，兼容面不变。
# 必须在 codesign 之前执行（改写会使旧签名失效）。
MIN_OS_VERSION="15.0"
GATE_SDK_VERSION="26.0"
retag_sdk() {
    echo "==> Retagging SDK $(basename "$1") to $GATE_SDK_VERSION (minos $MIN_OS_VERSION)..."
    vtool -set-build-version macos "$MIN_OS_VERSION" "$GATE_SDK_VERSION" -replace -output "$1" "$1"
}

echo "==> Building Tink universal executable (arm64 + x86_64)..."
# 清理 Swift 的旧构建缓存，避免项目目录改名后 ModuleCache 仍引用旧路径
rm -rf "$DIR/client/.build" 2> /dev/null || true
swift build -c release --arch arm64
ARM64_BIN="$DIR/client/.build/Tink-arm64"
ARM64_BIN_PATH="$(swift build -c release --arch arm64 --show-bin-path)/Tink"
RESOURCE_BUNDLE="$(dirname "$ARM64_BIN_PATH")/Tink_Tink.bundle"
cp "$ARM64_BIN_PATH" "$ARM64_BIN"
retag_sdk "$ARM64_BIN"
swift build -c release --arch x86_64
X86_64_BIN="$DIR/client/.build/Tink-x86_64"
X86_64_BIN_PATH="$(swift build -c release --arch x86_64 --show-bin-path)/Tink"
cp "$X86_64_BIN_PATH" "$X86_64_BIN"
retag_sdk "$X86_64_BIN"

create_app() {
    local arch_name="$1"
    local binary_path="$2"
    local app_dir="$DIR/build/$arch_name/Tink.app"
    local contents_dir="$app_dir/Contents"
    local macos_dir="$contents_dir/MacOS"
    local resources_dir="$contents_dir/Resources"

    echo "==> Creating $arch_name App Bundle at $app_dir..."
    rm -rf "$app_dir"
    mkdir -p "$macos_dir" "$resources_dir"

    cp "$binary_path" "$macos_dir/Tink"
    chmod +x "$macos_dir/Tink"

    # SwiftPM >= 6.x 的资源 bundle 为 bundle/Contents/Resources 结构，
    # 需展平拷贝，否则产物会嵌套为 Resources/Contents/Resources/
    if [ -d "$RESOURCE_BUNDLE/Contents/Resources" ]; then
        cp -R "$RESOURCE_BUNDLE/Contents/Resources/." "$resources_dir/"
    else
        cp -R "$RESOURCE_BUNDLE/." "$resources_dir/"
    fi
    cp "$APP_ICON" "$resources_dir/Tink.icns"

    cat << PLIST > "$contents_dir/Info.plist"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>Tink</string>
    <key>CFBundleIdentifier</key>
    <string>com.mrasong.tink</string>
    <key>CFBundleDevelopmentRegion</key>
    <string>en</string>
    <key>CFBundleLocalizations</key>
    <array>
        <string>en</string>
        <string>zh-Hans</string>
    </array>
    <key>CFBundleName</key>
    <string>Tink</string>
    <key>CFBundleIconFile</key>
    <string>Tink</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleShortVersionString</key>
    <string>$VERSION</string>
    <key>CFBundleVersion</key>
    <string>$BUILD_NUMBER</string>
    <key>TinkGitCommit</key>
    <string>$BUILD_ID</string>
    <key>LSMinimumSystemVersion</key>
    <string>15.0</string>
    <key>LSUIElement</key>
    <true/>
    <key>NSHighResolutionCapable</key>
    <true/>
</dict>
</plist>
PLIST

    echo "==> Ad-hoc signing $arch_name App Bundle..."
    codesign --force --deep --sign - "$app_dir"
    echo "✓ $arch_name app created: $app_dir"
    echo "  Architectures: $(lipo -archs "$macos_dir/Tink")"
}

rm -rf "$DIR/build/arm64" "$DIR/build/x86_64" "$DIR/build/universal"

lipo -create "$ARM64_BIN" "$X86_64_BIN" -output "$DIR/client/.build/release/Tink-universal"
create_app "arm64" "$ARM64_BIN"
create_app "x86_64" "$X86_64_BIN"
create_app "universal" "$DIR/client/.build/release/Tink-universal"

echo ""
echo "✓ All macOS app variants created under $DIR/build"
