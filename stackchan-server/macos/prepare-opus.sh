#!/bin/bash
set -eu

# Homebrew bottles may require the builder's newest macOS. Build a pinned,
# static codec for our deployment target instead of shipping that dependency.
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
DEPS_DIR="${STACKCHAN_MACOS_DEPS_DIR:-$SCRIPT_DIR/../dist/macos-deps}"
ARCH="${1:?usage: prepare-opus.sh arm64|amd64}"
case "$ARCH" in
	arm64) CLANG_ARCH=arm64 ;;
	amd64) CLANG_ARCH=x86_64 ;;
	*) echo "ERROR: unsupported architecture: $ARCH" >&2; exit 1 ;;
esac
VERSION=1.6.1
SHA256=6ffcb593207be92584df15b32466ed64bbec99109f007c82205f0194572411a1
mkdir -p "$DEPS_DIR"
DEPS_DIR=$(cd "$DEPS_DIR" && pwd)
ARCHIVE="$DEPS_DIR/opus-$VERSION.tar.gz"
if [ ! -f "$ARCHIVE" ]; then
	curl -fL --retry 3 "https://downloads.xiph.org/releases/opus/opus-$VERSION.tar.gz" -o "$ARCHIVE.part"
	mv "$ARCHIVE.part" "$ARCHIVE"
fi
ACTUAL_SHA=$(shasum -a 256 "$ARCHIVE" | awk '{print $1}')
[ "$ACTUAL_SHA" = "$SHA256" ] || { echo "ERROR: OPUS source checksum mismatch" >&2; exit 1; }
[ -d "$DEPS_DIR/opus-$VERSION" ] || tar -xzf "$ARCHIVE" -C "$DEPS_DIR"
PREFIX="$DEPS_DIR/opus-$VERSION-macos12-$ARCH"
if [ ! -f "$PREFIX/.complete-trimpath" ]; then
	cmake -S "$DEPS_DIR/opus-$VERSION" -B "$PREFIX-build" \
		-DCMAKE_BUILD_TYPE=Release -DCMAKE_OSX_ARCHITECTURES="$CLANG_ARCH" \
		-DCMAKE_OSX_DEPLOYMENT_TARGET=12.0 -DCMAKE_INSTALL_PREFIX="$PREFIX" \
		"-DCMAKE_C_FLAGS=-ffile-prefix-map=\"$DEPS_DIR\"=." \
		-DCMAKE_INSTALL_LIBDIR=lib -DBUILD_SHARED_LIBS=OFF \
		-DOPUS_BUILD_SHARED_LIBRARY=OFF -DOPUS_BUILD_TESTING=OFF \
		-DOPUS_BUILD_PROGRAMS=OFF >&2
	cmake --build "$PREFIX-build" --parallel 4 >&2
	cmake --install "$PREFIX-build" >&2
	cp "$DEPS_DIR/opus-$VERSION/COPYING" "$PREFIX/COPYING"
	touch "$PREFIX/.complete-trimpath"
fi
# CMake's opus.pc embeds unquoted absolute paths; quote them for checkouts with
# spaces so pkg-config and Go's cgo flag parser see each path as one argument.
printf 'Name: Opus\nDescription: Opus audio codec\nVersion: %s\nLibs: -L"%s/lib" -lopus\nLibs.private: -lm\nCflags: -I"%s/include/opus"\n' \
	"$VERSION" "$PREFIX" "$PREFIX" > "$PREFIX/lib/pkgconfig/opus.pc"
printf '%s\n' "$PREFIX"
