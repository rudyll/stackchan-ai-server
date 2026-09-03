# StackChan AI Server

<p align="center"><img src="stackchan-server/logo.png" alt="StackChan with a hand-drawn crown and angel wings" width="200" height="200"></p>

[![License: AGPL-3.0-only](https://img.shields.io/badge/License-AGPL--3.0--only-blue.svg)](LICENSE)
[![Sponsor via PayPal](https://img.shields.io/badge/Sponsor-PayPal-0070BA?logo=paypal&logoColor=white)](https://paypal.me/unitekno)

English | [中文](README.zh.md)

## About

**StackChan AI Server** turns your [StackChan](https://github.com/m5stack/StackChan) desktop robot into a configurable realtime or OpenAI-compatible voice assistant, with optional Home Assistant control — no Xiaozhi cloud account, no intent scripts to maintain.

StackChan is a palm-sized robot built on the M5Stack CoreS3 (ESP32-S3). The official firmware normally relies on the Xiaozhi cloud for speech recognition, language model, and TTS. This project replaces that dependency with a self-hosted server: run it as a Home Assistant add-on or standalone Docker service, choose a native realtime provider, or configure independent OpenAI-compatible STT, LLM, and TTS stages. Home Assistant device control is enabled only when the HA runtime is connected.

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
- Home Assistant is optional — standalone Docker provides voice conversation without HA
- No Xiaozhi account — audio is processed by your configured provider, while HA stays on your LAN when enabled
- Unmodified official firmware — no recompile needed

## How It Works

This project replaces the Xiaozhi cloud entirely. The StackChan device thinks it's talking to Xiaozhi's servers, but it's actually talking to this self-hosted server running either as a Home Assistant add-on or a standalone Docker service.

```
StackChan ESP32-S3  (unmodified xiaozhi-esp32 firmware)
    │  Xiaozhi WebSocket protocol v3 (OPUS audio + JSON)
    ▼
StackChan AI Server  (HA add-on or standalone Docker, port 12800)
    ├─ /xiaozhi/ota/  → returns local WebSocket address
    └─ /xiaozhi/ws    → WebSocket session
         ├─ OpenAI Realtime API  ─┐
         │                        ├─ STT + LLM + TTS, streaming (pick one)
         └─ Gemini Live API     ──┘
         └─ Home Assistant WebSocket API (device control, optional)
```

The device's audio and WebSocket traffic goes directly to StackChan AI Server; Home Assistant is not a transport relay. When HA is enabled, the server makes a separate authenticated HA WebSocket API connection only for smart-home control. Standalone can optionally use the same direct HA bridge; without that opt-in, it has no HA connection.

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

## Distributions

### StackChan AI Server

Low-latency speech-to-speech conversation powered by **OpenAI Realtime API** (`gpt-realtime`) **or Google Gemini Live API** (`gemini-2.5-flash-native-audio-latest`) — pick your provider in the add-on UI. Both options give natural-language control of Home Assistant devices.

**Features:**
- Switchable AI provider: OpenAI Realtime / Gemini Live (dropdown)
- ~0.5–1.5s response latency (server-side VAD, streaming audio)
- Controls HA devices by voice: lights, climate, covers, media players, scripts, scenes and automations
- Area-based control ("turn off all lights in the living room")
- Full multi-turn conversation — context maintained across utterances within a session
- OpenAI and Gemini model names can be entered as free text

The same server is also available as a standalone Docker runtime. See the standalone section below for setup and device onboarding.

---

## Installation

1. In Home Assistant, go to **Settings → Add-ons → Add-on Store**
2. Click the three-dot menu (top right) → **Repositories**
3. Add this URL:
   ```
   https://github.com/rudyll/stackchan-ai-server
   ```
4. Find **StackChan AI Server** in the store and click **Install**
5. Go to the add-on **Configuration** tab and fill in the required fields (see below)
6. Start the add-on

**HA update:** Version [2.8.0-beta.3](https://github.com/rudyll/stackchan-ai-server/releases/tag/v2.8.0-beta.3) brings the shared settings GUI, permanent NVS guide and crown-and-wings artwork into the add-on. Back up first, refresh the add-on store, then update and restart; keep the existing options and data. **Open Web UI** is the shared graphical page, while HA's **Configuration** tab is a separate form. Updating repository images alone does not update an installed server.

### Standalone Docker (beta)

Home Assistant is optional. To run the server with Docker Compose, copy `stackchan-server/.env.standalone.example` to `.env`, set `STACKCHAN_LOCAL_HOST` to the Docker host's LAN IP, add an AI API key, then run:

```bash
cd stackchan-server
cp .env.standalone.example .env
docker compose -f docker-compose.standalone.yml up --build -d
```

**Docker updates:** Fetch the latest source first, preserve `.env` and the mounted data directory, then rerun the command above. This distribution builds locally from source; there is no prebuilt registry image to pull. The [container checks](https://github.com/rudyll/stackchan-ai-server/actions/workflows/containers.yml) build all four declared HA architectures and exercise both launchers; they do not replace real-device/HA testing.

If `STACKCHAN_SETTINGS_TOKEN` is empty, retrieve the generated token from the first-start log before opening the GUI:

```bash
docker compose -f docker-compose.standalone.yml logs --no-color --tail=50 stackchan
```

By default, the WebSocket server is published on host port `12800` (the container listens on `12800`). The settings UI is published on `127.0.0.1:8099` by default; if you set `STACKCHAN_SETTINGS_PORT`, open that host port instead (the container still listens on `8099`). Enter the Bearer token printed on first startup. The browser receives a short-lived HttpOnly session cookie, while API requests remain protected. The token is persisted under `./data/settings-token`. The GUI provides separate OpenAI Realtime, Gemini Live, TokenHub, OpenRouter, and OpenAI-compatible provider entries, plus a provider check that can fetch model names and populate the matching voice catalog. Standalone defaults to no Home Assistant connection. If you want entity control, enable **Standalone → Home Assistant bridge** in the GUI, enter the HA URL and a Long-Lived Access Token, then enable the provider's HA tools; the server connects directly to HA's Core WebSocket API for entity discovery, state queries, and allowed service calls. The bridge does not relay device audio or WebSocket traffic. Standalone mode does not expose the port-443 OTA interception. Configure the device OTA URL as `http://<server-LAN-IP>:12800/xiaozhi/ota/`.

If the HA add-on and standalone Docker run on the same host, keep the add-on on `12800` and set `STACKCHAN_WS_PORT=12801` (and optionally `STACKCHAN_SETTINGS_PORT=8100`) before starting Compose. Then configure the device with `http://<server-LAN-IP>:12801/xiaozhi/ota/`. The container still listens internally on `12800`; the launcher returns the selected public port in the WebSocket URL.

Each device has one active OTA/WebSocket target. Devices configured with the HA add-on URL connect to the add-on; devices configured with the standalone URL connect to standalone. A stock firmware device with neither an NVS override nor a compiled `OTA_URL` continues to use its original cloud/HA interception path and will not discover standalone automatically.

### macOS — download the ready-made DMG (preview)

Download the [macOS 0.1.1 universal DMG](https://github.com/rudyll/stackchan-ai-server/releases/download/macos-v0.1.1/StackChan-AI-Server-0.1.1-macos-universal.dmg) from [GitHub Releases](https://github.com/rudyll/stackchan-ai-server/releases/tag/macos-v0.1.1), drag **StackChan AI Server.app** into **Applications**, then launch it. **Users do not need to build anything or install Docker, Go, Homebrew or OPUS.** The app contains Apple Silicon and Intel executables targeting macOS 12+, with the audio codec statically included. The Release also provides a SHA-256 checksum file.

The app chooses free ports, opens the local settings page and shows the first-run login token. Its 3D StackChan icon is also used in the shared HA/standalone GUI. This is an **ad-hoc signed preview, not Developer ID signed or Apple notarized**; after verifying the download source, use **System Settings → Privacy & Security → Open Anyway** if blocked as an unidentified developer ([Apple guidance](https://support.apple.com/en-us/102445)). Do not bypass malware warnings or disable Gatekeeper globally. Allow incoming connections if the macOS firewall asks and LAN devices need access. This preview has no menu-bar control: stop its `stackchan-server` processes in Activity Monitor before upgrading. See [installation details and optional developer build instructions](stackchan-server/macos/README.md).

The optional `STACKCHAN_STANDALONE_HA_ENABLED`, `STACKCHAN_STANDALONE_HA_URL`, and `STACKCHAN_STANDALONE_HA_TOKEN` variables can preconfigure the direct HA bridge. The optional `STACKCHAN_DEVICE_PROFILES`, `STACKCHAN_SYSTEM_PROMPT`, `STACKCHAN_AUDIO_PREBUFFER_MS`, and `STACKCHAN_AUDIO_PREBUFFER_MAX_WAIT_MS` variables provide first-start defaults. After startup, the GUI is the recommended way to edit them; saved values are persisted under the mounted data directory. The HA token is not returned by the settings API; leaving its password field blank keeps the saved token.

---

## Configuration

For the recommended setup, open the add-on's **Open Web UI** button after starting it. The protected Home Assistant Ingress page groups settings into **Basic**, **Voice pipeline**, **Background tasks**, and **Device profiles**. The standard add-on Configuration tab remains available for legacy settings.

Pick **one** AI provider via `ai_provider` and fill in only its API key. The other provider's fields can stay blank.

| Option | Required | Description |
|--------|----------|-------------|
| `local_host` | ✅ | LAN IP of the host running StackChan AI Server (e.g. `192.168.1.100`). For the add-on this is normally the HA host; for standalone it is the Docker host. |
| `ha_enabled` | | Keep Home Assistant tools and background tasks enabled. The add-on defaults to `true`; standalone runtime will default to `false`. |
| `ha_mcp_token` | When HA is enabled | HA Long-Lived Access Token. Create one in **Profile → Security → Long-Lived Access Tokens**. Leave it empty in standalone mode. |
| `standalone_ha_enabled` | | Opt in to a direct Home Assistant connection from standalone. Default: `false`. |
| `standalone_ha_url` | When standalone HA is enabled | Home Assistant URL, such as `http://homeassistant.local:8123`; the server adds `/api/websocket` automatically. |
| `standalone_ha_token` | When standalone HA is enabled | HA Long-Lived Access Token. It is stored in the standalone data directory and never returned by the settings API. |
| `ai_provider` | ✅ | `openai` (default), `gemini`, `tokenhub`, `openrouter`, or `openai_compatible`. |
| `system_prompt` | | Custom personality/instructions for the assistant. |
| **OpenAI** (when `ai_provider=openai`) | | |
| `openai_api_key` | ✅ | Your OpenAI API key from [platform.openai.com](https://platform.openai.com). |
| `openai_realtime_model` | | Realtime model name (free text). Default: `gpt-realtime`. Other model IDs may be entered when available to your account. |
| `openai_tts_voice` | | Realtime voice. Default: `alloy`. Built-in voices include `alloy`, `ash`, `ballad`, `coral`, `echo`, `sage`, `shimmer`, `verse`, `marin`, and `cedar`. |
| **Gemini** (when `ai_provider=gemini`) | | |
| `gemini_api_key` | ✅ | Your Google AI Studio API key from [aistudio.google.com](https://aistudio.google.com/app/apikey). |
| `gemini_model` | | Gemini Live model name (free text). Default: `gemini-2.5-flash-native-audio-latest`. Other model IDs may be entered when supported by Gemini Live. |
| `gemini_voice` | | TTS voice. Default: `Aoede`. 30 native audio voices available via dropdown (Aoede, Charon, Fenrir, Kore, Puck, Leda, Orus, Zephyr, and more). |
| `gemini_enable_tools` | | Enable HA device control tools for Gemini. Add-on default: on; standalone default: off. |
| `gemini_enable_search` | | Enable Google Search grounding for Gemini. Default: off. **⚠️ Mutually exclusive with `gemini_enable_tools`** — Gemini does not allow grounding and function calling simultaneously. Enabling both causes 1011 connection errors. To use web search, set `gemini_enable_tools=false`. |
| **Compatible pipeline** | | For `tokenhub`, `openrouter`, and `openai_compatible`: a turn-based STT → Chat Completions → TTS path, not OpenAI Realtime WebSocket. |
| `stt_*` | | Optional independent STT Base URL, API Key, and model. |
| `llm_*` | | Optional independent LLM Base URL, API Key, and model. |
| `tts_*` | | Optional independent TTS Base URL, API Key, model, and voice. |
| `compatible_*` | | Backward-compatible fallback values for all three stages when their stage-specific values are blank. |
| TokenHub | | Select `tokenhub`; set `tokenhub_base_url` and `tokenhub_api_key`, then configure stage-specific `stt_*`, `llm_*`, and `tts_*` fields as needed. |
| OpenRouter | | Select `openrouter`; set `openrouter_api_key` and configure the LLM model plus STT/TTS fields. The base URL is set automatically. |
| Generic compatible endpoint | | Select `openai_compatible`; use the GUI's `llm_*`, `stt_*`, and `tts_*` fields. Legacy `compatible_*` values remain as fallbacks. |
| **Background tasks (Beta)** (currently `ai_provider=openai` only) | | Long-running work enters a per-device queue while the realtime conversation remains available. |
| `background_tasks_enabled` | | Enable background tasks. Default: off. |
| `background_agent_base_url` | | OpenAI-compatible base URL supporting `/v1/chat/completions`. |
| `background_agent_api_key` | | Agent key. When blank, falls back to `llm_api_key`, `compatible_api_key`, then `openai_api_key`. |
| `background_agent_model` | ✅ when enabled | Chat Completions model for background work; do not use a Realtime audio model. |
| `background_agent_timeout_seconds` | | Per-task timeout. Default: 300 seconds. |

The add-on logs `[LAT]` timings from device `listen:stop` to STT, LLM, TTS start, and first audio. Use these real measurements to compare providers.

### Local history and wake-word standby (standalone)

Open **Provider → 对话记忆与唤醒** in the settings UI to enable text history. Records
are stored per device in local JSON files and can be exported as Markdown or
cleared from the UI. The default retention is 90 days (up to 2000 messages per
device); a new connection includes only the latest 20 messages, capped at 12000
text characters. Set the context message count to 0 for archive-only use.
Recording is off by default; enabling context reuse sends that recent text to
your selected AI provider. This does not save raw audio or automatically recall
every old conversation.

Standalone now defaults to a 15-second silent follow-up window after response
audio has been sent. After it expires, the server closes the audio channel and
standard Xiaozhi firmware returns to its existing local wake-word/button standby.
Silent packets do not keep the channel open. Set `conversation_idle_seconds=0`
to retain unlimited conversation; the HA add-on keeps that legacy default.
Background speech within the window can still trigger a reply. Custom wake words
require firmware support; the server cannot change them by editing the prompt.
See [storage, settings and firmware limitations](docs/conversation-memory-and-wake.md).

### Continuous conversation and background tasks (Beta)

> **Beta:** This path has automated coverage but still needs broad testing with physical StackChan devices, live Home Assistant installations, and different OpenAI-compatible background models.

When enabled, OpenAI Realtime receives tools to create, inspect, and cancel background work. Short HA operations still execute directly. Longer analysis and multi-step operations enter a FIFO queue scoped to the device `Device-Id`, while the realtime voice session acknowledges the work and remains available. State is persisted in `/data/background-tasks.json`. In standalone, enable the optional HA bridge before enabling background tasks.

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

The device firmware needs to know your local server address instead of the Xiaozhi cloud. The server can be a Home Assistant add-on, standalone Docker, or native macOS app. There are two ways to configure the device.

**Guided setup:** Open **设备接入 / NVS 注入 (Device setup)** in the settings sidebar, or use the first-connection banner. HA **Open Web UI** and standalone share this page at the same server version; HA's built-in add-on **Configuration** tab is a separate form. The guide is always available and shows the configured server host, device port, copyable OTA URL, script link, and USB/ESP-IDF steps. It does not flash devices or change network settings. The endpoint is read-only and uses the server configuration, never the browser/Ingress address; a displayed URL is not a connectivity check.

Run the injector on the computer connected to the device by USB, not inside HA or the server container. Enter the displayed host and port separately; do not use the settings port (`8099`) or HA's management port (`8123`). If HA uses `12800` on the same host, give standalone a different published port, such as `12801`. The current add-on advertises a fixed device port of `12800`; keep its host mapping on `12800`. For Docker, change `STACKCHAN_LOCAL_HOST` / `STACKCHAN_WS_PORT` in `.env` and recreate the container. On macOS, quit the app, edit those entries in `~/Library/Application Support/StackChan AI Server/runtime.env`, then reopen it. A device uses one target at a time; enabling the optional standalone HA bridge does not change that target.

> **Which method should I use?**
> Use **Method A (NVS)** for most cases — re-injection is just two commands and doesn't require recompiling.
> Use **Method B (compile)** only if you want to make other firmware customisations at the same time.
>
> ⚠️ **Important:** The current injector and manual method below **replace the entire NVS partition**, including existing Wi-Fi and other NVS settings. Back up NVS first if you need to preserve it, and be prepared to set up Wi-Fi again. If a later reflash or upgrade overwrites NVS, re-inject the server address; not every firmware upgrade necessarily replaces NVS.
>
> 💡 **Shortcut:** Run `python3 flash_nvs.py` for an interactive guided injector that handles all four steps automatically (English / 中文). It accepts the server's LAN IPv4 address or a resolvable local hostname such as `stackchan.local`, plus the reachable TCP port. Use a DHCP reservation when possible; if standalone shares a host with the HA add-on on `12800`, enter standalone's `12801` port.

The device needs an OTA URL as its bootstrap address, so NVS currently must contain the standalone host address and port. The stock firmware path does not automatically discover a local Docker service; mDNS or UDP discovery would require a firmware change and would also need to handle VLANs, firewalls, and multiple servers. We therefore keep explicit NVS configuration as the reliable path.

### Method A — Write NVS key (recommended)

The firmware checks NVS (non-volatile storage) for an OTA URL override before using its hardcoded default. The setting remains until NVS is erased or overwritten; if that happens, run the injector again using the current server address and port.

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
ota_url,data,string,http://<YOUR_SERVER_HOST>:<PUBLIC_WS_PORT>/xiaozhi/ota/
EOF
```
Replace `<YOUR_SERVER_HOST>` with the LAN IPv4 address or a hostname resolvable by the device (for example `192.168.1.100` or `stackchan.local`), and `<PUBLIC_WS_PORT>` with the service's reachable port (`12800` by default; use `STACKCHAN_WS_PORT` for a custom standalone host port). For the add-on the host is normally the Home Assistant host; for standalone it is the Docker host. The NVS injector validates both IPv4 addresses and hostname syntax, but it does not perform service discovery.

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
   - Set it to `http://<YOUR_SERVER_LAN_IP>:<PUBLIC_WS_PORT>/xiaozhi/ota/` (`12800` by default; use `STACKCHAN_WS_PORT` when standalone uses a custom host port)
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
4. Once connected, the device will use the OTA URL you configured (via NVS or menuconfig) to reach your local StackChan AI Server instead of the Xiaozhi cloud

### After a firmware OTA upgrade

Check whether the device still uses your local OTA URL. If the upgrade or reflash erased or overwrote NVS, run the injector again using the current server address and port. Re-injection replaces NVS, so back up needed settings and be prepared to configure Wi-Fi again.

---

## Ports

| Port | Purpose |
|------|---------|
| `12800/tcp` | Main StackChan WebSocket server (OTA discovery + WebSocket AI session) |
| `443/tcp` | Legacy HTTPS intercept (unused) |

---

## License

The current combined project is **AGPL-3.0-only** — see [LICENSE](LICENSE),
[copyright and retained third-party rights](stackchan-server/NOTICE.md), and
[contribution rules](CONTRIBUTING.md). Commercial use is allowed when the license
is followed; there is no mandatory donation, profit share, or upstream PR.

Previously published MIT versions, including HA `v2.8.0-beta.3` and the downloadable
`macos-v0.1.1` DMG, keep their original license. This change applies to the current
source and future applicable versions; it does not replace old release artifacts.
See [licensing and source-distribution guidance](docs/licensing.md).

## Support the project

[Sponsorship is entirely voluntary](SPONSORING.md). It helps fund maintenance,
device/provider testing and packaging. Not donating does not restrict your
licensed rights. [Sponsor via PayPal](https://paypal.me/unitekno), or see the
[sponsorship details](SPONSORING.md#paypal).
