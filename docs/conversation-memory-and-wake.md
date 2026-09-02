# Conversation history and wake-word standby

## Storage and context are separate

The settings UI's **Provider → 对话记忆与唤醒** group controls text history and the
idle audio-channel timeout. These settings apply to new device connections.

| Setting | Default | Meaning |
| --- | --- | --- |
| `conversation_history_enabled` | `false` | Opt in to saving user/assistant transcripts. |
| `conversation_history_days` | `90` | Keep 1–3650 days of records, subject to the count limit below. |
| `conversation_context_messages` | `20` | Include up to 0–100 recent messages on reconnect; 0 means archive only. |
| `conversation_idle_seconds` | Standalone: `15`; HA add-on: `0` | End the audio channel after silence; 0 disables automatic closure; maximum 300 seconds. |

Docker exposes the corresponding `STACKCHAN_CONVERSATION_*` environment variables
in `.env.standalone.example`. GUI values override startup defaults. The macOS
application uses the same settings UI and implementation.

The canonical store is `conversation-history/<SHA256-of-Device-Id>.json` under
`STACKCHAN_DATA_DIR`: normally the mounted `./data` directory for Docker, or
`~/Library/Application Support/StackChan AI Server` for the macOS application.
Files are written with mode `0600` in a `0700` directory and replaced atomically.
Transcripts are plain text, not encrypted. No raw audio, tool payloads or provider
configuration are added to this history store. Existing diagnostic logs have
their own retention and can still contain conversation text.

Each device retains at most 2000 messages, with at most 4096 Unicode characters
per message. Expiry is checked on access, at server startup and hourly, including
inactive devices. The cleanup worker is started with the settings server; errors
appear as `[HISTORY] retention cleanup failed` in the server log. Invalid JSON is
reported rather than silently replaced with an empty archive.

New sessions read only that device's recent history, capped at 12000 text
characters in addition to the message limit. The selected provider receives
these records as explicitly quoted past context, not as new requests. Past tool
calls are not replayed. All three provider paths use this context: OpenAI
Realtime, Gemini Live, and the compatible STT → LLM → TTS pipeline. Gemini requests
audio transcriptions when history is enabled and combines their fragments per
turn. Interrupted Gemini replies are not archived; other providers' generated
text may include a reply whose device playback was interrupted.

This is recent cross-session context, not semantic search or automatic long-term
fact extraction. An older record can remain on disk without being included in the
next model request. Turning history off stops saving and loading it, but does not
erase existing files. Set the message limit to 0 to save without sending old text
to a provider.

Enter the device's `Device-Id` in the same UI group to export Markdown or clear
its records. The authenticated settings API also supports
`GET /api/conversation-history?device_id=...&format=md` and
`DELETE /api/conversation-history?device_id=...`. Clearing disk history cannot
erase context already loaded by an active provider session: end that conversation
before starting a new one. New messages will be saved again if recording remains
enabled. Records are shared by people using the same robot; Device-Id is a
logical key, not user authentication. Keep the device endpoint on a trusted LAN.

## Wake-word behavior

The firmware performs local wake-word detection. The server does not recognize a
keyword from an always-uploaded audio stream and cannot change the firmware's
wake-word model from this settings page.

The upstream [Xiaozhi protocol](https://github.com/78/xiaozhi-esp32/blob/main/docs/websocket.md)
opens an audio channel for wake-word/button activation and returns to idle when
that channel closes. The [firmware implementation](https://github.com/78/xiaozhi-esp32/blob/main/main/application.cc)
also has builds that do not send a separate `listen:detect` event. For that
reason the server accepts the initial `listen:start` after a device opens a
channel instead of requiring a `detect` message from every firmware build.

After activation, audible input renews the short conversation window. Silent
audio packets and repeated automatic `listen:start` messages do not renew it.
While a response is being generated or sent, a separate two-minute stalled-response
limit prevents premature closure. The short follow-up window restarts once all
queued response frames have been sent. At expiry the server stops accepting audio
and closes the device/provider connection; the standard firmware returns to
local wake-word/button standby. Background job results remain queued for the
next device connection rather than keeping its microphone session open forever.

The energy floor used to distinguish audio activity from silence is not speaker
identification or intent detection. Background speech/noise inside the open
window may extend it or trigger a reply. Firmware that automatically reconnects
and starts listening on its own will need a firmware-side wake gate. Custom wake
words and a strict wake event for every activation likewise require firmware
support. Verify standby, follow-up, interruption and long-sentence behavior on
the actual StackChan firmware before treating this as a hardware-validated release.
