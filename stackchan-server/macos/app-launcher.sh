#!/bin/bash
set -eu

APP_ROOT=$(cd "$(dirname "$0")/.." && pwd)
RESOURCES="$APP_ROOT/Resources"
DATA_DIR="${STACKCHAN_DATA_DIR:-$HOME/Library/Application Support/StackChan AI Server}"
RUNTIME_FILE="$DATA_DIR/runtime.env"
TOKEN_FILE="$DATA_DIR/settings-token"
CONFIG_DIR="$DATA_DIR/config"
CONFIG_PATH="$CONFIG_DIR/config.yaml"

mkdir -p "$CONFIG_DIR" "$DATA_DIR/logs"
chmod 700 "$DATA_DIR"

valid_port() {
	case "$1" in
		''|*[!0-9]*) return 1 ;;
		*) [ "$1" -ge 1 ] && [ "$1" -le 65535 ] ;;
	esac
}

read_runtime() {
	[ -r "$RUNTIME_FILE" ] || return 0
	case "$1" in
		STACKCHAN_WS_PORT) sed -n 's/^STACKCHAN_WS_PORT=//p' "$RUNTIME_FILE" | head -n 1 ;;
		STACKCHAN_SETTINGS_PORT) sed -n 's/^STACKCHAN_SETTINGS_PORT=//p' "$RUNTIME_FILE" | head -n 1 ;;
		STACKCHAN_LOCAL_HOST) sed -n 's/^STACKCHAN_LOCAL_HOST=//p' "$RUNTIME_FILE" | head -n 1 ;;
	esac
}

port_in_use() {
	if command -v nc >/dev/null 2>&1; then
		nc -z 127.0.0.1 "$1" >/dev/null 2>&1
	else
		lsof -nP -iTCP:"$1" -sTCP:LISTEN >/dev/null 2>&1
	fi
}

choose_port() {
	local preferred="$1"
	local explicit="$2"
	local port="$preferred"
	if [ -n "$explicit" ]; then
		valid_port "$explicit" || { echo "ERROR: invalid port: $explicit" >&2; exit 1; }
		echo "$explicit"
		return
	fi
	while port_in_use "$port"; do
		port=$((port + 1))
		[ "$port" -le 65535 ] || { echo "ERROR: no free port found" >&2; exit 1; }
	done
	echo "$port"
}

detect_local_host() {
	local interface address
	interface=$(route -n get default 2>/dev/null | sed -n 's/^interface: //p' | head -n 1 || true)
	if [ -n "$interface" ]; then
		address=$(ipconfig getifaddr "$interface" 2>/dev/null || true)
		if [ -n "$address" ]; then
			echo "$address"
			return
		fi
	fi
	echo "127.0.0.1"
}

LOCAL_HOST="${STACKCHAN_LOCAL_HOST:-$(read_runtime STACKCHAN_LOCAL_HOST)}"
LOCAL_HOST="${LOCAL_HOST:-$(detect_local_host)}"
STORED_WS_PORT="$(read_runtime STACKCHAN_WS_PORT)"
STORED_SETTINGS_PORT="$(read_runtime STACKCHAN_SETTINGS_PORT)"
WS_PORT=$(choose_port "${STORED_WS_PORT:-12800}" "${STACKCHAN_WS_PORT:-}")
SETTINGS_PORT=$(choose_port "${STORED_SETTINGS_PORT:-8099}" "${STACKCHAN_SETTINGS_PORT:-}")

TOKEN_CREATED=0
if [ -n "${STACKCHAN_SETTINGS_TOKEN:-}" ]; then
	SETTINGS_TOKEN="$STACKCHAN_SETTINGS_TOKEN"
elif [ -s "$TOKEN_FILE" ]; then
	SETTINGS_TOKEN=$(tr -d '\r\n' < "$TOKEN_FILE")
else
	SETTINGS_TOKEN=$(openssl rand -hex 32)
	printf '%s' "$SETTINGS_TOKEN" > "$TOKEN_FILE"
	chmod 600 "$TOKEN_FILE"
	TOKEN_CREATED=1
fi

sed_escape() {
	printf '%s' "$1" | sed 's/[\\&|]/\\&/g'
}

LOCAL_HOST_ESCAPED=$(sed_escape "$LOCAL_HOST")
SETTINGS_TOKEN_ESCAPED=$(sed_escape "$SETTINGS_TOKEN")
sed \
	-e "s|__LOCAL_HOST__|$LOCAL_HOST_ESCAPED|g" \
	-e "s/__WS_PORT__/$WS_PORT/g" \
	-e "s/__SETTINGS_PORT__/$SETTINGS_PORT/g" \
	-e "s|__SETTINGS_TOKEN__|$SETTINGS_TOKEN_ESCAPED|g" \
	"$RESOURCES/config.yaml" > "$CONFIG_PATH"
chmod 600 "$CONFIG_PATH"
printf 'STACKCHAN_LOCAL_HOST=%s\nSTACKCHAN_WS_PORT=%s\nSTACKCHAN_SETTINGS_PORT=%s\n' \
	"$LOCAL_HOST" "$WS_PORT" "$SETTINGS_PORT" > "$RUNTIME_FILE"
chmod 600 "$RUNTIME_FILE"

cd "$DATA_DIR"
export STACKCHAN_DATA_DIR="$DATA_DIR"
export GF_GCFG_FILE="$CONFIG_PATH"

"$RESOURCES/stackchan-server" &
SERVER_PID=$!
cleanup() {
	kill "$SERVER_PID" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

for _ in 1 2 3 4 5 6 7 8 9 10; do
	if curl -fsS "http://127.0.0.1:${SETTINGS_PORT}/login" >/dev/null 2>&1; then
		if [ "${STACKCHAN_OPEN_SETTINGS:-1}" != "0" ]; then
			open "http://127.0.0.1:${SETTINGS_PORT}/"
		fi
		break
	fi
	sleep 0.5
done

if [ "$TOKEN_CREATED" = 1 ] && [ "${STACKCHAN_OPEN_SETTINGS:-1}" != "0" ] && command -v osascript >/dev/null 2>&1; then
	osascript -e "display dialog \"首次打开设置页需要输入 Settings Token：\\n\\n$SETTINGS_TOKEN\\n\\n设备 OTA 地址：\\nhttp://$LOCAL_HOST:$WS_PORT/xiaozhi/ota/\\n\\nToken 已保存到：\\n$TOKEN_FILE\" with title \"StackChan AI Server\" buttons {\"知道了\"} default button \"知道了\""
fi

wait "$SERVER_PID"
