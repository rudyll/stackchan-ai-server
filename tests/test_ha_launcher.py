import json
import shlex
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


@unittest.skipUnless(shutil.which("jq"), "jq is required to exercise HA option parsing")
class HALauncherTest(unittest.TestCase):
    def read_gemini_flags(self, options):
        # Execute the real option-reading section, without writing /app or starting a server.
        launcher = (ROOT / "stackchan-server/run.sh").read_text()
        prefix = launcher.split("mkdir -p /app/manifest/config", 1)[0]
        with tempfile.TemporaryDirectory() as directory:
            options_file = Path(directory) / "options.json"
            options_file.write_text(json.dumps(options))
            prefix = prefix.replace("OPTIONS=/data/options.json", "OPTIONS=" + shlex.quote(str(options_file)))
            command = prefix + '\nprintf "%s\\n%s\\n" "$GEMINI_ENABLE_TOOLS" "${GEMINI_ENABLE_SEARCH:-missing}"\n'
            result = subprocess.run(["bash", "-c", command], check=True, capture_output=True, text=True)
            return result.stdout.splitlines()

    def test_disabled_tools_and_enabled_search_are_preserved(self):
        self.assertEqual(self.read_gemini_flags({"gemini_enable_tools": False, "gemini_enable_search": True}), ["false", "true"])

    def test_missing_flags_keep_ha_defaults(self):
        self.assertEqual(self.read_gemini_flags({}), ["true", "false"])

    def test_search_is_written_to_runtime_configuration(self):
        self.assertIn("gemini_enable_search: ${GEMINI_ENABLE_SEARCH}", (ROOT / "stackchan-server/run.sh").read_text())


if __name__ == "__main__":
    unittest.main()
