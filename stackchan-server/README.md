# StackChan AI Server

<p align="center"><img src="https://raw.githubusercontent.com/rudyll/stackchan-ai-server/main/stackchan-server/logo.png" alt="StackChan with a hand-drawn crown and angel wings" width="160" height="160"></p>

[![License: AGPL-3.0-only](https://img.shields.io/badge/License-AGPL--3.0--only-blue.svg)](LICENSE)

Current combined source: **AGPL-3.0-only**, with original third-party permissions
retained in [NOTICE](NOTICE.md). Previously published MIT releases, including
`v2.8.0-beta.3` and `macos-v0.1.1`, keep their original license.
[Sponsorship is voluntary / 赞助完全自愿](../SPONSORING.md); no confirmed payment
destination is published yet. [Contribution rules](../CONTRIBUTING.md).

Home Assistant add-on and standalone Docker runtime for [StackChan](https://github.com/m5stack/StackChan) — an AI robot built on M5Stack CoreS3.

## Distributions

### StackChan AI Server

A self-hosted AI server that gives your StackChan robot **GPT-4 / Gemini-level voice intelligence**, with optional Home Assistant control — no Xiaozhi cloud account or intent scripts to maintain.

### Why not just use HA Assist?

| | **StackChan AI Server** | **HA Assist** |
|---|---|---|
| **Understanding** | GPT-4o / Gemini 2.5 — understands natural, conversational speech | Rules-based intent matching — only recognises pre-defined phrases |
| **Conversation** | Full multi-turn context across the whole session | Stateless — every utterance is independent, no memory of what was just said |
| **Ambiguous commands** | Asks one clarifying question, then acts (e.g. "好热" → "哪个房间？" → turns on AC) | Fails or picks a random device if the command doesn't match a pattern exactly |
| **Multiple devices** | Names the matches and asks "which one, or all?" | No disambiguation — controls all or errors out |
| **Voice quality** | Real-time neural audio (OpenAI Realtime / Gemini Live) — natural, low-latency | TTS pipeline with noticeable STT→LLM→TTS delay |
| **Setup** | One system prompt, no scripts | Requires defining intents and scripts per device action |
| **Scenes / Scripts / Automations** | Searches and activates by name automatically | Only if you write a matching intent |

**How it works:**

```
StackChan device (official firmware, unmodified)
    ↓  Xiaozhi WebSocket protocol (port 12800)
StackChan AI Server  (HA add-on or standalone Docker, port 12800)
    ├──▶  OpenAI Realtime API  (voice-to-voice, GPT-4o)
    │         or
    ├──▶  Google Gemini Live API  (voice-to-voice, Gemini 2.5)
    │
    └──▶  Home Assistant WebSocket API  (device control, local)
```

The device's audio and WebSocket traffic goes directly to StackChan AI Server; Home Assistant is not a transport relay. When HA is enabled, the server makes a separate authenticated HA WebSocket API connection only for smart-home control. Standalone can optionally use the same direct HA bridge; without that opt-in, it has no HA connection.

1. The device connects to this server instead of the Xiaozhi cloud
2. Voice is streamed end-to-end to OpenAI or Gemini — no separate STT/TTS steps
3. When HA is enabled and the AI wants to control a device, it calls built-in HA tools: list areas, search entities, list scenes/scripts/automations, call services, get state
4. All HA calls are executed locally via the HA WebSocket API; standalone uses them only when its optional HA bridge is enabled

## Installation

1. In Home Assistant, go to **Settings → Add-ons → Add-on Store**
2. Click the three-dot menu (top right) → **Repositories**
3. Add this URL:
   ```
   https://github.com/rudyll/stackchan-ai-server
   ```
4. Find **StackChan AI Server** in the store and click **Install**

For existing installations, back up first, refresh the store, and update to
[2.8.0-beta.3](https://github.com/rudyll/stackchan-ai-server/releases/tag/v2.8.0-beta.3).
Restart and open **Web UI** for the shared settings, permanent NVS guide and new
artwork. Keep your options and data. See the [add-on changelog](CHANGELOG.md).

### Standalone Docker (beta)

Copy `.env.standalone.example` to `.env`, set `STACKCHAN_LOCAL_HOST` to the Docker host's LAN IP, add an AI API key, and run:

```bash
cp .env.standalone.example .env
docker compose -f docker-compose.standalone.yml up --build -d
```

To update Docker, fetch the latest source, preserve `.env` and the mounted data
directory, then rerun the command above. This is a source-built distribution;
there is no prebuilt registry image to pull.

If `STACKCHAN_SETTINGS_TOKEN` is empty, retrieve the generated token from the first-start log before opening the GUI:

```bash
docker compose -f docker-compose.standalone.yml logs --no-color --tail=50 stackchan
```

By default, the server is published on host port `12800` (the container listens on `12800`). The settings UI is published on host `127.0.0.1:8099` by default; if `STACKCHAN_SETTINGS_PORT` is set, open that host port instead (the container still listens on `8099`) and enter the Bearer token printed on first startup. It then uses a short-lived HttpOnly session cookie, while API requests remain protected. It provides separate entries for OpenAI Realtime, Gemini Live, TokenHub, OpenRouter, and OpenAI-compatible providers, plus model discovery, provider-specific voice catalogs, and Gemini HA-tools/Search toggles. Standalone defaults to no Home Assistant connection. To control entities, enable the optional bridge in the GUI and enter the HA URL plus a Long-Lived Access Token; the server then connects directly to HA's Core WebSocket API. Standalone mode does not start the port-443 OTA interception.

If the HA add-on and standalone Docker share one host, keep the add-on on host port `12800` and set `STACKCHAN_WS_PORT=12801` (and optionally `STACKCHAN_SETTINGS_PORT=8100`) in `.env`. Configure the device's standalone OTA URL as `http://<server-LAN-IP>:12801/xiaozhi/ota/`. The container still listens internally on `12800`; the launcher advertises the selected public port in the returned WebSocket URL.

Each device has one active OTA/WebSocket target. Devices configured with the HA add-on URL connect to the add-on; devices configured with the standalone URL connect to standalone. Without an NVS override or a compiled `OTA_URL`, stock firmware will not discover standalone automatically.

The optional `STACKCHAN_STANDALONE_HA_ENABLED`, `STACKCHAN_STANDALONE_HA_URL`, and `STACKCHAN_STANDALONE_HA_TOKEN` variables can preconfigure the direct HA bridge. The optional `STACKCHAN_DEVICE_PROFILES`, `STACKCHAN_SYSTEM_PROMPT`, `STACKCHAN_AUDIO_PREBUFFER_MS`, and `STACKCHAN_AUDIO_PREBUFFER_MAX_WAIT_MS` variables are first-start defaults. Prefer the GUI for later edits; saved values persist in the mounted data directory.

### macOS standalone DMG (preview)

Download the [ready-made universal DMG](https://github.com/rudyll/stackchan-ai-server/releases/download/macos-v0.1.1/StackChan-AI-Server-0.1.1-macos-universal.dmg) from the [macOS 0.1.1 Release](https://github.com/rudyll/stackchan-ai-server/releases/tag/macos-v0.1.1). Drag the app into Applications; no user build, Docker, Homebrew or audio-library installation is needed. Apple Silicon and Intel executables target macOS 12+. The new 3D StackChan icon appears in both the app and shared settings UI. This preview is ad-hoc signed, not Developer ID signed or notarized. See [installation, security and upgrade instructions](macos/README.md).

## Device Setup

Open **设备接入 / NVS 注入 (Device setup)** in the settings sidebar for a permanent guide with the current host, device port, copyable OTA URL, script link, and USB/ESP-IDF steps. HA **Open Web UI** and standalone use the same page at the same server version; HA's built-in add-on **Configuration** tab remains separate. The guide neither flashes devices nor changes network settings, and does not infer the device address from the browser URL. A shown address is not a reachability check.

Run `flash_nvs.py` on the USB-connected computer. Enter host and port separately; use the device-facing port, not settings port `8099` or HA port `8123`. If HA occupies host port `12800`, standalone can use `12801`. Keep the add-on's host mapping on `12800`, its current fixed advertised port. Change Docker's `.env` (`STACKCHAN_LOCAL_HOST` / `STACKCHAN_WS_PORT`) and recreate the container; for macOS, quit the app, edit the same entries in `~/Library/Application Support/StackChan AI Server/runtime.env`, and reopen it. **The injector replaces the whole NVS partition**, including Wi-Fi and other settings. Back up needed NVS data before writing and be prepared to set up Wi-Fi again. See the [English](../README.md#firmware-setup) / [中文](../README.zh.md#固件配置) instructions. Re-inject after a reflash or upgrade only if the override was erased or overwritten.

Standalone also supports optional local text history (JSON with Markdown export)
and a 15-second silent follow-up window before returning the device to firmware
wake-word/button standby. Configure these in **Provider → 对话记忆与唤醒**. History
recording defaults off; when enabled, only bounded recent context is sent to the
AI provider. See [details and firmware limitations](../docs/conversation-memory-and-wake.md).

Flash the official StackChan firmware from [github.com/m5stack/StackChan](https://github.com/m5stack/StackChan) — no firmware modifications needed.

The HA add-on intercepts the OTA check on port 443 and redirects the device to the local server automatically. Standalone Docker uses the NVS OTA URL or compiled firmware method described in the root README. For the add-on, make sure:

- The device and HA are on the same LAN
- `local_host` in the add-on config is set to the LAN IP of your HA instance

## Configuration

| Option | Description |
|--------|-------------|
| `local_host` | LAN IP of the host running StackChan AI Server (e.g. `192.168.1.100`). For the add-on this is normally the HA host; for standalone it is the Docker host. |
| `ha_enabled` | Enable Home Assistant tools and background tasks. The HA add-on defaults to `true`; standalone runtime uses `false`. |
| `ha_mcp_token` | HA Long-Lived Access Token for device control when HA is enabled. Leave empty in standalone mode. |
| `standalone_ha_enabled` | Opt in to a direct Home Assistant connection from standalone. Default: `false`. |
| `standalone_ha_url` | Home Assistant URL for the standalone bridge; `/api/websocket` is added when omitted. |
| `standalone_ha_token` | HA Long-Lived Access Token for the standalone bridge; never returned by the settings API. |
| `ai_provider` | `openai`, `gemini`, `tokenhub`, `openrouter`, or `openai_compatible` |
| `openai_api_key` | OpenAI API key (required when provider is `openai`) |
| `openai_realtime_model` | OpenAI Realtime model to use |
| `openai_tts_voice` | Voice for OpenAI TTS output |
| `gemini_api_key` | Google AI API key (required when provider is `gemini`) |
| `gemini_model` | Gemini Live model to use |
| `gemini_voice` | Voice for Gemini audio output |
| `gemini_enable_tools` | Enable HA device control tools for Gemini. Add-on default: on; standalone default: off. |
| `gemini_enable_search` | Enable Google Search grounding for Gemini (default: off). **⚠️ Mutually exclusive with `gemini_enable_tools`** — Gemini does not support grounding and function calling at the same time. Enabling both will cause connection errors (1011). To use web search, set `gemini_enable_tools=false`. |
| `system_prompt` | System prompt sent to the AI. Controls language, personality, and control behaviour. |

## License

MIT — see [LICENSE](LICENSE)
