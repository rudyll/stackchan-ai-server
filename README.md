# StackChan HA Add-ons

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

English | [中文](README.zh.md)

## About

**StackChan HA Add-ons** turns your [StackChan](https://github.com/m5stack/StackChan) desktop robot into a configurable realtime or OpenAI-compatible voice assistant with full Home Assistant control — no Xiaozhi cloud account, no firmware modifications, no intent scripts to maintain.

StackChan is a palm-sized robot built on the M5Stack CoreS3 (ESP32-S3). The official firmware normally relies on the Xiaozhi cloud for speech recognition, language model, and TTS. This add-on replaces that entirely: choose a native realtime provider, or configure independent OpenAI-compatible STT, LLM, and TTS stages; Home Assistant device control happens locally over the HA WebSocket API.

### Why not just use HA Assist?

| | **StackChan AI Server** | **HA Assist** |
|---|---|---|
| **Understanding** | GPT-4o / Gemini 2.5 — understands natural, conversational speech | Rules-based intent matching — only recognises pre-defined phrases |
| **Conversation** | Full multi-turn context across the whole session | Stateless — every utterance is independent, no memory of what was just said |
| **Ambiguous commands** | Asks one clarifying question, then acts (e.g. "too hot" → "which room?" → turns on AC) | Fails or picks a random device if the command doesn't match a pattern exactly |
| **Multiple devices** | Names the matches and asks "which one, or all?" | No disambiguation |
| **Voice quality** | Real-time neural audio — natural, low-latency | TTS pipeline with noticeable STT→LLM→TTS delay |
| **Setup** | One system prompt, no scripts | Requires defining intents and scripts per device action |
| **Scenes / Scripts / Automations** | Searches and activates by name automatically | Only if you write a matching intent |

**Key features:**
- Choice of provider: **OpenAI Realtime API**, **Google Gemini Live API**, or an OpenAI-compatible STT → LLM → TTS pipeline (TokenHub, OpenRouter, or a compatible endpoint)
- Full multi-turn conversation — the AI remembers context across utterances within a session
- Controls lights, climate, covers, media players, scripts, scenes and automations by voice
- Area-based control ("turn off all lights in the living room")
- No Xiaozhi account — audio is processed by OpenAI/Gemini, HA stays on your LAN
- Unmodified official firmware — no recompile needed

## How It Works

This add-on replaces the Xiaozhi cloud entirely. The StackChan device thinks it's talking to Xiaozhi's servers, but it's actually talking to this local server running on your Home Assistant.

```
StackChan ESP32-S3  (unmodified xiaozhi-esp32 firmware)
    │  Xiaozhi WebSocket protocol v3 (OPUS audio + JSON)
    ▼
StackChan AI Server  (this add-on, on your HA at port 12800)
    ├─ /xiaozhi/ota/  → returns local WebSocket address
    └─ /xiaozhi/ws    → WebSocket session
         ├─ OpenAI Realtime API  ─┐
         │                        ├─ STT + LLM + TTS, streaming (pick one)
         └─ Gemini Live API     ──┘
         └─ Home Assistant WebSocket API (device control)
```

**Audio pipeline (streaming, ~0.5–1.5s latency):**

```
Device OPUS (16kHz) → PCM → OpenAI Realtime / Gemini Live
                              ↓ server VAD detects speech end
                         Streaming PCM response (24kHz)
                              ↓
                         OPUS encode → Device speaker
```

No Xiaozhi account is needed. The cloud dependency is whichever provider or compatible endpoint you configure.

---

## Add-ons

### StackChan AI Server

Low-latency speech-to-speech conversation powered by **OpenAI Realtime API** (`gpt-realtime`) **or Google Gemini Live API** (`gemini-2.5-flash-native-audio-latest`) — pick your provider in the add-on UI. Both options give natural-language control of Home Assistant devices.

**Features:**
- Switchable AI provider: OpenAI Realtime / Gemini Live (dropdown)
- ~0.5–1.5s response latency (server-side VAD, streaming audio)
- Controls HA devices by voice: lights, climate, covers, media players, scripts, scenes and automations
- Area-based control ("turn off all lights in the living room")
- Full multi-turn conversation — context maintained across utterances within a session
- OpenAI and Gemini model names can be entered as free text

---

## Installation

1. In Home Assistant, go to **Settings → Add-ons → Add-on Store**
2. Click the three-dot menu (top right) → **Repositories**
3. Add this URL:
   ```
   https://github.com/rudyll/stackchan-ai-server
   ```
4. Find **StackChan AI Server** in the store and click **Install**

### Standalone Docker (beta)

Home Assistant is optional. To run the server with Docker Compose, copy `stackchan-server/.env.standalone.example` to `.env`, set `STACKCHAN_LOCAL_HOST` to the Docker host's LAN IP, add an AI API key, then run:

```bash
cd stackchan-server
cp .env.standalone.example .env
docker compose -f docker-compose.standalone.yml up --build -d
```

The WebSocket server is available on port `12800`. The settings UI is bound to `127.0.0.1:8099`; the first startup prints a Bearer token and persists it under `./data/settings-token`. Standalone mode does not connect to Home Assistant or expose the port-443 OTA interception. Configure the device OTA URL as `http://<server-LAN-IP>:12800/xiaozhi/ota/`.
5. Go to the add-on **Configuration** tab and fill in the required fields (see below)
6. Start the add-on

---

## Configuration

For the recommended setup, open the add-on's **Open Web UI** button after starting it. The protected Home Assistant Ingress page groups settings into **Basic**, **Voice pipeline**, **Background tasks**, and **Device profiles**. The standard add-on Configuration tab remains available for legacy settings.

Pick **one** AI provider via `ai_provider` and fill in only its API key. The other provider's fields can stay blank.

| Option | Required | Description |
|--------|----------|-------------|
| `local_host` | ✅ | LAN IP of your Home Assistant instance (e.g. `192.168.1.100`). The device uses this to connect. |
| `ha_enabled` | | Keep Home Assistant tools and background tasks enabled. The add-on defaults to `true`; standalone runtime will default to `false`. |
| `ha_mcp_token` | When HA is enabled | HA Long-Lived Access Token. Create one in **Profile → Security → Long-Lived Access Tokens**. Leave it empty in standalone mode. |
| `ai_provider` | ✅ | `openai` (default), `gemini`, `tokenhub`, `openrouter`, or `openai_compatible`. |
| `system_prompt` | | Custom personality/instructions for the assistant. |
| **OpenAI** (when `ai_provider=openai`) | | |
| `openai_api_key` | ✅ | Your OpenAI API key from [platform.openai.com](https://platform.openai.com). |
| `openai_realtime_model` | | Realtime model name (free text). Default: `gpt-realtime`. Other model IDs may be entered when available to your account. |
| `openai_tts_voice` | | TTS voice. Default: `alloy`. Female voices: `nova`, `shimmer`, `coral`, `sage`, `cedar`, `marin`, `cove`. |
| **Gemini** (when `ai_provider=gemini`) | | |
| `gemini_api_key` | ✅ | Your Google AI Studio API key from [aistudio.google.com](https://aistudio.google.com/app/apikey). |
| `gemini_model` | | Gemini Live model name (free text). Default: `gemini-2.5-flash-native-audio-latest`. Other model IDs may be entered when supported by Gemini Live. |
| `gemini_voice` | | TTS voice. Default: `Aoede`. 30 native audio voices available via dropdown (Aoede, Charon, Fenrir, Kore, Puck, Leda, Orus, Zephyr, and more). |
| `gemini_enable_tools` | | Enable HA device control tools for Gemini. Default: on. |
| `gemini_enable_search` | | Enable Google Search grounding for Gemini. Default: off. **⚠️ Mutually exclusive with `gemini_enable_tools`** — Gemini does not allow grounding and function calling simultaneously. Enabling both causes 1011 connection errors. To use web search, set `gemini_enable_tools=false`. |
| **Compatible pipeline** | | For `tokenhub`, `openrouter`, and `openai_compatible`: a turn-based STT → Chat Completions → TTS path, not OpenAI Realtime WebSocket. |
| `stt_*` | | Optional independent STT Base URL, API Key, and model. |
| `llm_*` | | Optional independent LLM Base URL, API Key, and model. |
| `tts_*` | | Optional independent TTS Base URL, API Key, model, and voice. |
| `compatible_*` | | Backward-compatible fallback values for all three stages when their stage-specific values are blank. |
| TokenHub | | Select `tokenhub`; set `tokenhub_base_url`, `tokenhub_api_key`, plus compatible model fields. |
| OpenRouter | | Select `openrouter`; set `openrouter_api_key`, plus compatible model fields. The base URL is set automatically. |
| Generic compatible endpoint | | Select `openai_compatible`; set `compatible_base_url`, `compatible_api_key`, and compatible model fields. |
| **Background tasks (Beta)** (currently `ai_provider=openai` only) | | Long-running work enters a per-device queue while the realtime conversation remains available. |
| `background_tasks_enabled` | | Enable background tasks. Default: off. |
| `background_agent_base_url` | | OpenAI-compatible base URL supporting `/v1/chat/completions`. |
| `background_agent_api_key` | | Agent key. When blank, falls back to `llm_api_key`, `compatible_api_key`, then `openai_api_key`. |
| `background_agent_model` | ✅ when enabled | Chat Completions model for background work; do not use a Realtime audio model. |
| `background_agent_timeout_seconds` | | Per-task timeout. Default: 300 seconds. |

The add-on logs `[LAT]` timings from device `listen:stop` to STT, LLM, TTS start, and first audio. Use these real measurements to compare providers.

### Continuous conversation and background tasks (Beta)

> **Beta:** This path has automated coverage but still needs broad testing with physical StackChan devices, live Home Assistant installations, and different OpenAI-compatible background models.

When enabled, OpenAI Realtime receives tools to create, inspect, and cancel background work. Short HA operations still execute directly. Longer analysis and multi-step operations enter a FIFO queue scoped to the device `Device-Id`, while the realtime voice session acknowledges the work and remains available. State is persisted in `/data/background-tasks.json`.

Completion waits until the user, model, and physical audio queue are idle. A result is claimed by one voice connection before announcement; disconnecting or interrupting releases the claim for a later reconnect, and a successfully announced result is not repeated. In-flight work that cannot be recovered after an add-on restart becomes an explicit failed result.

The first backend uses an OpenAI-compatible Chat Completions model with the existing Home Assistant tools. It does not automatically gain web search or code execution. Background tools are currently exposed only to the OpenAI Realtime frontend, not Gemini Live or the turn-based compatible voice pipeline.

### Provider choice and mainland-China latency

`openai` and `gemini` are native bidirectional realtime audio integrations. They require the respective OpenAI Realtime or Gemini Live API key. From mainland China, their overseas endpoints can add latency or intermittent audio delivery; real performance depends on the network route, not only the selected model.

`tokenhub`, `openrouter`, and `openai_compatible` use the turn-based HTTP pipeline. Configure `stt_*`, `llm_*`, and `tts_*` separately to mix providers. Each stage's endpoint must support its matching OpenAI-style endpoint: `/v1/audio/transcriptions`, `/v1/chat/completions`, or `/v1/audio/speech`. A TokenHub account that supplies only text chat is suitable for the LLM stage, but needs separate STT and TTS providers. OpenRouter routing and an OpenAI-compatible label likewise do not guarantee low mainland-China latency.

For the best chance of a smooth mainland-China deployment, use local or domestic STT and TTS with a domestic LLM. The independent stage settings support this combination; leave a stage blank only when the legacy `compatible_*` endpoint can provide it.

### Playback buffering and device profiles

`audio_prebuffer_ms` defaults to `300`: it accumulates initial audio before playback so uneven upstream audio chunks do not become audible gaps. `audio_prebuffer_max_wait_ms` defaults to `900` and caps the wait. Try `480 / 1200` for unstable networks or `120 / 500` for faster first speech.

`device_profiles` is a JSON object keyed by WebSocket `Device-Id`. Its fields override the global provider, prompt, model, and voice settings, while API keys remain global. Example:

```json
{
  "AA:BB:CC:DD:EE:FF": {
    "system_prompt": "You are the living-room StackChan. Reply briefly.",
    "openai_tts_voice": "coral"
  }
}
```

Supported overrides: `provider`, `system_prompt`, `openai_realtime_model`, `openai_tts_voice`, `gemini_model`, `gemini_voice`, `compatible_model`, `compatible_stt_model`, `compatible_tts_model`, and `compatible_tts_voice`.

### Wake word, standby, and power

Wake-word detection and always-listening behaviour belong to the StackChan firmware. This add-on only receives audio after the firmware emits `listen:detect/start/stop`, and does not currently control a custom wake word. For less power draw and fewer accidental activations, use touch/screen activation; a firmware variant could also add an idle timeout and a 15–30 second conversation window after a wake event.

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
