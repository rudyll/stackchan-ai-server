#!/usr/bin/env python3
"""
flash_nvs.py — StackChan NVS OTA-URL injector
Guides you through writing the local server address into the device NVS partition.
Requires ESP-IDF environment to be activated (idf.py / parttool.py must be on PATH).

Usage:  python3 flash_nvs.py
"""

import os
import sys
import csv
import glob
import shutil
import struct
import tempfile
import subprocess

# ── language ────────────────────────────────────────────────────────────────

def choose_language():
    print("\nLanguage / 语言")
    print("  1. English")
    print("  2. 中文")
    while True:
        c = input("Choose / 选择 [1/2]: ").strip()
        if c in ("1", ""):
            return "en"
        if c == "2":
            return "zh"

STRINGS = {
    "en": {
        "title":         "StackChan NVS Injector",
        "idf_missing":   "ERROR: IDF_PATH is not set. Please activate your ESP-IDF environment first.\n"
                         "  macOS/Linux:  . $HOME/esp/esp-idf/export.sh\n"
                         "  Windows PS:   C:\\esp\\v6.0.1\\esp-idf\\export.ps1",
        "parttool_miss": "ERROR: parttool.py not found in ESP-IDF. Check your IDF_PATH.",
        "nvsgen_miss":   "ERROR: nvs_partition_gen.py not found in ESP-IDF. Check your IDF_PATH.",
        "ports_none":    "No serial ports found. Connect the device and try again.",
        "port_prompt":   "Select port number",
        "port_manual":   "Enter port path manually",
        "ip_prompt":     "Enter your Home Assistant LAN IP (e.g. 192.168.1.100)",
        "ip_bad":        "That doesn't look like a valid IP. Try again.",
        "confirm":       "Ready to write NVS to {port} with ota_url=http://{ip}:12800/xiaozhi/ota/\nProceed? [y/N]",
        "querying":      "Querying NVS partition size...",
        "size_found":    "NVS partition size: {size}",
        "generating":    "Generating NVS binary...",
        "writing":       "Writing NVS to device...",
        "done":          "Done! Power-cycle the device. It should now connect to your local StackChan server.",
        "ota_note":      "NOTE: After any firmware OTA upgrade, run this script again to re-inject the NVS key.",
        "aborted":       "Aborted.",
        "error":         "ERROR: {msg}",
    },
    "zh": {
        "title":         "StackChan NVS 写入工具",
        "idf_missing":   "错误：未设置 IDF_PATH，请先激活 ESP-IDF 环境。\n"
                         "  macOS/Linux:  . $HOME/esp/esp-idf/export.sh\n"
                         "  Windows PS:   C:\\esp\\v6.0.1\\esp-idf\\export.ps1",
        "parttool_miss": "错误：在 ESP-IDF 中找不到 parttool.py，请检查 IDF_PATH。",
        "nvsgen_miss":   "错误：在 ESP-IDF 中找不到 nvs_partition_gen.py，请检查 IDF_PATH。",
        "ports_none":    "未找到串口设备，请连接设备后重试。",
        "port_prompt":   "请选择串口编号",
        "port_manual":   "手动输入串口路径",
        "ip_prompt":     "请输入 Home Assistant 的局域网 IP（例如 192.168.1.100）",
        "ip_bad":        "IP 格式不正确，请重新输入。",
        "confirm":       "即将向 {port} 写入 NVS，ota_url=http://{ip}:12800/xiaozhi/ota/\n确认继续？[y/N]",
        "querying":      "正在查询 NVS 分区大小...",
        "size_found":    "NVS 分区大小：{size}",
        "generating":    "正在生成 NVS 二进制文件...",
        "writing":       "正在写入设备...",
        "done":          "完成！请重新上电，设备将连接到本地 StackChan 服务器。",
        "ota_note":      "注意：每次固件 OTA 升级后，需重新运行本脚本写入 NVS。",
        "aborted":       "已取消。",
        "error":         "错误：{msg}",
    },
}

def t(lang, key, **kw):
    return STRINGS[lang][key].format(**kw)

# ── helpers ──────────────────────────────────────────────────────────────────

def find_idf_tool(name):
    """Search for a Python script inside IDF_PATH."""
    idf = os.environ.get("IDF_PATH", "")
    if not idf:
        return None
    for root, _dirs, files in os.walk(idf):
        if name in files:
            return os.path.join(root, name)
    return None

