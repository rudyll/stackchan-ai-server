# Changelog

## 2.8.0-beta.1 (Beta)

- Added per-device background task queues for OpenAI Realtime conversations, with create, status, and cancellation tools.
- Persisted task state and pending announcements across device reconnects, with explicit restart failure recovery and result expiry.
- Delayed completion announcements until the user, model response, and device audio queue are idle, and prevented duplicate delivery.
- Added Home Assistant Ingress settings for an OpenAI-compatible background Agent endpoint, model, timeout, and prompt.
- Added FIFO, cancellation, owner-isolation, reconnect, restart, expiry, and race tests for background task lifecycle behavior.

### Beta testing notes

- Enable the feature from **Open Web UI → Background tasks** and configure an OpenAI-compatible model that supports `/v1/chat/completions`.
- Background task tools are currently available only when the foreground provider is OpenAI Realtime; Gemini Live and the turn-based compatible voice pipeline are not included in this beta.
- Web search and code execution are not added automatically. The background model can use the existing Home Assistant tools.
- Tasks interrupted by an add-on restart are reported as failed; completed results waiting for announcement are retained for up to seven days.
- This release passed automated unit, race, vet, build, shell, and YAML checks. Physical StackChan and live Home Assistant testing is requested from beta testers.

## 2.7.0

- Added a protected Home Assistant Ingress configuration UI with provider-aware basic settings, independent voice-pipeline settings, and Device-Id profiles.
- Replaced static voice dropdowns in the web UI with provider-native free-text voice fields.

## 2.6.0

- Added independent OpenAI-compatible STT, LLM, and TTS endpoint, API key, model, and voice settings.
- Preserved the existing `compatible_*` settings as fallbacks for single-endpoint configurations.

## 2.5.0

- Added configurable initial-audio prebuffering to reduce audible playback gaps from uneven upstream audio delivery.
- Added `device_profiles`: Device-Id keyed overrides for provider, prompt, model, and voice.
- Clarified native realtime versus compatible HTTP pipeline requirements and mainland-China latency tradeoffs in both READMEs.

## 2.4.0

- Added `tokenhub`, `openrouter`, and `openai_compatible` provider choices.
- Added a configurable OpenAI-compatible STT → Chat Completions → TTS pipeline with Home Assistant tool calling and interruption support.
- Added `[LAT]` logs for STT, LLM, TTS start, and first-audio latency measured from device `listen:stop`.

### Notes

- The new compatible pipeline is turn-based HTTP, not a bidirectional realtime audio protocol. Configure models that support `/v1/audio/transcriptions`, `/v1/chat/completions`, and `/v1/audio/speech` at the selected endpoint.
- OpenRouter uses `https://openrouter.ai/api/v1` automatically. TokenHub requires its OpenAI-compatible base URL to be entered explicitly.
