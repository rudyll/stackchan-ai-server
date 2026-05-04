# StackChan HA Add-ons

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

English | [中文](README.zh.md)

Home Assistant add-on repository for StackChan — an AI voice assistant robot built on M5Stack CoreS3, using the unmodified [xiaozhi-esp32](https://github.com/78/xiaozhi-esp32) firmware.

## How It Works

This add-on replaces the Xiaozhi cloud entirely. The StackChan device thinks it's talking to Xiaozhi's servers, but it's actually talking to this local server running on your Home Assistant.

```
StackChan ESP32-S3  (unmodified xiaozhi-esp32 firmware)
    │  Xiaozhi WebSocket protocol v3 (OPUS audio + JSON)
    ▼
StackChan AI Server  (this add-on, on your HA at port 12800)
    ├─ /xiaozhi/ota/  → returns local WebSocket address
    └─ /xiaozhi/ws    → WebSocket session
         ├─ OpenAI Realtime API (STT + LLM + TTS, streaming)
         └─ Home Assistant WebSocket API (device control)
```

**Audio pipeline (streaming, ~0.5–1.5s latency):**

```
Device OPUS (16kHz) → PCM → OpenAI Realtime API
                              ↓ server VAD detects speech end
                         Streaming PCM response (24kHz)
                              ↓
                         OPUS encode → Device speaker
```

No Xiaozhi account needed. No cloud dependency except OpenAI.

---

## Add-ons

### StackChan AI Server

Powered by **OpenAI Realtime API** (`gpt-realtime-1.5`) for low-latency speech-to-speech conversation, with Home Assistant device control via natural language.

**Features:**
- ~0.5–1.5s response latency (server-side VAD, streaming audio)
- Controls HA devices by voice: lights, climate, covers, media players, scripts
- Area-based control ("turn off all lights in the living room")
- Conversation history maintained across utterances within a session
- Configurable voice and model via dropdown in the add-on UI

---

## Installation

1. In Home Assistant, go to **Settings → Add-ons → Add-on Store**
2. Click the three-dot menu (top right) → **Repositories**
3. Add this URL:
   ```
   https://github.com/rudyll/stackchan_ha_addons
   ```
4. Find **StackChan AI Server** in the store and click **Install**
5. Go to the add-on **Configuration** tab and fill in the required fields (see below)
6. Start the add-on

---

## Configuration

| Option | Required | Description |
|--------|----------|-------------|
| `local_host` | ✅ | LAN IP of your Home Assistant instance (e.g. `192.168.1.100`). The device uses this to connect. |
| `ha_mcp_token` | ✅ | HA Long-Lived Access Token. Create one in **Profile → Security → Long-Lived Access Tokens**. |
| `openai_api_key` | ✅ | Your OpenAI API key from [platform.openai.com](https://platform.openai.com). |
| `openai_realtime_model` | | Realtime model to use. Default: `gpt-realtime-1.5`. Mini (cheaper): `gpt-realtime-mini`, `gpt-4o-mini-realtime-preview`. |
| `openai_tts_voice` | | TTS voice. Default: `alloy`. Female voices: `nova`, `shimmer`, `coral`, `sage`, `cedar`, `marin`, `cove`. |
| `system_prompt` | | Custom personality/instructions for the assistant. |

---

## Firmware Setup

The device firmware needs to know your local server address instead of the Xiaozhi cloud. There are two ways to do this.

> **Which method should I use?**
> Use **Method A (NVS)** for most cases — it survives firmware OTA upgrades and doesn't require recompiling.
> Use **Method B (compile)** only if you want to make other firmware customisations at the same time.

### Method A — Write NVS key (recommended)

The firmware checks NVS (non-volatile storage) for an OTA URL override before using its hardcoded default. This setting **persists across firmware OTA upgrades**, so you only need to do it once.

#### Prerequisites

You need ESP-IDF installed. Follow the [official installation guide](https://docs.espressif.com/projects/esp-idf/en/stable/esp32s3/get-started/index.html) if you haven't done this yet.

**Every time you open a new terminal**, activate the ESP-IDF environment first — otherwise the `parttool.py` scripts will not be found.

- **macOS / Linux:**
  ```bash
  . $HOME/esp/esp-idf/export.sh
  ```
- **Windows (PowerShell):**
  ```powershell
  C:\esp\v6.0.1\esp-idf\export.ps1
  ```
- **Windows (Command Prompt):**
  ```cmd
  C:\esp\v6.0.1\esp-idf\export.bat
  ```

Verify activation: `idf.py --version` should print the IDF version without errors.

#### Steps

**Step 1 — Find your NVS partition size:**
```bash
python3 $IDF_PATH/components/partition_table/parttool.py \
    --port /dev/tty.usbserial-XXXX \
    get_partition_info --partition-name nvs
```
Note the `size` value (commonly `0x4000` or `0x6000`). Replace `/dev/tty.usbserial-XXXX` with your device's serial port (see [Finding your serial port](#finding-your-serial-port) below).

**Step 2 — Create the NVS data file:**
```bash
cat > nvs.csv << 'EOF'
key,type,encoding,value
wifi,namespace,,
ota_url,data,string,http://<YOUR_HA_IP>:12800/xiaozhi/ota/
EOF
```
Replace `<YOUR_HA_IP>` with your Home Assistant's LAN IP (same as `local_host` in the add-on config).

**Step 3 — Generate the NVS binary** (replace `0x4000` with the actual size from Step 1):
```bash
python3 $IDF_PATH/components/nvs_flash/nvs_partition_generator/nvs_partition_gen.py \
    generate nvs.csv nvs.bin 0x4000
```

**Step 4 — Write to device:**
```bash
python3 $IDF_PATH/components/partition_table/parttool.py \
    --port /dev/tty.usbserial-XXXX \
    write_partition --partition-name nvs --input nvs.bin
```

### Method B — Compile from source

Use this only if you need to make other firmware customisations. Note that the OTA URL baked in via `menuconfig` will be **overwritten if the device performs a firmware OTA upgrade** — in that case you will need to redo Step 3–4 of Method A anyway.

#### Prerequisites

Same ESP-IDF installation and environment activation as Method A above.

#### Steps

1. Clone and set up the [StackChan firmware](https://github.com/m5stack/StackChan/tree/main/firmware):
   ```bash
   git clone https://github.com/m5stack/StackChan.git
   cd StackChan/firmware
   python3 fetch_repos.py
   ```

2. Install third-party component dependencies:
   ```bash
   idf.py add-dependency "bblanchon/arduinojson"
   idf.py update-dependencies
   ```
   **Do not skip this step** — it installs `ArduinoJson` and other components declared in `idf_component.yml`. Skipping it causes a `Failed to resolve component 'ArduinoJson'` error during build.

3. Open menuconfig and set the OTA URL:
   ```bash
   idf.py menuconfig
   ```
   - Press `/` and search for `OTA_URL`
   - Set it to `http://<YOUR_HA_IP>:12800/xiaozhi/ota/`
   - Save and exit

4. Build and flash:
   ```bash
   idf.py set-target esp32s3
   idf.py build
   idf.py -p /dev/tty.usbserial-XXXX -b 921600 flash
   ```

### Finding your serial port

- **macOS:** `ls /dev/tty.usb*`
- **Linux:** `ls /dev/ttyUSB* /dev/ttyACM*`

### First-time Wi-Fi setup

If the device has no Wi-Fi credentials (factory reset or first flash):

1. Download the **StackChan World** app (iOS / Android)
2. Open the app and follow the "Add device" flow
3. The app uses Bluetooth to push your Wi-Fi credentials to the device
4. Once connected, the device will use the OTA URL you configured (via NVS or menuconfig) to reach your local add-on instead of the Xiaozhi cloud

### After a firmware OTA upgrade

If the device performs a firmware OTA upgrade, the NVS `ota_url` key is preserved — your local server address remains intact. However, if you used Method B (menuconfig), the baked-in OTA URL **will be overwritten** by the new firmware. In that case, redo Steps 3–4 of Method A to re-inject the NVS key.

---

## Ports

| Port | Purpose |
|------|---------|
| `12800/tcp` | Main StackChan WebSocket server (OTA discovery + WebSocket AI session) |
| `443/tcp` | Legacy HTTPS intercept (unused) |

---

## License

MIT — see [LICENSE](LICENSE)
