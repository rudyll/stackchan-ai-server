import hashlib
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[1]
AGPL_SHA256 = "0d96a4ff68ad6d4b6f1f30f713b18d5184912ba8dd389f86aa7710db079abcb0"
PAYPAL_URL = "https://paypal.me/unitekno"


class LicensingTest(unittest.TestCase):
    def test_full_license_is_identical_to_official_gnu_text(self):
        for path in ("LICENSE", "stackchan-server/LICENSE"):
            data = (ROOT / path).read_bytes()
            self.assertEqual(hashlib.sha256(data).hexdigest(), AGPL_SHA256)

    def test_original_mit_and_copyright_are_retained(self):
        for filename, holder in (("MIT-M5Stack.txt", "M5Stack Technology CO LTD"),
                                 ("MIT-legacy-project.txt", "rudyll")):
            text = (ROOT / "stackchan-server/licenses" / filename).read_text()
            self.assertIn(holder, text)
            self.assertIn("Permission is hereby granted, free of charge", text)
            self.assertIn("THE SOFTWARE IS PROVIDED", text)
        notice = (ROOT / "stackchan-server/NOTICE.md").read_text()
        for preserved in ("v2.8.0-beta.3", "macos-v0.1.1", "AGPL-3.0-only", "MIT"):
            self.assertIn(preserved, notice)

    def test_current_readmes_and_contribution_policy_are_consistent(self):
        for path in ("README.md", "README.zh.md", "stackchan-server/README.md"):
            text = (ROOT / path).read_text()
            self.assertIn("AGPL-3.0-only", text)
            self.assertIn("SPONSORING.md", text)
            self.assertNotIn("License-MIT-yellow.svg", text)
        self.assertIn("You retain copyright", (ROOT / "CONTRIBUTING.md").read_text())

    def test_sponsorship_uses_only_the_confirmed_payment_destination(self):
        text = (ROOT / "SPONSORING.md").read_text()
        self.assertIn("entirely voluntary", text)
        self.assertNotIn("No verified payment destination", text)
        self.assertIn("No cryptocurrency receiving address", text)
        funding = (ROOT / ".github/FUNDING.yml").read_text()
        self.assertEqual(funding.strip(), 'custom: ["' + PAYPAL_URL + '"]')
        for path in ("SPONSORING.md", "README.md", "README.zh.md", "stackchan-server/README.md"):
            with self.subTest(path=path):
                content = (ROOT / path).read_text()
                destinations = set(re.findall(r'https://paypal\.me/[^\s)"<]+', content))
                self.assertEqual(destinations, {PAYPAL_URL})
                self.assertNotIn("rudy219", content)
                self.assertNotIn("paypal-qr", content)

    def test_sponsorship_is_visible_at_the_top_of_readmes(self):
        for path in ("README.md", "README.zh.md", "stackchan-server/README.md"):
            with self.subTest(path=path):
                header = (ROOT / path).read_text().split("\n## ", 1)[0]
                self.assertIn("Sponsor-PayPal", header)
                self.assertIn("](" + PAYPAL_URL + ")", header)

    def test_packages_preserve_project_and_dependency_licenses(self):
        docker = (ROOT / "stackchan-server/Dockerfile").read_text()
        macos = (ROOT / "stackchan-server/macos/build-dmg.sh").read_text()
        self.assertIn("COPY LICENSE NOTICE.md", docker)
        self.assertIn("COPY licenses/", docker)
        self.assertIn("/usr/share/licenses/stackchan/licenses/", docker)
        self.assertIn("sh /collect-licenses.sh", docker)
        self.assertIn("/usr/share/licenses/stackchan/dependencies/", docker)
        self.assertIn('"$SERVER_DIR/NOTICE.md"', macos)
        self.assertIn('"$SERVER_DIR/licenses/"*.txt', macos)
        self.assertIn('"$APP_DIR/Contents/Resources/Licenses/licenses/"', macos)
        self.assertIn('"$APP_DIR/Contents/Resources/Licenses/LICENSE"', macos)
        self.assertIn('"$APP_DIR/Contents/Resources/Licenses/NOTICE.md"', macos)


if __name__ == "__main__":
    unittest.main()
