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
            "STACKCHAN_DEVICE_PROFILES",
            "STACKCHAN_SYSTEM_PROMPT",
            "STACKCHAN_AUDIO_PREBUFFER_MS",
            "STACKCHAN_AUDIO_PREBUFFER_MAX_WAIT_MS",
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

    def test_host_ports_are_separate_from_container_ports(self):
        compose = COMPOSE.read_text()
        launcher = LAUNCHER.read_text()
        self.assertIn('"${STACKCHAN_WS_PORT:-12800}:12800"', compose)
        self.assertIn('"127.0.0.1:${STACKCHAN_SETTINGS_PORT:-8099}:8099"', compose)
        self.assertIn('local_port: ${WS_PORT}', launcher)
        self.assertIn('settings_listen_address: ":8099"', launcher)
        self.assertIn('ha_enabled: false', launcher)
        self.assertIn('ota_https_enabled: false', launcher)
        command = (ROOT / "stackchan-server/server/internal/cmd/cmd.go").read_text()
        self.assertIn('s.SetAddr(g.Cfg().MustGet(ctx, "server.address", ":12800").String())', command)
        self.assertNotIn('s.SetPort(', command)


if __name__ == "__main__":
    unittest.main()
