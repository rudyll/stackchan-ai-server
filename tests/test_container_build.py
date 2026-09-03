import re
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


class ContainerBuildTest(unittest.TestCase):
    def test_cgo_builder_and_runtime_use_same_alpine(self):
        dockerfile = (ROOT / "stackchan-server/Dockerfile").read_text()
        version = re.search(r"ARG BUILD_FROM=alpine:([\d.]+)", dockerfile).group(1)
        self.assertIn("-alpine" + version + " AS builder", dockerfile)
        self.assertIn("FROM ${BUILD_FROM}", dockerfile)
        bases = re.findall(r'"alpine:([\d.]+)"', (ROOT / "stackchan-server/build.yaml").read_text())
        self.assertEqual(bases, [version] * 4)
        self.assertNotIn("go mod tidy", dockerfile)
        self.assertIn("-mod=readonly", dockerfile)

    def test_build_context_is_allowlisted(self):
        rules = (ROOT / "stackchan-server/.dockerignore").read_text().splitlines()
        self.assertEqual([r for r in rules if r.startswith("!")],
                         ["!Dockerfile", "!run.sh", "!standalone.sh", "!collect-licenses.sh",
                          "!LICENSE", "!NOTICE.md", "!licenses/", "!licenses/**", "!server/", "!server/**"])
        self.assertIn("*", rules)
        self.assertIn("**/.env*", rules)
        self.assertIn("**/logs/**", rules)
        self.assertIn("server/manifest/**", rules)

    def test_addon_release_notes_match_version_and_root_changelog(self):
        version = re.search(r'^version: "([^"]+)"',
                            (ROOT / "stackchan-server/config.yaml").read_text(), re.M).group(1)
        def release_notes(path):
            sections = (ROOT / path).read_text().split("\n## ")[1:]
            return next(section.strip() for section in sections if section.startswith(version + " (Beta)"))
        root_notes = release_notes("CHANGELOG.md")
        addon_notes = release_notes("stackchan-server/CHANGELOG.md")
        self.assertTrue(root_notes.startswith(version + " (Beta)"))
        self.assertEqual(addon_notes, root_notes)
        for readme in ("README.md", "README.zh.md", "stackchan-server/README.md"):
            self.assertIn("/releases/tag/v" + version, (ROOT / readme).read_text())


if __name__ == "__main__":
    unittest.main()
