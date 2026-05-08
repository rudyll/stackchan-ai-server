# StackChan HA Add-ons

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Home Assistant add-on repository for [StackChan](https://github.com/m5stack/StackChan) — an AI robot built on M5Stack CoreS3.

## Add-ons

### StackChan AI Server

A local AI server that connects your StackChan device to OpenAI Realtime or Google Gemini Live, with built-in Home Assistant device control. No Xiaozhi cloud account required — the add-on speaks the Xiaozhi WebSocket protocol directly so the official StackChan firmware works unchanged.

**How it works:**

```
StackChan device (official firmware, unmodified)
    ↓  Xiaozhi WebSocket protocol (port 12800)
StackChan AI Server  (this add-on, running on HA)
    ├──▶  OpenAI Realtime API  (voice-to-voice)
    │         or
    ├──▶  Google Gemini Live API  (voice-to-voice)
    │
    └──▶  Home Assistant WebSocket API  (device control)
```

1. The device connects to this server instead of the Xiaozhi cloud
2. Voice is streamed to OpenAI or Gemini for real-time conversation
3. When the AI wants to control a device it calls built-in HA tools (list areas, search entities, call services, get state)
4. HA tool calls are executed locally via the HA WebSocket API

## Installation

1. In Home Assistant, go to **Settings → Add-ons → Add-on Store**
2. Click the three-dot menu (top right) → **Repositories**
3. Add this URL:
   ```
   https://github.com/rudyll/stackchan_ha_addons
   ```
4. Find **StackChan AI Server** in the store and click **Install**

## Device Setup

Flash the official StackChan firmware from [github.com/m5stack/StackChan](https://github.com/m5stack/StackChan) — no firmware modifications needed.

The add-on intercepts the OTA check on port 443 and redirects the device to the local server automatically. Just make sure:

- The device and HA are on the same LAN
- `local_host` in the add-on config is set to the LAN IP of your HA instance

## Configuration

| Option | Description |
|--------|-------------|
| `local_host` | LAN IP of this HA instance (e.g. `10.20.20.8`). The device uses this to connect back. |
| `ha_mcp_token` | HA Long-Lived Access Token for device control. Create one in **Profile → Security → Long-Lived Access Tokens**. |
| `ai_provider` | `openai` or `gemini` |
| `openai_api_key` | OpenAI API key (required when provider is `openai`) |
| `openai_realtime_model` | OpenAI Realtime model to use |
| `openai_tts_voice` | Voice for OpenAI TTS output |
| `gemini_api_key` | Google AI API key (required when provider is `gemini`) |
| `gemini_model` | Gemini Live model to use |
| `gemini_voice` | Voice for Gemini audio output |
| `gemini_enable_tools` | Enable HA tool calling for Gemini (disable to bisect issues) |
| `system_prompt` | System prompt sent to the AI at the start of each session |

## License

MIT — see [LICENSE](LICENSE)
