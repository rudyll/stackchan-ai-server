#!/bin/bash
set -eu

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
SERVER_DIR=$(cd "$SCRIPT_DIR/.." && pwd)
OUTPUT_DIR="${OUTPUT_DIR:-$SERVER_DIR/dist}"
VERSION=$(tr -d '\r\n' < "$SCRIPT_DIR/VERSION")
APP_NAME="StackChan AI Server"
ARCH="${STACKCHAN_MACOS_ARCH:-universal}"
case "$ARCH" in
	arm64|amd64) ARCHES="$ARCH" ;;
	universal) ARCHES="arm64 amd64" ;;
	*) echo "ERROR: STACKCHAN_MACOS_ARCH must be arm64, amd64 or universal" >&2; exit 1 ;;
esac
for tool in go cmake clang pkg-config hdiutil sips iconutil codesign lipo; do
	command -v "$tool" >/dev/null 2>&1 || { echo "ERROR: $tool is required" >&2; exit 1; }
done
mkdir -p "$OUTPUT_DIR"
OUTPUT_DIR=$(cd "$OUTPUT_DIR" && pwd)
RELEASE_DIR="$OUTPUT_DIR/macos-$VERSION-$ARCH"
DMG_NAME="StackChan-AI-Server-$VERSION-macos-$ARCH.dmg"
DMG_PATH="$RELEASE_DIR/$DMG_NAME"
[ ! -e "$RELEASE_DIR" ] || { echo "ERROR: output already exists: $RELEASE_DIR (choose a new OUTPUT_DIR)" >&2; exit 1; }
mkdir "$RELEASE_DIR"
BUILD_DIR=$(mktemp -d "$OUTPUT_DIR/.macos-build.XXXXXX")
APP_DIR="$RELEASE_DIR/$APP_NAME.app"
mkdir -p "$APP_DIR/Contents/MacOS" "$APP_DIR/Contents/Resources/Licenses/licenses"
sed "s/__VERSION__/$VERSION/g" "$SCRIPT_DIR/Info.plist" > "$APP_DIR/Contents/Info.plist"
cp "$SCRIPT_DIR/config.yaml" "$APP_DIR/Contents/Resources/config.yaml"
cp "$SCRIPT_DIR/app-launcher.sh" "$APP_DIR/Contents/MacOS/stackchan-server"
cp "$SERVER_DIR/LICENSE" "$APP_DIR/Contents/Resources/Licenses/LICENSE"
cp "$SERVER_DIR/NOTICE.md" "$APP_DIR/Contents/Resources/Licenses/NOTICE.md"
cp "$SERVER_DIR/licenses/"*.txt "$APP_DIR/Contents/Resources/Licenses/licenses/"
cp "$SCRIPT_DIR/NOTICE.txt" "$APP_DIR/Contents/Resources/Licenses/NOTICE.txt"
cp "$SCRIPT_DIR/INSTALL.txt" "$APP_DIR/Contents/Resources/INSTALL.txt"
printf 'Version: %s\nSource revision: %s\nArchitectures: %s\nMinimum macOS target: 12.0\n' \
	"$VERSION" "${STACKCHAN_BUILD_REVISION:-local-build}" "$ARCHES" \
	> "$APP_DIR/Contents/Resources/BUILD-INFO.txt"
chmod 755 "$APP_DIR/Contents/MacOS/stackchan-server"

ICON="$SERVER_DIR/server/internal/service/ai/assets/stackchan-icon.png"
ICONSET="$BUILD_DIR/StackChan.iconset"
mkdir "$ICONSET"
for size in 16 32 128 256 512; do
	sips -z "$size" "$size" "$ICON" --out "$ICONSET/icon_${size}x${size}.png" >/dev/null
	double=$((size * 2))
	sips -z "$double" "$double" "$ICON" --out "$ICONSET/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$ICONSET" -o "$APP_DIR/Contents/Resources/StackChan.icns"

