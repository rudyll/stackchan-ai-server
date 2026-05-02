#!/bin/bash
# HA supervisor always writes add-on options to /data/options.json.
# Read with jq — no bashio dependency needed.

OPTIONS=/data/options.json

get() { jq -r --arg k "$1" --arg d "$2" '.[$k] // $d' "$OPTIONS"; }

LOCAL_HOST=$(get local_host "127.0.0.1")
HA_MCP_TOKEN=$(get ha_mcp_token "")
OPENAI_KEY=$(get openai_api_key "")
OPENAI_MODEL=$(get openai_model "gpt-4o-mini")
OPENAI_VOICE=$(get openai_tts_voice "alloy")
SYSTEM_PROMPT=$(get system_prompt "You are StackChan, a friendly desktop robot assistant. Keep replies concise.")

mkdir -p /app/manifest/config
cat > /app/manifest/config/config.yaml <<EOF
server:
  address: ":12800"

logger:
  stdout: true
  level: "info"

database:
  default:
    link: ""

jwt:
  secret: ""

admin:
  users: []

rsa:
  server:
    public:
    private:
  client:
    public:
    private:

xiaozhi:
  secret_key:
  generate_license_token:

ai:
  local_host: "${LOCAL_HOST}"
  local_port: 12800
  ha_ws_url: "ws://homeassistant:8123/api/websocket"
  ha_mcp_token: "${HA_MCP_TOKEN}"
  openai_api_key: "${OPENAI_KEY}"
  openai_model: "${OPENAI_MODEL}"
  openai_tts_voice: "${OPENAI_VOICE}"
  system_prompt: "${SYSTEM_PROMPT}"
EOF

echo "INFO: Starting StackChan AI server on :12800"
echo "INFO: local_host=${LOCAL_HOST}  model=${OPENAI_MODEL}"

exec /app/stackchan-server
