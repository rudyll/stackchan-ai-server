import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
ENV_EXAMPLE = ROOT / "stackchan-server" / ".env.standalone.example"
COMPOSE = ROOT / "stackchan-server" / "docker-compose.standalone.yml"
LAUNCHER = ROOT / "stackchan-server" / "standalone.sh"


class StandaloneConfigTest(unittest.TestCase):
    def test_env_example_lists_all_provider_and_host_port_inputs(self):
        content = ENV_EXAMPLE.read_text()
        for key in (
            "STACKCHAN_AI_PROVIDER",
            "STACKCHAN_OPENAI_API_KEY",
            "STACKCHAN_GEMINI_API_KEY",
            "STACKCHAN_TOKENHUB_BASE_URL",
            "STACKCHAN_TOKENHUB_API_KEY",
            "STACKCHAN_OPENROUTER_API_KEY",
            "STACKCHAN_COMPATIBLE_BASE_URL",
            "STACKCHAN_COMPATIBLE_API_KEY",
            "STACKCHAN_STT_BASE_URL",
            "STACKCHAN_LLM_BASE_URL",
            "STACKCHAN_TTS_BASE_URL",
            "STACKCHAN_WS_PORT",
            "STACKCHAN_SETTINGS_PORT",
            "STACKCHAN_SETTINGS_TOKEN",
        ):
            self.assertIn(key, content)

    def test_provider_inputs_reach_compose_and_launcher(self):
        compose = COMPOSE.read_text()
        launcher = LAUNCHER.read_text()
        for key in (
            "STACKCHAN_AI_PROVIDER",
            "STACKCHAN_OPENAI_API_KEY",
            "STACKCHAN_GEMINI_API_KEY",
            "STACKCHAN_TOKENHUB_BASE_URL",
            "STACKCHAN_TOKENHUB_API_KEY",
            "STACKCHAN_OPENROUTER_API_KEY",
            "STACKCHAN_COMPATIBLE_BASE_URL",
            "STACKCHAN_COMPATIBLE_API_KEY",
            "STACKCHAN_STT_BASE_URL",
            "STACKCHAN_LLM_BASE_URL",
            "STACKCHAN_TTS_BASE_URL",
        ):
            self.assertIn(key, compose)
            self.assertIn(key, launcher)


if __name__ == "__main__":
    unittest.main()
