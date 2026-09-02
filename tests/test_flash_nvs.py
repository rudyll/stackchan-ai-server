import unittest
from unittest.mock import patch

from flash_nvs import build_ota_url, main, validate_port, validate_server_host


class FlashNvsTest(unittest.TestCase):
    def test_declining_overwrite_warning_never_accesses_flash(self):
        for lang, warning in (("en", "entire NVS partition"), ("zh", "整个 NVS 分区")):
            with self.subTest(lang=lang), patch.dict("os.environ", {"IDF_PATH": "/test-idf"}), \
                    patch("flash_nvs.choose_language", return_value=lang), \
                    patch("flash_nvs.find_idf_tool", return_value="test-tool.py"), \
                    patch("flash_nvs.list_serial_ports", return_value=["test-serial"]), \
                    patch("builtins.input", side_effect=["1", "stackchan.local", "12801", "n"]) as prompt, \
                    patch("builtins.print"), patch("flash_nvs.run") as run:
                with self.assertRaises(SystemExit) as exit_result:
                    main()
                self.assertEqual(exit_result.exception.code, 0)
                self.assertIn(warning, prompt.call_args.args[0])
                self.assertIn("http://stackchan.local:12801/xiaozhi/ota/", prompt.call_args.args[0])
                run.assert_not_called()

    def test_custom_standalone_port_is_kept_in_ota_url(self):
        self.assertEqual(
            build_ota_url("192.168.1.100", "12801"),
            "http://192.168.1.100:12801/xiaozhi/ota/",
        )

    def test_local_hostname_is_allowed_for_ota_url(self):
        self.assertTrue(validate_server_host("stackchan.local"))
        self.assertTrue(validate_server_host("stackchan-01.lan"))
        self.assertEqual(
            build_ota_url("stackchan.local", "12800"),
            "http://stackchan.local:12800/xiaozhi/ota/",
        )

    def test_server_host_rejects_url_injection_and_ipv6(self):
        for value in ("https://stackchan.local", "stackchan.local/path", "bad_host", "[::1]"):
            self.assertFalse(validate_server_host(value))

    def test_port_validation_accepts_tcp_range_only(self):
        self.assertTrue(validate_port("12800"))
        self.assertTrue(validate_port("65535"))
        self.assertFalse(validate_port("0"))
        self.assertFalse(validate_port("65536"))
        self.assertFalse(validate_port("not-a-port"))


if __name__ == "__main__":
    unittest.main()
