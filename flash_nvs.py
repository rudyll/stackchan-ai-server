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
import re
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
        "host_prompt":   "Enter the StackChan server LAN IP or hostname (e.g. 192.168.1.100 or stackchan.local)",
        "host_bad":      "That doesn't look like a valid IPv4 address or hostname. Try again.",
        "server_port":   "Enter the server port [12800]",
        "port_bad":      "That doesn't look like a valid TCP port. Try again.",
        "confirm":       "WARNING: This replaces the entire NVS partition, including Wi-Fi and other settings. Back up needed NVS data first; Wi-Fi may need setup again.\nReady to write NVS to {port} with ota_url=http://{host}:{server_port}/xiaozhi/ota/\nProceed? [y/N]",
        "querying":      "Querying NVS partition size...",
        "size_found":    "NVS partition size: {size}",
        "generating":    "Generating NVS binary...",
        "writing":       "Writing NVS to device...",
        "done":          "Done! Power-cycle the device. It should now connect to your local StackChan server.",
        "ota_note":      "NOTE: If a firmware upgrade or reflash erases the NVS override, run this script again.",
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
        "host_prompt":   "请输入 StackChan 服务器的局域网 IP 或主机名（例如 192.168.1.100 或 stackchan.local）",
        "host_bad":      "地址格式不正确，请输入 IPv4 地址或局域网主机名。",
        "server_port":   "请输入服务器端口 [12800]",
        "port_bad":      "端口格式不正确，请输入 1–65535 之间的 TCP 端口。",
        "confirm":       "警告：将重写整个 NVS 分区，包括 Wi-Fi 和其他设置。需要保留时请先备份 NVS，写入后可能需要重新配网。\n即将向 {port} 写入 NVS，ota_url=http://{host}:{server_port}/xiaozhi/ota/\n确认继续？[y/N]",
        "querying":      "正在查询 NVS 分区大小...",
        "size_found":    "NVS 分区大小：{size}",
        "generating":    "正在生成 NVS 二进制文件...",
        "writing":       "正在写入设备...",
        "done":          "完成！请重新上电，设备将连接到本地 StackChan 服务器。",
        "ota_note":      "注意：若固件升级或重刷清除了 NVS 覆盖值，再重新运行本脚本注入。",
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

def validate_server_host(host):
    """Accept an IPv4 address or a DNS/mDNS-style hostname for the device URL."""
    value = host.strip()
    if not value or len(value) > 253:
        return False
    if validate_ip(value):
        return True
    labels = value.split(".")
    return all(
        0 < len(label) <= 63
        and re.fullmatch(r"[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?", label)
        for label in labels
    )

def validate_port(port):
    try:
        return 1 <= int(port.strip()) <= 65535
    except (AttributeError, ValueError):
        return False

def build_ota_url(host, server_port):
    return f"http://{host}:{server_port}/xiaozhi/ota/"

def run(cmd, lang):
    result = subprocess.run(cmd, capture_output=True, text=True)
    if result.returncode != 0:
        err = result.stderr.strip() or result.stdout.strip()
        print(t(lang, "error", msg=err))
        sys.exit(1)
    return result.stdout

def parse_nvs_size(output):
    """Extract NVS partition size from parttool.py get_partition_info output.

    With `--info size` parttool prints just '0x4000'.
    Without it, the line is '0x9000 0x4000' (offset size) — pick the LAST
    hex token, since size always follows offset.
    """
    import re
    hex_tokens = re.findall(r"0[xX][0-9a-fA-F]+", output)
    if not hex_tokens:
        return None
    # NVS size is at most 0x100000 (1 MiB) in any sane partition table; the
    # offset is usually 0x9000+. If we got two tokens, the smaller one is size.
    if len(hex_tokens) == 1:
        return int(hex_tokens[0], 16)
    # Two+ tokens: parttool's "offset size" → last token is size.
    return int(hex_tokens[-1], 16)

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

    # Server address
    print()
    while True:
        host = input(f"{t(lang, 'host_prompt')}: ").strip()
        if validate_server_host(host):
            break
        print(t(lang, "host_bad"))

    while True:
        server_port = input(f"{t(lang, 'server_port')}: ").strip() or "12800"
        if validate_port(server_port):
            server_port = str(int(server_port))
            break
        print(t(lang, "port_bad"))

    # Confirm
    print()
    answer = input(t(lang, "confirm", port=port, host=host, server_port=server_port) + " ").strip().lower()
    if answer not in ("y", "yes", "是", "确认"):
        print(t(lang, "aborted"))
        sys.exit(0)

    with tempfile.TemporaryDirectory() as tmpdir:
        csv_path = os.path.join(tmpdir, "nvs.csv")
        bin_path = os.path.join(tmpdir, "nvs.bin")

        # Step 1: query NVS partition size.
        # `--info size` makes parttool emit just the size; without it parttool
        # prints "<offset> <size>" on one line which is fiddly to parse.
        print("\n" + t(lang, "querying"))
        out = run([sys.executable, parttool,
                   "--port", port,
                   "get_partition_info", "--partition-name", "nvs",
                   "--info", "size"], lang)
        size = parse_nvs_size(out)
        if not size:
            print(t(lang, "error", msg=f"Could not parse NVS size from output:\n{out}"))
            sys.exit(1)
        print(t(lang, "size_found", size=hex(size)))

        # Step 2: write NVS CSV
        ota_url = build_ota_url(host, server_port)
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
