#!/usr/bin/with-contenv bashio

# Read options set via the add-on UI.
HA_MCP_TOKEN=$(bashio::config 'ha_mcp_token')
XIAOZHI_MCP_URL=$(bashio::config 'xiaozhi_mcp_url')
UPSTREAM_OTA=$(bashio::config 'upstream_ota_url')

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
  # HA native WebSocket API — accessible inside the add-on via the supervisor network.
  ha_ws_url: "ws://homeassistant:8123/api/websocket"
  ha_mcp_token: "${HA_MCP_TOKEN}"
  # Xiaozhi MCP relay endpoint (wss://api.xiaozhi.me/mcp/?token=...).
  xiaozhi_mcp_url: "${XIAOZHI_MCP_URL}"
  upstream_ota_url: "${UPSTREAM_OTA}"
EOF

bashio::log.info "Starting StackChan AI server on port 12800"
bashio::log.info "Xiaozhi MCP bridge target: ${XIAOZHI_MCP_URL}"

exec /app/stackchan-server
