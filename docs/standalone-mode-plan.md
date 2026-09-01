# Standalone mode plan: use third-party AI without Home Assistant

## Goal

Allow a StackChan/Xiaozhi device to use supported third-party AI providers without installing Home Assistant. Home Assistant remains an optional integration for users who want smart-home tools.

The recommended direction is a dual-runtime StackChan server, not a wholesale replacement with another server:

```text
Custom Xiaozhi firmware or OTA configuration
                    |
                    v
StackChan Server (WebSocket / OPUS)
  |-- Standalone mode: AI conversation and local settings
  |-- HA mode: optional Home Assistant tools and background tasks
                    |
                    v
OpenAI Realtime / Gemini Live / OpenAI-compatible STT + LLM + TTS
```

The current server already has the device protocol and provider layer needed for this direction. It must first remove its mandatory Home Assistant connection and Supervisor-only configuration assumptions.

## Upstream references

- [78/xiaozhi-esp32](https://github.com/78/xiaozhi-esp32) is the device firmware/client. It is the preferred basis for a firmware build that points directly at a self-hosted server.
- [xinnan-tech/xiaozhi-esp32-server](https://github.com/xinnan-tech/xiaozhi-esp32-server) is a standalone backend reference. Its provider matrix and deployment experience are useful, but its code should not be copied or substituted without an explicit compatibility and licensing review.

## Decision

Build standalone mode into this project in small, separately releasable steps.

- Retain the existing Xiaozhi v3 WebSocket/OPUS path and device behavior.
- Retain the existing OpenAI Realtime, Gemini Live, OpenAI-compatible, TokenHub, and OpenRouter provider paths.
- Make Home Assistant tools conditional instead of mandatory.
- Use a custom firmware endpoint as the normal onboarding path. DNS/OTA interception for stock firmware remains an advanced option because it adds TLS, DNS, and network setup risk.

## Phase 0 — Define the runtime contract (implemented)

Add a documented `ha_enabled` setting and define two supported modes.

| Capability | Standalone | Home Assistant add-on |
| --- | --- | --- |
| Voice conversation | Required | Required |
| Third-party AI providers | Required | Required |
| Device configuration UI | Token-protected | HA Ingress-protected |
| Home Assistant tools | Disabled | Optional/enabled |
| Background HA tasks | Disabled | Existing behavior |

Acceptance criteria:

- An empty HA token never causes an otherwise valid standalone device session to close.
- Prompts, tool lists, and logs do not imply that smart-home control is available in standalone mode.
- Existing HA add-on behavior remains unchanged when `ha_enabled=true`.

## Phase 1 — Make Home Assistant optional in the server (implemented)

Change the WebSocket session setup so it does not always dial `ai.ha_ws_url` and close when that connection fails. Introduce a no-op tool path or nullable HA client, and only register HA and background-task tools when Home Assistant is enabled and connected.

Update the system prompt and provider setup so standalone users receive a normal voice assistant rather than smart-home instructions. Add regression tests for both modes, including the negative case where no HA URL or token exists.

## Phase 2 — Add a standalone runtime and secure settings (implemented; beta)

The current launcher reads `/data/options.json`, writes generated configuration, assumes the Supervisor hostname `homeassistant`, and relies on HA Ingress to protect port 8099. Add a separate runtime path with:

- a documented YAML file and/or environment variables;
- a persistent data directory for settings and device profiles;
- Docker Compose for a local server, with WebSocket port 12800 and settings port 8099;
- an initial administrator token or password for the settings UI;
- no default public exposure of the settings UI.

Do not weaken the existing add-on path. The HA launcher may continue to generate its configuration from `/data/options.json`.

Acceptance criteria:

- A fresh Docker Compose installation can serve one device using an OpenAI-compatible provider without HA.
- Secrets persist only in the mounted data directory and are excluded from example files.
- The settings UI rejects unauthenticated reads and writes in standalone mode.

Implemented in the current beta: Docker Compose and environment-based startup, a persistent `/data` directory, generated or supplied settings token, loopback-only settings port mapping, conditional HA/background-task startup, and a protected GUI with multiple provider entries plus model/voice catalog discovery.

## Phase 3 — Device onboarding

Document and test the preferred device path: build the [78/xiaozhi-esp32](https://github.com/78/xiaozhi-esp32) firmware with the self-hosted server/OTA endpoint. Provide one tested configuration example for the supported StackChan hardware.

Keep stock-firmware DNS/OTA interception separate in the documentation as an advanced setup. It should state its required host names, certificate handling, rollback method, and local-network security implications.

Acceptance criteria:

- A newly flashed device connects to the standalone server without any Home Assistant components.
- The device can complete an OPUS voice turn and receive audio playback from each supported provider class.

## Phase 4 — Provider compatibility matrix

Publish a small matrix distinguishing native realtime providers from the turn-based OpenAI-compatible pipeline:

- Native realtime: OpenAI Realtime and Gemini Live.
- Compatible pipeline: STT, chat completion, and TTS endpoints may be supplied independently.
- Provider-specific additions are made only when they can satisfy the required API contract, not merely because they expose an OpenAI-like endpoint.

Use the standalone server project as a reference for missing domestic-provider adapters. Each new adapter needs documented endpoint requirements, audio formats, model/voice fields, failure behavior, and an integration test or mocked transport test.

## Phase 5 — Release and physical validation

Release standalone mode as a beta after all previous acceptance criteria pass. Required validation includes:

1. Go unit tests, targeted race tests, build, shell syntax, YAML validation, and diff checks.
2. Docker Compose startup and authenticated settings UI smoke test.
3. One physical device test without HA, covering connect, speech input, playback, reconnect, and provider failure messages.
4. One regression test with Home Assistant enabled, covering voice control and current add-on configuration migration.

The release notes must state which providers and device hardware were physically tested, and identify untested combinations.

## Risks and guardrails

- A standalone settings UI is a credentials boundary; it must not inherit the add-on's assumption that HA Ingress has already authenticated the request.
- Provider APIs and model IDs change. Keep model fields free-text, but reject incompatible audio or endpoint contracts with actionable errors.
- Custom firmware has the lowest onboarding ambiguity. OTA/DNS interception needs a documented rollback route before it is presented as a standard option.
- Keep the dual-runtime change incremental. Avoid broad provider rewrites or upstream code imports until an individual capability is needed and reviewed.
