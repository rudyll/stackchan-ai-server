import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location(
    "smoke_containers", Path(__file__).resolve().parents[1] / "scripts/smoke-containers.py")
smoke = importlib.util.module_from_spec(spec)
spec.loader.exec_module(smoke)


class ContainerSmokeTest(unittest.TestCase):
    def test_restart_rechecks_published_port_instead_of_using_stale_port(self):
        with patch.object(smoke, "run", side_effect=["127.0.0.1:32768", "test-container", "127.0.0.1:32770"]) as run:
            with patch.object(smoke, "check_persistence") as check:
                smoke.restart_ha_and_check("test-container", "")
        self.assertEqual(run.call_args_list[1].args, ("docker", "restart", "test-container"))
        check.assert_called_once_with("http://127.0.0.1:32770", "")


if __name__ == "__main__":
    unittest.main()
