# StackChan AI Server

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

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

The device's audio and WebSocket traffic goes directly to StackChan AI Server; Home Assistant is not a transport relay. When HA is enabled, the server makes a separate local HA WebSocket API connection only for smart-home control. Standalone mode has no HA hop.

1. The device connects to this server instead of the Xiaozhi cloud
2. Voice is streamed end-to-end to OpenAI or Gemini — no separate STT/TTS steps
3. When HA is enabled and the AI wants to control a device, it calls built-in HA tools: list areas, search entities, list scenes/scripts/automations, call services, get state
4. All HA calls are executed locally via the HA WebSocket API; standalone mode omits them

## Installation

1. In Home Assistant, go to **Settings → Add-ons → Add-on Store**
2. Click the three-dot menu (top right) → **Repositories**
3. Add this URL:
   ```
   https://github.com/rudyll/stackchan-ai-server
   ```
4. Find **StackChan AI Server** in the store and click **Install**

### Standalone Docker (beta)

Copy `.env.standalone.example` to `.env`, set `STACKCHAN_LOCAL_HOST` to the Docker host's LAN IP, add an AI API key, and run:

```bash
cp .env.standalone.example .env
docker compose -f docker-compose.standalone.yml up --build -d
```

By default, the server is published on host port `12800` (the container listens on `12800`). The settings UI is bound to `127.0.0.1:8099`; open `http://127.0.0.1:8099/` in a browser and enter the Bearer token printed on first startup. It then uses a short-lived HttpOnly session cookie, while API requests remain protected. It provides separate entries for OpenAI Realtime, Gemini Live, TokenHub, OpenRouter, and OpenAI-compatible providers, plus model discovery, provider-specific voice catalogs, and Gemini HA-tools/Search toggles. Standalone mode does not connect to Home Assistant or start the port-443 OTA interception.

If the HA add-on and standalone Docker share one host, keep the add-on on host port `12800` and set `STACKCHAN_WS_PORT=12801` (and optionally `STACKCHAN_SETTINGS_PORT=8100`) in `.env`. Configure the device's standalone OTA URL as `http://<server-LAN-IP>:12801/xiaozhi/ota/`. The container still listens internally on `12800`; the launcher advertises the selected public port in the returned WebSocket URL.

Each device has one active OTA/WebSocket target. Devices configured with the HA add-on URL connect to the add-on; devices configured with the standalone URL connect to standalone. Without an NVS override or a compiled `OTA_URL`, stock firmware will not discover standalone automatically.

## Device Setup

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