def list_serial_ports():
    patterns = [
        "/dev/tty.usbserial-*",
        "/dev/tty.usbmodem*",
        "/dev/ttyUSB*",
        "/dev/ttyACM*",
        "/dev/cu.usbserial-*",
    ]
    ports = []
    for p in patterns:
        ports.extend(glob.glob(p))
    # Windows COM ports
    if sys.platform == "win32":
        import serial.tools.list_ports
        ports = [p.device for p in serial.tools.list_ports.comports()]
    return sorted(set(ports))

def validate_ip(ip):
    parts = ip.strip().split(".")
    if len(parts) != 4:
        return False
    try:
        return all(0 <= int(p) <= 255 for p in parts)
    except ValueError:
        return False

def run(cmd, lang):
    result = subprocess.run(cmd, capture_output=True, text=True)
    if result.returncode != 0:
        err = result.stderr.strip() or result.stdout.strip()
        print(t(lang, "error", msg=err))
        sys.exit(1)
    return result.stdout

def parse_nvs_size(output):
    """Extract size value from parttool.py get_partition_info output."""
    for line in output.splitlines():
        line = line.strip()
        # Output may be a single hex value or "size: 0x4000"
        if line.startswith("0x") or line.startswith("0X"):
            return int(line, 16)
        if "size" in line.lower():
            parts = line.split()
            for p in parts:
                if p.startswith("0x") or p.startswith("0X"):
                    return int(p, 16)
    # fallback: find any hex-looking token
    import re
    m = re.search(r"0[xX][0-9a-fA-F]+", output)
    if m:
        return int(m.group(), 16)
    return None

# ── main flow ────────────────────────────────────────────────────────────────

def main():
    lang = choose_language()
    print("\n" + "=" * 50)
    print(t(lang, "title"))
    print("=" * 50)

    # Check ESP-IDF
    if not os.environ.get("IDF_PATH"):
        print(t(lang, "idf_missing"))
        sys.exit(1)

    parttool = find_idf_tool("parttool.py")
    if not parttool:
        print(t(lang, "parttool_miss"))
        sys.exit(1)

    nvsgen = find_idf_tool("nvs_partition_gen.py")
    if not nvsgen:
        print(t(lang, "nvsgen_miss"))
        sys.exit(1)

    # Select serial port
    ports = list_serial_ports()
    if not ports:
        print(t(lang, "ports_none"))
        sys.exit(1)

    print()
    for i, p in enumerate(ports, 1):
        print(f"  {i}. {p}")
    print(f"  {len(ports)+1}. {t(lang, 'port_manual')}")

    while True:
        raw = input(f"{t(lang, 'port_prompt')} [1-{len(ports)+1}]: ").strip()
        if raw == str(len(ports) + 1):
            port = input("  > ").strip()
            break
        try:
            idx = int(raw)
            if 1 <= idx <= len(ports):
                port = ports[idx - 1]
                break
        except ValueError:
            pass

    # HA IP
    print()
    while True:
        ip = input(f"{t(lang, 'ip_prompt')}: ").strip()
        if validate_ip(ip):
            break
        print(t(lang, "ip_bad"))

    # Confirm
    print()
    answer = input(t(lang, "confirm", port=port, ip=ip) + " ").strip().lower()
    if answer not in ("y", "yes", "是", "确认"):
        print(t(lang, "aborted"))
        sys.exit(0)

    with tempfile.TemporaryDirectory() as tmpdir:
        csv_path = os.path.join(tmpdir, "nvs.csv")
        bin_path = os.path.join(tmpdir, "nvs.bin")

        # Step 1: query NVS partition size
        print("\n" + t(lang, "querying"))
        out = run([sys.executable, parttool,
                   "--port", port,
                   "get_partition_info", "--partition-name", "nvs"], lang)
        size = parse_nvs_size(out)
        if not size:
            print(t(lang, "error", msg=f"Could not parse NVS size from output:\n{out}"))
            sys.exit(1)
        print(t(lang, "size_found", size=hex(size)))

        # Step 2: write NVS CSV
        ota_url = f"http://{ip}:12800/xiaozhi/ota/"
        with open(csv_path, "w", newline="") as f:
            w = csv.writer(f)
            w.writerow(["key", "type", "encoding", "value"])
            w.writerow(["wifi", "namespace", "", ""])
            w.writerow(["ota_url", "data", "string", ota_url])

        # Step 3: generate NVS binary
        print(t(lang, "generating"))
        run([sys.executable, nvsgen,
             "generate", csv_path, bin_path, hex(size)], lang)

        # Step 4: write to device
        print(t(lang, "writing"))
        run([sys.executable, parttool,
             "--port", port,
             "write_partition", "--partition-name", "nvs", "--input", bin_path], lang)

    print("\n" + t(lang, "done"))
    print(t(lang, "ota_note"))

if __name__ == "__main__":
    main()
