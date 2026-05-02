#!/usr/bin/with-contenv bashio

# Read options set via the add-on UI.
LOCAL_HOST=$(bashio::config 'local_host')
HA_MCP_TOKEN=$(bashio::config 'ha_mcp_token')
OPENAI_KEY=$(bashio::config 'openai_api_key')
OPENAI_MODEL=$(bashio::config 'openai_model')
OPENAI_VOICE=$(bashio::config 'openai_tts_voice')
SYSTEM_PROMPT=$(bashio::config 'system_prompt')

# Write GoFrame config.yaml at runtime so the Go server picks it up.
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

bashio::log.info "Starting StackChan AI server on port 12800"
bashio::log.info "Local host advertised to devices: ${LOCAL_HOST}"
bashio::log.info "OpenAI model: ${OPENAI_MODEL}"

exec /app/stackchan-server
