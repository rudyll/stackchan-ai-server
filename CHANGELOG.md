# Changelog

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
