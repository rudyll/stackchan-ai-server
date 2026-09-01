#!/bin/bash
set -eu

# Standalone runtime configuration. Secrets are supplied through environment
# variables and the generated file is kept inside the container filesystem.
# User-managed settings and the settings token live in STACKCHAN_DATA_DIR.
umask 077

DATA_DIR="${STACKCHAN_DATA_DIR:-/data}"
mkdir -p "$DATA_DIR"
chmod 700 "$DATA_DIR"

TOKEN_FILE="$DATA_DIR/settings-token"
SETTINGS_AUTH_TOKEN="${STACKCHAN_SETTINGS_TOKEN:-}"
if [ -z "$SETTINGS_AUTH_TOKEN" ]; then
	if [ -s "$TOKEN_FILE" ]; then
		SETTINGS_AUTH_TOKEN=$(tr -d '\r\n' < "$TOKEN_FILE")
	else
		SETTINGS_AUTH_TOKEN=$(head -c 32 /dev/urandom | base64 | tr -d '\r\n=')
		printf '%s' "$SETTINGS_AUTH_TOKEN" > "$TOKEN_FILE"
		chmod 600 "$TOKEN_FILE"
		echo "INFO: generated settings token (save it now): $SETTINGS_AUTH_TOKEN"
	fi
fi

yaml_string() {
	local value="$1"
	value=${value//\\/\\\\}
	value=${value//\"/\\\"}
	value=${value//$'\n'/\\n}
	printf '"%s"' "$value"
}

bool_value() {
	case "${1:-}" in
		true|false) printf '%s' "$1" ;;
		*) printf '%s' "$2" ;;
	esac
}

int_value() {
	case "${1:-}" in
		''|*[!0-9-]*) printf '%s' "$2" ;;
		*) printf '%s' "$1" ;;
	esac
}

port_value() {
	local value="${1:-}"
	case "$value" in
		''|*[!0-9]*|??????*) printf '%s' "$2"; return ;;
		*) ;;
	esac
	if [ "$value" -lt 1 ] || [ "$value" -gt 65535 ]; then
		printf '%s' "$2"
	else
		printf '%s' "$value"
	fi
}

LOCAL_HOST="${STACKCHAN_LOCAL_HOST:?Set STACKCHAN_LOCAL_HOST to the Docker host LAN IP}"
WS_PORT=$(port_value "${STACKCHAN_WS_PORT:-}" 12800)
AI_PROVIDER="${STACKCHAN_AI_PROVIDER:-openai}"
OPENAI_KEY="${STACKCHAN_OPENAI_API_KEY:-}"
OPENAI_RT_MODEL="${STACKCHAN_OPENAI_REALTIME_MODEL:-gpt-realtime}"
OPENAI_VOICE="${STACKCHAN_OPENAI_TTS_VOICE:-alloy}"
GEMINI_KEY="${STACKCHAN_GEMINI_API_KEY:-}"
GEMINI_MODEL="${STACKCHAN_GEMINI_MODEL:-gemini-2.5-flash-native-audio-latest}"
GEMINI_VOICE="${STACKCHAN_GEMINI_VOICE:-Aoede}"
GEMINI_ENABLE_TOOLS=$(bool_value "${STACKCHAN_GEMINI_ENABLE_TOOLS:-}" false)
GEMINI_ENABLE_SEARCH=$(bool_value "${STACKCHAN_GEMINI_ENABLE_SEARCH:-}" false)
COMPATIBLE_BASE_URL="${STACKCHAN_COMPATIBLE_BASE_URL:-}"
COMPATIBLE_API_KEY="${STACKCHAN_COMPATIBLE_API_KEY:-}"
COMPATIBLE_MODEL="${STACKCHAN_COMPATIBLE_MODEL:-}"
COMPATIBLE_STT_MODEL="${STACKCHAN_COMPATIBLE_STT_MODEL:-whisper-1}"
COMPATIBLE_TTS_MODEL="${STACKCHAN_COMPATIBLE_TTS_MODEL:-tts-1}"
COMPATIBLE_TTS_VOICE="${STACKCHAN_COMPATIBLE_TTS_VOICE:-alloy}"
TOKENHUB_BASE_URL="${STACKCHAN_TOKENHUB_BASE_URL:-}"
TOKENHUB_API_KEY="${STACKCHAN_TOKENHUB_API_KEY:-}"
OPENROUTER_API_KEY="${STACKCHAN_OPENROUTER_API_KEY:-}"
STT_BASE_URL="${STACKCHAN_STT_BASE_URL:-}"
STT_API_KEY="${STACKCHAN_STT_API_KEY:-}"
STT_MODEL="${STACKCHAN_STT_MODEL:-}"
LLM_BASE_URL="${STACKCHAN_LLM_BASE_URL:-}"
LLM_API_KEY="${STACKCHAN_LLM_API_KEY:-}"
LLM_MODEL="${STACKCHAN_LLM_MODEL:-}"
TTS_BASE_URL="${STACKCHAN_TTS_BASE_URL:-}"
TTS_API_KEY="${STACKCHAN_TTS_API_KEY:-}"
TTS_MODEL="${STACKCHAN_TTS_MODEL:-}"
TTS_VOICE="${STACKCHAN_TTS_VOICE:-}"
AUDIO_PREBUFFER_MS=$(int_value "${STACKCHAN_AUDIO_PREBUFFER_MS:-}" 300)
AUDIO_PREBUFFER_MAX_WAIT_MS=$(int_value "${STACKCHAN_AUDIO_PREBUFFER_MAX_WAIT_MS:-}" 900)
BACKGROUND_TASKS_ENABLED=$(bool_value "${STACKCHAN_BACKGROUND_TASKS_ENABLED:-}" false)
BACKGROUND_AGENT_BASE_URL="${STACKCHAN_BACKGROUND_AGENT_BASE_URL:-https://api.openai.com}"
BACKGROUND_AGENT_API_KEY="${STACKCHAN_BACKGROUND_AGENT_API_KEY:-}"
BACKGROUND_AGENT_MODEL="${STACKCHAN_BACKGROUND_AGENT_MODEL:-}"
BACKGROUND_AGENT_TIMEOUT_SECONDS=$(int_value "${STACKCHAN_BACKGROUND_AGENT_TIMEOUT_SECONDS:-}" 300)
DEVICE_PROFILES="${STACKCHAN_DEVICE_PROFILES:-{}}"
SYSTEM_PROMPT="${STACKCHAN_SYSTEM_PROMPT:-You are StackChan, a friendly desktop robot assistant. Keep replies concise.}"

