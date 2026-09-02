import plistlib
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
MACOS = ROOT / "stackchan-server/macos"


class MacOSPackageTest(unittest.TestCase):
    def test_version_icon_and_download_names_are_aligned(self):
        version = (MACOS / "VERSION").read_text().strip()
        self.assertRegex(version, r"^\d+\.\d+\.\d+$")
        plist = plistlib.loads((MACOS / "Info.plist").read_text().replace("__VERSION__", version).encode())
        self.assertEqual(plist["CFBundleShortVersionString"], version)
        self.assertEqual(plist["CFBundleVersion"], version)
        self.assertEqual(plist["CFBundleIconFile"], "StackChan.icns")
        self.assertEqual(plist["LSMinimumSystemVersion"], "12.0")
        for readme in (ROOT / "README.md", ROOT / "README.zh.md", MACOS / "README.md"):
            self.assertIn(f"StackChan-AI-Server-{version}-macos-universal.dmg", readme.read_text())
            self.assertIn(f"macos-v{version}", readme.read_text())

    def test_distributable_uses_pinned_static_codec_and_preserves_existing_builds(self):
        builder = (MACOS / "build-dmg.sh").read_text()
        codec = (MACOS / "prepare-opus.sh").read_text()
        self.assertIn("lipo -create", builder)
        self.assertIn('ARCHES="arm64 amd64"', builder)
        self.assertIn("MACOSX_DEPLOYMENT_TARGET=12.0", builder)
        self.assertIn('CGO_CFLAGS="-O2 -g -mmacosx-version-min=12.0"', builder)
        self.assertIn("-Wl,-fatal_warnings", builder)
        self.assertIn("-DCMAKE_OSX_DEPLOYMENT_TARGET=12.0", codec)
        self.assertIn("-DBUILD_SHARED_LIBS=OFF", codec)
        self.assertIn('"$ACTUAL_SHA" = "$SHA256"', codec)
        self.assertIn("output already exists", builder)
        self.assertNotIn("rm -rf", builder)
        self.assertNotIn('cp "$OPUS_LIB"', builder)
        self.assertIn('cp "$SCRIPT_DIR/NOTICE.txt"', builder)


if __name__ == "__main__":
    unittest.main()
