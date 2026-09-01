import unittest

from flash_nvs import build_ota_url, validate_port


class FlashNvsTest(unittest.TestCase):
    def test_custom_standalone_port_is_kept_in_ota_url(self):
        self.assertEqual(
            build_ota_url("192.168.1.100", "12801"),
            "http://192.168.1.100:12801/xiaozhi/ota/",
        )

    def test_port_validation_accepts_tcp_range_only(self):
        self.assertTrue(validate_port("12800"))
        self.assertTrue(validate_port("65535"))
        self.assertFalse(validate_port("0"))
        self.assertFalse(validate_port("65536"))
        self.assertFalse(validate_port("not-a-port"))


if __name__ == "__main__":
    unittest.main()