DEVICE_PROFILES_B64=$(printf %s "$DEVICE_PROFILES" | base64 | tr -d '\r\n')
SYSTEM_PROMPT_B64=$(printf %s "$SYSTEM_PROMPT" | base64 | tr -d '\r\n')

mkdir -p /app/manifest/config
cat > /app/manifest/config/config.yaml <<EOF
server:
  address: ":12800"

logger:
  stdout: true
  level: "debug"

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
  local_host: $(yaml_string "$LOCAL_HOST")
  local_port: ${WS_PORT}
  ha_enabled: false
  ota_https_enabled: false
  settings_listen_address: ":8099"
  settings_auth_token: $(yaml_string "$SETTINGS_AUTH_TOKEN")
  provider: $(yaml_string "$AI_PROVIDER")
  openai_api_key: $(yaml_string "$OPENAI_KEY")
  openai_realtime_model: $(yaml_string "$OPENAI_RT_MODEL")
  openai_tts_voice: $(yaml_string "$OPENAI_VOICE")
  gemini_api_key: $(yaml_string "$GEMINI_KEY")
  gemini_model: $(yaml_string "$GEMINI_MODEL")
  gemini_voice: $(yaml_string "$GEMINI_VOICE")
  gemini_enable_tools: ${GEMINI_ENABLE_TOOLS}
  gemini_enable_search: ${GEMINI_ENABLE_SEARCH}
  compatible_base_url: $(yaml_string "$COMPATIBLE_BASE_URL")
  compatible_api_key: $(yaml_string "$COMPATIBLE_API_KEY")
  compatible_model: $(yaml_string "$COMPATIBLE_MODEL")
  compatible_stt_model: $(yaml_string "$COMPATIBLE_STT_MODEL")
  compatible_tts_model: $(yaml_string "$COMPATIBLE_TTS_MODEL")
  compatible_tts_voice: $(yaml_string "$COMPATIBLE_TTS_VOICE")
  tokenhub_base_url: $(yaml_string "$TOKENHUB_BASE_URL")
  tokenhub_api_key: $(yaml_string "$TOKENHUB_API_KEY")
  openrouter_api_key: $(yaml_string "$OPENROUTER_API_KEY")
  stt_base_url: $(yaml_string "$STT_BASE_URL")
  stt_api_key: $(yaml_string "$STT_API_KEY")
  stt_model: $(yaml_string "$STT_MODEL")
  llm_base_url: $(yaml_string "$LLM_BASE_URL")
  llm_api_key: $(yaml_string "$LLM_API_KEY")
  llm_model: $(yaml_string "$LLM_MODEL")
  tts_base_url: $(yaml_string "$TTS_BASE_URL")
  tts_api_key: $(yaml_string "$TTS_API_KEY")
  tts_model: $(yaml_string "$TTS_MODEL")
  tts_voice: $(yaml_string "$TTS_VOICE")
  audio_prebuffer_ms: ${AUDIO_PREBUFFER_MS}
  audio_prebuffer_max_wait_ms: ${AUDIO_PREBUFFER_MAX_WAIT_MS}
  background_tasks_enabled: ${BACKGROUND_TASKS_ENABLED}
  background_agent_base_url: $(yaml_string "$BACKGROUND_AGENT_BASE_URL")
  background_agent_api_key: $(yaml_string "$BACKGROUND_AGENT_API_KEY")
  background_agent_model: $(yaml_string "$BACKGROUND_AGENT_MODEL")
  background_agent_timeout_seconds: ${BACKGROUND_AGENT_TIMEOUT_SECONDS}
  device_profiles_b64: $(yaml_string "$DEVICE_PROFILES_B64")
  system_prompt_b64: $(yaml_string "$SYSTEM_PROMPT_B64")
EOF

echo "INFO: Starting standalone StackChan AI server on :12800 (public port ${WS_PORT})"
echo "INFO: local_host=${LOCAL_HOST}  provider=${AI_PROVIDER}  settings_ui=127.0.0.1:8099"

exec /app/stackchan-server
