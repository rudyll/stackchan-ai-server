#!/usr/bin/with-contenv bashio

# Read options set via the add-on UI.
LOCAL_HOST=$(bashio::config 'local_host')
HA_MCP_TOKEN=$(bashio::config 'ha_mcp_token')
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
  local_host: "${LOCAL_HOST}"
  local_port: 12800
  # HA WebSocket MCP server — accessible inside the add-on via the supervisor network.
  ha_mcp_url: "ws://homeassistant:8123/api/ws_mcp_server/ws"
  ha_mcp_token: "${HA_MCP_TOKEN}"
  upstream_ota_url: "${UPSTREAM_OTA}"
EOF

bashio::log.info "Starting StackChan AI server on port 12800"
bashio::log.info "Local host advertised to devices: ${LOCAL_HOST}"

exec /app/stackchan-server