for build_arch in $ARCHES; do
	OPUS_PREFIX=$(bash "$SCRIPT_DIR/prepare-opus.sh" "$build_arch")
	cp "$OPUS_PREFIX/COPYING" "$APP_DIR/Contents/Resources/Licenses/Opus.txt"
	# Only the static codec is in this prefix. No Homebrew library is linked.
	env CGO_ENABLED=1 GOOS=darwin GOARCH="$build_arch" CC=clang \
		MACOSX_DEPLOYMENT_TARGET=12.0 \
		CGO_CFLAGS="-O2 -g -mmacosx-version-min=12.0" \
		CGO_CXXFLAGS="-O2 -g -mmacosx-version-min=12.0" \
		CGO_LDFLAGS="-O2 -g -mmacosx-version-min=12.0 -Wl,-fatal_warnings" \
		PKG_CONFIG_PATH="$OPUS_PREFIX/lib/pkgconfig" PKG_CONFIG_LIBDIR="$OPUS_PREFIX/lib/pkgconfig" \
		go -C "$SERVER_DIR/server" build -mod=readonly -tags=nolibopusfile \
		-trimpath -ldflags="-s -w" -o "$BUILD_DIR/server-$build_arch" .
done
if [ "$ARCH" = universal ]; then
	lipo -create "$BUILD_DIR/server-arm64" "$BUILD_DIR/server-amd64" -output "$APP_DIR/Contents/Resources/stackchan-server"
else
	cp "$BUILD_DIR/server-$ARCH" "$APP_DIR/Contents/Resources/stackchan-server"
fi

# Include standard-library and compiled-module license files in the app.
GO_ROOT=$(go env GOROOT)
GO_LICENSE="$GO_ROOT/LICENSE"
[ -f "$GO_LICENSE" ] || GO_LICENSE="$(dirname "$GO_ROOT")/LICENSE"
cp "$GO_LICENSE" "$APP_DIR/Contents/Resources/Licenses/Go.txt"
go -C "$SERVER_DIR/server" list -mod=readonly -deps -tags=nolibopusfile \
	-f '{{if .Module}}{{.Module.Path}}|{{.Module.Dir}}{{end}}' . | sort -u > "$BUILD_DIR/module-dirs"
while IFS='|' read -r module_path module_dir; do
	[ -n "$module_dir" ] || continue
	module_name=$(printf '%s' "$module_path" | tr '/:' '__')
	find "$module_dir" -maxdepth 1 -type f \( -iname 'license*' -o -iname 'copying*' -o -iname 'notice*' \) -print | while IFS= read -r license_file; do
		cp "$license_file" "$APP_DIR/Contents/Resources/Licenses/$module_name-$(basename "$license_file")"
	done
done < "$BUILD_DIR/module-dirs"

if [ -n "${CODESIGN_IDENTITY:-}" ]; then
	codesign --force --options runtime --timestamp --sign "$CODESIGN_IDENTITY" "$APP_DIR/Contents/Resources/stackchan-server"
	codesign --force --options runtime --timestamp --sign "$CODESIGN_IDENTITY" "$APP_DIR"
else
	# Ad-hoc sealing is not Developer ID signing or Apple notarization.
	codesign --force --sign - "$APP_DIR/Contents/Resources/stackchan-server"
	codesign --force --sign - "$APP_DIR"
	echo "INFO: ad-hoc signed preview; no Developer ID or Apple notarization"
fi
codesign --verify --deep --strict "$APP_DIR"
otool -L "$APP_DIR/Contents/Resources/stackchan-server"
if otool -L "$APP_DIR/Contents/Resources/stackchan-server" | grep -Eq '/opt/homebrew/|/usr/local/|libopus'; then
	echo "ERROR: unexpected external library dependency" >&2; exit 1
fi
mkdir "$BUILD_DIR/dmg-root"
ditto "$APP_DIR" "$BUILD_DIR/dmg-root/$APP_NAME.app"
cp "$SCRIPT_DIR/INSTALL.txt" "$BUILD_DIR/dmg-root/READ-ME-FIRST.txt"
ln -s /Applications "$BUILD_DIR/dmg-root/Applications"
hdiutil create -volname "$APP_NAME" -srcfolder "$BUILD_DIR/dmg-root" -format UDZO "$DMG_PATH" >/dev/null
(
	cd "$RELEASE_DIR"
	shasum -a 256 "$DMG_NAME" > "$DMG_NAME.sha256"
)
echo "Created: $DMG_PATH"
echo "App: $APP_DIR"
echo "Build intermediates: $BUILD_DIR"
