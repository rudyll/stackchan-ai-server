# Changelog

## Unreleased

- Added an optional standalone Home Assistant bridge: configure the HA URL and Long-Lived Access Token to let voice sessions discover entities, query state, call services, and run background HA tasks without routing device traffic through HA.
- Added the first macOS native standalone packaging path: an unsigned development DMG builder with bundled OPUS runtime, automatic LAN host/port selection, persistent per-user state, and local settings-page launch.
- Refreshed the standalone settings GUI with a responsive layout, sidebar navigation, runtime and configuration summary cards, grouped pipeline stages, and a persistent save bar without changing the settings API.
- Updated indirect `golang.org/x/net` and `golang.org/x/text` dependencies, plus the required `golang.org/x/sys` version and checksums, to address the reported dependency advisory.
- The NVS injector now accepts a resolvable LAN hostname as well as an IPv4 address for standalone OTA URLs; automatic service discovery is not enabled because it would require firmware support.
- Added the standalone settings GUI with separate OpenAI Realtime, Gemini Live, TokenHub, OpenRouter, and OpenAI-compatible provider entries.
- Added protected provider catalog discovery: model names are fetched from the configured Provider API, while native voice catalogs are populated for OpenAI and Gemini.
- Added clearer discovery errors and kept catalog checks separate from saving settings, so a failed check cannot overwrite the active configuration.
- Added standalone GUI controls for Gemini HA tools and Google Search, with the runtime now honoring those saved flags for new sessions.
- Fixed the standalone default so the Home Assistant-only Gemini tools setting starts disabled; added Gemini fields to the standalone environment example.
- Aligned the add-on OpenAI voice schema with the current GUI/provider voice catalog.
- Added a browser login page for the standalone settings UI; the token is exchanged for a short-lived HttpOnly session cookie.
- Disabled HA-only tools and background-task controls in the standalone GUI while keeping Gemini Search available.
- Documented how to retrieve the generated settings token after detached Compose startup and how to use a custom settings host port.
- Added TokenHub and OpenRouter environment examples, plus custom-port support and regression coverage in the NVS OTA injector for same-host HA and standalone deployments.
- Kept standalone runtime mode read-only at the API and persistence layers, filtered settings responses to GUI fields, rejected unsupported updates, and serialized concurrent settings writes.
- Reject invalid `device_profiles` JSON in the settings API, and allow an explicitly cleared profile field to override older environment configuration.
- Reject unknown Provider values at the settings API instead of deferring the error until a device connects.
- Preserve standalone environment-configured Device-Id profiles when the settings UI is opened and an unrelated field is saved.
- Add no-store and browser security headers to the settings UI and API responses.
- Show actionable settings API validation errors in the standalone GUI instead of a generic save failure.
- Add a standalone-only logout link to the settings GUI.
- Show the active runtime mode in the settings GUI, including whether Home Assistant is connected or omitted.
- Document the standalone first-start defaults for Device-Id profiles, system prompt, and audio buffering.

## 2.8.0-beta.2 (Beta)

- Updated the OpenAI Realtime session configuration to the current GA `audio.input` and `audio.output` schema.
- Resampled 16 kHz device audio to the 24 kHz PCM format required by OpenAI Realtime and handled current output audio and transcript event names.
- Changed the OpenAI Realtime and Gemini Live model settings from fixed lists to free-text fields so newly available model IDs can be used without an add-on release.
- Updated the default OpenAI Realtime model to the stable `gpt-realtime` alias.
- Added regression tests for the Realtime session shape and input-audio resampling.

### Beta testing notes

- Automated Go tests and builds pass, but a live OpenAI API session and physical StackChan audio test were not run because no API key or device was available in this workspace.

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
