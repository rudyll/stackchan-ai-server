#!/bin/bash
set -eu

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
SERVER_DIR=$(cd "$SCRIPT_DIR/.." && pwd)
OUTPUT_DIR="${OUTPUT_DIR:-$SERVER_DIR/dist}"
APP_NAME="StackChan AI Server"
APP_DIR="$OUTPUT_DIR/$APP_NAME.app"
DMG_PATH="$OUTPUT_DIR/$APP_NAME.dmg"
ARCH="${STACKCHAN_MACOS_ARCH:-$(uname -m)}"

case "$ARCH" in
	arm64|amd64) ;;
	*) echo "ERROR: STACKCHAN_MACOS_ARCH must be arm64 or amd64" >&2; exit 1 ;;
esac

command -v go >/dev/null 2>&1 || { echo "ERROR: Go is required" >&2; exit 1; }
command -v pkg-config >/dev/null 2>&1 || { echo "ERROR: pkg-config is required" >&2; exit 1; }
command -v hdiutil >/dev/null 2>&1 || { echo "ERROR: hdiutil is required on macOS" >&2; exit 1; }
command -v install_name_tool >/dev/null 2>&1 || { echo "ERROR: install_name_tool is required on macOS" >&2; exit 1; }
pkg-config --exists opus || { echo "ERROR: libopus is required; install it with: brew install opus" >&2; exit 1; }

OPUS_LIB_DIR=$(pkg-config --variable=libdir opus)
OPUS_LIB="$OPUS_LIB_DIR/libopus.0.dylib"
[ -f "$OPUS_LIB" ] || { echo "ERROR: cannot find $OPUS_LIB" >&2; exit 1; }

rm -rf "$APP_DIR" "$DMG_PATH" "$OUTPUT_DIR/dmg-root"
mkdir -p "$APP_DIR/Contents/MacOS" "$APP_DIR/Contents/Resources"

cp "$SCRIPT_DIR/Info.plist" "$APP_DIR/Contents/Info.plist"
cp "$SCRIPT_DIR/config.yaml" "$APP_DIR/Contents/Resources/config.yaml"
cp "$OPUS_LIB" "$APP_DIR/Contents/Resources/libopus.0.dylib"
cp "$SCRIPT_DIR/app-launcher.sh" "$APP_DIR/Contents/MacOS/stackchan-server"
chmod 755 "$APP_DIR/Contents/MacOS/stackchan-server"

(
	cd "$SERVER_DIR/server"
	env CGO_ENABLED=1 GOOS=darwin GOARCH="$ARCH" go build -mod=readonly -tags=nolibopusfile -trimpath -ldflags="-s -w" -o "$APP_DIR/Contents/Resources/stackchan-server" .
)

install_name_tool -id "@rpath/libopus.0.dylib" "$APP_DIR/Contents/Resources/libopus.0.dylib"
install_name_tool -add_rpath "@loader_path" "$APP_DIR/Contents/Resources/stackchan-server"
OPUS_DEPS=$(otool -L "$APP_DIR/Contents/Resources/stackchan-server" | awk '/libopus/{print $1}')
[ -n "$OPUS_DEPS" ] || { echo "ERROR: built server does not link libopus" >&2; exit 1; }
while IFS= read -r OPUS_DEP; do
	[ "$OPUS_DEP" = "@rpath/libopus.0.dylib" ] || install_name_tool -change "$OPUS_DEP" "@rpath/libopus.0.dylib" "$APP_DIR/Contents/Resources/stackchan-server"
done <<EOF
$OPUS_DEPS
EOF

# Go emits a linker-signed binary. install_name_tool invalidates that
# signature, so re-sign the nested payload before the app is launched.
codesign --force --sign - "$APP_DIR/Contents/Resources/libopus.0.dylib"
codesign --force --sign - "$APP_DIR/Contents/Resources/stackchan-server"

if [ -n "${CODESIGN_IDENTITY:-}" ]; then
	codesign --force --deep --options runtime --timestamp --sign "$CODESIGN_IDENTITY" "$APP_DIR"
else
	echo "INFO: CODESIGN_IDENTITY not set; building unsigned development DMG"
fi

mkdir -p "$OUTPUT_DIR/dmg-root"
cp -R "$APP_DIR" "$OUTPUT_DIR/dmg-root/$APP_NAME.app"
ln -s /Applications "$OUTPUT_DIR/dmg-root/Applications"
hdiutil create -volname "$APP_NAME" -srcfolder "$OUTPUT_DIR/dmg-root" -ov -format UDZO "$DMG_PATH" >/dev/null

echo "Created: $DMG_PATH"
