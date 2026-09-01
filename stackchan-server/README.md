# StackChan HA Add-ons

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Home Assistant add-on repository for [StackChan](https://github.com/m5stack/StackChan) — an AI robot built on M5Stack CoreS3.

## Add-ons

### StackChan AI Server

A local AI server that gives your StackChan robot **GPT-4 / Gemini-level voice intelligence with full Home Assistant control** — no Xiaozhi cloud account, no firmware modifications, no intent scripts to maintain.

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
StackChan AI Server  (this add-on, running on HA)
    ├──▶  OpenAI Realtime API  (voice-to-voice, GPT-4o)
    │         or
    ├──▶  Google Gemini Live API  (voice-to-voice, Gemini 2.5)
    │
    └──▶  Home Assistant WebSocket API  (device control, local)
```

1. The device connects to this server instead of the Xiaozhi cloud
2. Voice is streamed end-to-end to OpenAI or Gemini — no separate STT/TTS steps
3. When the AI wants to control a device it calls built-in HA tools: list areas, search entities, list scenes/scripts/automations, call services, get state
4. All HA calls are executed locally via the HA WebSocket API — nothing leaves your network except the AI audio stream

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

The server listens on port `12800`. The settings UI is bound to `127.0.0.1:8099` and requires the Bearer token printed on first startup. Standalone mode does not connect to Home Assistant or start the port-443 OTA interception.

## Device Setup

Flash the official StackChan firmware from [github.com/m5stack/StackChan](https://github.com/m5stack/StackChan) — no firmware modifications needed.

The add-on intercepts the OTA check on port 443 and redirects the device to the local server automatically. Just make sure:

- The device and HA are on the same LAN
- `local_host` in the add-on config is set to the LAN IP of your HA instance

## Configuration

| Option | Description |
|--------|-------------|
| `local_host` | LAN IP of this HA instance (e.g. `192.168.1.100`). The device uses this to connect back. |
| `ha_enabled` | Enable Home Assistant tools and background tasks. The HA add-on defaults to `true`; standalone runtime uses `false`. |
| `ha_mcp_token` | HA Long-Lived Access Token for device control when HA is enabled. Leave empty in standalone mode. |
| `ai_provider` | `openai` or `gemini` |
| `openai_api_key` | OpenAI API key (required when provider is `openai`) |
| `openai_realtime_model` | OpenAI Realtime model to use |
| `openai_tts_voice` | Voice for OpenAI TTS output |
| `gemini_api_key` | Google AI API key (required when provider is `gemini`) |
| `gemini_model` | Gemini Live model to use |
| `gemini_voice` | Voice for Gemini audio output |
| `gemini_enable_tools` | Enable HA device control tools for Gemini (default: on). |
| `gemini_enable_search` | Enable Google Search grounding for Gemini (default: off). **⚠️ Mutually exclusive with `gemini_enable_tools`** — Gemini does not support grounding and function calling at the same time. Enabling both will cause connection errors (1011). To use web search, set `gemini_enable_tools=false`. |
| `system_prompt` | System prompt sent to the AI. Controls language, personality, and control behaviour. |

## License

MIT — see [LICENSE](LICENSE)
