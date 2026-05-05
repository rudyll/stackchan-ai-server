# StackChan HA Add-ons

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

English | [中文](README.zh.md)

## About

**StackChan HA Add-ons** turns your [StackChan](https://github.com/m5stack/StackChan) desktop robot into an AI voice assistant integrated with your smart home — no Xiaozhi account needed.

StackChan is a palm-sized robot built on the M5Stack CoreS3 (ESP32-S3). It ships with the open-source [xiaozhi-esp32](https://github.com/78/xiaozhi-esp32) firmware, which normally relies on the Xiaozhi cloud for speech recognition, language model, and text-to-speech. This add-on replaces that with a Home Assistant add-on: your voice data goes to **OpenAI** instead of Xiaozhi, and Home Assistant stays entirely on your local network.

The device firmware is **never modified** — the add-on speaks the same Xiaozhi WebSocket protocol v3 the device already expects. Voice commands are processed by either **OpenAI Realtime API** or **Google Gemini Live API** (streaming speech-to-speech, ~0.5–1.5 s latency, switchable from the add-on UI), and the robot can control any Home Assistant device by voice.

**Key features:**
- ~0.5–1.5 s end-to-end latency via OpenAI Realtime API streaming
- Controls lights, climate, covers, media players, and scripts by voice
- Area-based control ("turn off all lights in the living room")
- No Xiaozhi account — audio is processed by OpenAI, HA stays on your LAN
- Easy installation as a standard Home Assistant add-on

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

Pick **one** AI provider via `ai_provider` and fill in only its API key. The other provider's fields can stay blank.

| Option | Required | Description |
|--------|----------|-------------|
| `local_host` | ✅ | LAN IP of your Home Assistant instance (e.g. `192.168.1.100`). The device uses this to connect. |
| `ha_mcp_token` | ✅ | HA Long-Lived Access Token. Create one in **Profile → Security → Long-Lived Access Tokens**. |
| `ai_provider` | ✅ | `openai` (default) or `gemini`. Selects which backend handles speech + LLM + TTS. |
| `system_prompt` | | Custom personality/instructions for the assistant. |
| **OpenAI** (when `ai_provider=openai`) | | |
| `openai_api_key` | ✅ | Your OpenAI API key from [platform.openai.com](https://platform.openai.com). |
| `openai_realtime_model` | | Realtime model. Default: `gpt-realtime-1.5`. Mini (cheaper): `gpt-realtime-mini`, `gpt-4o-mini-realtime-preview`. |
| `openai_tts_voice` | | TTS voice. Default: `alloy`. Female voices: `nova`, `shimmer`, `coral`, `sage`, `cedar`, `marin`, `cove`. |
| **Gemini** (when `ai_provider=gemini`) | | |
| `gemini_api_key` | ✅ | Your Google AI Studio API key from [aistudio.google.com](https://aistudio.google.com/app/apikey). |
| `gemini_model` | | Gemini Live model. Default: `gemini-2.5-flash-preview-native-audio-dialog`. |
| `gemini_voice` | | TTS voice. Default: `Aoede`. Options: `Aoede`, `Charon`, `Fenrir`, `Kore`, `Puck`. |

---

## Firmware Setup

The device firmware needs to know your local server address instead of the Xiaozhi cloud. There are two ways to do this.

> **Which method should I use?**
> Use **Method A (NVS)** for most cases — re-injection is just two commands and doesn't require recompiling.
> Use **Method B (compile)** only if you want to make other firmware customisations at the same time.
>
> ⚠️ **Important:** The official xiaozhi-esp32 OTA upgrade writes a full flash image and **overwrites the NVS partition**. After any firmware upgrade you will need to re-inject the NVS key (Steps 3–4 of Method A). This is still much faster than recompiling.
>
> 💡 **Shortcut:** Run `python3 flash_nvs.py` for an interactive guided injector that handles all four steps automatically (English / 中文).

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

The official xiaozhi-esp32 OTA upgrade writes a full flash image, which **overwrites the NVS partition**. After any firmware upgrade you will need to re-inject the NVS key by redoing Steps 3–4 of Method A. This is still much faster than recompiling the firmware from source.

---

## Ports

| Port | Purpose |
|------|---------|
| `12800/tcp` | Main StackChan WebSocket server (OTA discovery + WebSocket AI session) |
| `443/tcp` | Legacy HTTPS intercept (unused) |

---

## License

MIT — see [LICENSE](LICENSE)
