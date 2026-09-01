#!/bin/bash
# HA supervisor always writes add-on options to /data/options.json.
# Read with jq — no bashio dependency needed.

OPTIONS=/data/options.json

get() { jq -r --arg k "$1" --arg d "$2" '.[$k] // $d' "$OPTIONS"; }

LOCAL_HOST=$(get local_host "127.0.0.1")
HA_ENABLED=$(get ha_enabled "true")
HA_MCP_TOKEN=$(get ha_mcp_token "")
AI_PROVIDER=$(get ai_provider "openai")
OPENAI_KEY=$(get openai_api_key "")
OPENAI_RT_MODEL=$(get openai_realtime_model "gpt-realtime")
OPENAI_VOICE=$(get openai_tts_voice "alloy")
GEMINI_KEY=$(get gemini_api_key "")
GEMINI_MODEL=$(get gemini_model "gemini-2.5-flash-native-audio-latest")
GEMINI_VOICE=$(get gemini_voice "Aoede")
GEMINI_ENABLE_TOOLS=$(get gemini_enable_tools "true")
COMPATIBLE_BASE_URL=$(get compatible_base_url "")
COMPATIBLE_API_KEY=$(get compatible_api_key "")
COMPATIBLE_MODEL=$(get compatible_model "")
COMPATIBLE_STT_MODEL=$(get compatible_stt_model "whisper-1")
COMPATIBLE_TTS_MODEL=$(get compatible_tts_model "tts-1")
COMPATIBLE_TTS_VOICE=$(get compatible_tts_voice "alloy")
TOKENHUB_BASE_URL=$(get tokenhub_base_url "")
TOKENHUB_API_KEY=$(get tokenhub_api_key "")
OPENROUTER_API_KEY=$(get openrouter_api_key "")
STT_BASE_URL=$(get stt_base_url "")
STT_API_KEY=$(get stt_api_key "")
STT_MODEL=$(get stt_model "")
LLM_BASE_URL=$(get llm_base_url "")
LLM_API_KEY=$(get llm_api_key "")
LLM_MODEL=$(get llm_model "")
TTS_BASE_URL=$(get tts_base_url "")
TTS_API_KEY=$(get tts_api_key "")
TTS_MODEL=$(get tts_model "")
TTS_VOICE=$(get tts_voice "")
AUDIO_PREBUFFER_MS=$(get audio_prebuffer_ms "300")
AUDIO_PREBUFFER_MAX_WAIT_MS=$(get audio_prebuffer_max_wait_ms "900")
BACKGROUND_TASKS_ENABLED=$(get background_tasks_enabled "false")
BACKGROUND_AGENT_BASE_URL=$(get background_agent_base_url "https://api.openai.com")
BACKGROUND_AGENT_API_KEY=$(get background_agent_api_key "")
BACKGROUND_AGENT_MODEL=$(get background_agent_model "")
BACKGROUND_AGENT_TIMEOUT_SECONDS=$(get background_agent_timeout_seconds "300")
DEVICE_PROFILES=$(get device_profiles "{}")
DEVICE_PROFILES_B64=$(printf %s "$DEVICE_PROFILES" | base64 | tr -d '\n')
SYSTEM_PROMPT=$(get system_prompt "You are StackChan, a friendly desktop robot assistant. Keep replies concise.")
SYSTEM_PROMPT_B64=$(printf %s "$SYSTEM_PROMPT" | base64 | tr -d '\n')

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
  local_host: "${LOCAL_HOST}"
  local_port: 12800
  ha_enabled: ${HA_ENABLED}
  ota_https_enabled: true
  settings_listen_address: ":8099"
  settings_auth_token: ""
  ha_ws_url: "ws://homeassistant:8123/api/websocket"
  ha_mcp_token: "${HA_MCP_TOKEN}"
  provider: "${AI_PROVIDER}"
  openai_api_key: "${OPENAI_KEY}"
  openai_realtime_model: "${OPENAI_RT_MODEL}"
  openai_tts_voice: "${OPENAI_VOICE}"
  gemini_api_key: "${GEMINI_KEY}"
  gemini_model: "${GEMINI_MODEL}"
  gemini_voice: "${GEMINI_VOICE}"
  gemini_enable_tools: ${GEMINI_ENABLE_TOOLS}
  compatible_base_url: "${COMPATIBLE_BASE_URL}"
  compatible_api_key: "${COMPATIBLE_API_KEY}"
  compatible_model: "${COMPATIBLE_MODEL}"
  compatible_stt_model: "${COMPATIBLE_STT_MODEL}"
  compatible_tts_model: "${COMPATIBLE_TTS_MODEL}"
  compatible_tts_voice: "${COMPATIBLE_TTS_VOICE}"
  tokenhub_base_url: "${TOKENHUB_BASE_URL}"
  tokenhub_api_key: "${TOKENHUB_API_KEY}"
  openrouter_api_key: "${OPENROUTER_API_KEY}"
  stt_base_url: "${STT_BASE_URL}"
  stt_api_key: "${STT_API_KEY}"
  stt_model: "${STT_MODEL}"
  llm_base_url: "${LLM_BASE_URL}"
  llm_api_key: "${LLM_API_KEY}"
  llm_model: "${LLM_MODEL}"
  tts_base_url: "${TTS_BASE_URL}"
  tts_api_key: "${TTS_API_KEY}"
  tts_model: "${TTS_MODEL}"
  tts_voice: "${TTS_VOICE}"
  audio_prebuffer_ms: ${AUDIO_PREBUFFER_MS}
  audio_prebuffer_max_wait_ms: ${AUDIO_PREBUFFER_MAX_WAIT_MS}
  background_tasks_enabled: ${BACKGROUND_TASKS_ENABLED}
  background_agent_base_url: "${BACKGROUND_AGENT_BASE_URL}"
  background_agent_api_key: "${BACKGROUND_AGENT_API_KEY}"
  background_agent_model: "${BACKGROUND_AGENT_MODEL}"
  background_agent_timeout_seconds: ${BACKGROUND_AGENT_TIMEOUT_SECONDS}
  device_profiles_b64: "${DEVICE_PROFILES_B64}"
  system_prompt_b64: "${SYSTEM_PROMPT_B64}"
EOF

echo "INFO: Starting StackChan AI server on :12800"
echo "INFO: local_host=${LOCAL_HOST}  provider=${AI_PROVIDER}"

exec /app/stackchan-server
