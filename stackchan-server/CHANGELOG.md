# Changelog

## 2.8.0-beta.3 (Beta)

- Released the accumulated shared-server and standalone improvements below for the HA add-on. Added crown-and-wings artwork to the bundled settings header/favicon, HA store icon/logo and bilingual README headers.
- Fixed HA launcher handling of `gemini_enable_tools: false` and forwarded `gemini_enable_search` into the runtime configuration. Previously, false was replaced by the default and Search was omitted.
- Matched the Docker compiler/runtime Alpine versions at 3.23, made Go dependency resolution read-only at build time, and excluded local data, credentials, logs and packages from the Docker build context.
- Added a four-architecture container verification workflow that starts both the HA launcher and standalone Compose, checking settings/authentication, saved settings after restart, embedded artwork, and advertised NVS/OTA ports without live credentials.
- Added a permanent device setup / NVS guide to the shared HA and standalone settings UI, with runtime host/port, copyable OTA URL, USB/ESP-IDF instructions, and whole-NVS overwrite warnings. Setup metadata is authenticated in standalone and read-only; invalid or loopback targets do not produce a copyable URL.
- Kept settings API requests relative to the HA Ingress path and allowed same-origin embedding only in HA mode; standalone retains frame protection. Separated the HTTP listener address from the advertised device port so Docker custom host-port mappings work alongside native macOS ports.
- Added opt-in per-device local text history, bounded cross-session context for all provider paths, authenticated Markdown export/clear controls, and startup/hourly retention cleanup.
- Added a configurable silent follow-up timeout (15 seconds by default in standalone) that closes the audio channel so standard firmware returns to local wake-word/button standby; silent packets and automatic listen messages do not extend it.
- Combined Gemini transcription fragments per turn for history and stopped logging the full Gemini setup payload, which may now contain private past context.
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
- Disabled HA-only tools and background-task controls when standalone's optional HA bridge is off, while keeping Gemini Search available.
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

### Upgrade and testing notes

- HA: back up the add-on, refresh the add-on store, then update to `2.8.0-beta.3` and restart. Keep existing options and `/data`; open **Web UI** for the updated settings and permanent NVS guide. The built-in HA Configuration tab is a separate form. The store icon is not the navigation sidebar icon.
- Docker: fetch the updated source, preserve `.env` and the mounted data directory, then run `docker compose -f docker-compose.standalone.yml up --build -d` from `stackchan-server`. Containers are built from source; no prebuilt registry image is published. Existing containers do not update merely because GitHub changed.
- macOS: the independently versioned `macos-v0.1.1` universal DMG remains the current download; this HA/container release does not replace that artifact.
- Automated tests do not replace physical StackChan audio, real HA Supervisor/Ingress or live AI-provider testing. Those end-to-end checks remain outstanding. The existing full-project vet warnings in `internal/web_socket/web_socket.go` are outside this release; the full Go suite is run with `-vet=off`, with a separate AI-package race check.
