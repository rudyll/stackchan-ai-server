# StackChan HA Add-ons

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Home Assistant add-on repository for [StackChan-r](https://github.com/rudyll/StackChan-r) — an AI robot built on M5Stack CoreS3.

## Add-ons

### StackChan AI Server

A local proxy server that sits between your StackChan device and the Xiaozhi AI cloud. It injects Home Assistant tools (via [ha-mcp-for-stackchan](https://github.com/rudyll/ha-mcp-for-stackchan)) into the AI's available toolset, so your robot can control lights, check sensor states, run automations, and more — all through natural conversation.

**How it works:**

```
StackChan device
    ↓  WebSocket
StackChan AI Server  (this add-on, running on HA)
    ↓  proxies to                    ↓  MCP tools
Xiaozhi cloud (AI/LLM)        ha-mcp-for-stackchan
```

1. The device checks in with this server instead of the Xiaozhi cloud directly
2. This server fetches the real AI credentials and forwards the WebSocket connection
3. When the AI asks "what tools are available?", this server appends all your HA tools
4. When the AI calls an HA tool (e.g. turn on lights), this server handles it locally

## Installation

1. In Home Assistant, go to **Settings → Add-ons → Add-on Store**
2. Click the three-dot menu (top right) → **Repositories**
3. Add this URL:
   ```
   https://github.com/rudyll/stackchan_ha_addons
   ```
4. Find **StackChan AI Server** in the store and click **Install**

## Requirements

- [ha-mcp-for-stackchan](https://github.com/rudyll/ha-mcp-for-stackchan) installed and running in Home Assistant
- A StackChan-r device with firmware compiled to point to this server (set `CONFIG_OTA_URL` in `sdkconfig.defaults`)
- A Xiaozhi account with the device registered

## Configuration

| Option | Description |
|--------|-------------|
| `local_host` | LAN IP of this HA instance (e.g. `10.20.20.8`). The device uses this to connect. |
| `ha_mcp_token` | HA Long-Lived Access Token. Create one in **Profile → Security → Long-Lived Access Tokens**. |
| `upstream_ota_url` | Xiaozhi OTA URL. Leave as default unless you self-host the Xiaozhi server. |

## Firmware Setup

After installing the add-on, recompile the StackChan-r firmware with your HA IP in `firmware/sdkconfig.defaults`:

```
CONFIG_OTA_URL="http://10.20.20.8:12800/xiaozhi/ota/"
```

## License

MIT — see [LICENSE](LICENSE)
