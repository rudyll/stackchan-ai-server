/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

// Provider abstraction for the realtime LLM/TTS backend.
//
// ws_simulator.go talks to whatever LLM provider is configured via this
// interface. Audio is normalized to int16 PCM (16kHz in, 24kHz out) and
// callbacks are kept provider-agnostic, so any new backend (OpenAI Realtime,
// Gemini Live, etc.) only needs to map its wire protocol → these callbacks.
package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
)

// deviceProfile overrides global behaviour for one physical device. API keys
// remain global, so profile JSON can safely be stored in add-on options.
type deviceProfile struct {
	Provider            string `json:"provider"`
	SystemPrompt        string `json:"system_prompt"`
	OpenAIRealtimeModel string `json:"openai_realtime_model"`
	OpenAITTSVoice      string `json:"openai_tts_voice"`
	GeminiModel         string `json:"gemini_model"`
	GeminiVoice         string `json:"gemini_voice"`
	CompatibleModel     string `json:"compatible_model"`
	CompatibleSTTModel  string `json:"compatible_stt_model"`
	CompatibleTTSModel  string `json:"compatible_tts_model"`
	CompatibleTTSVoice  string `json:"compatible_tts_voice"`
}

func deviceProfileFor(ctx context.Context, deviceID string) deviceProfile {
	encoded := g.Cfg().MustGet(ctx, "ai.device_profiles_b64", "").String()
	if encoded == "" || deviceID == "" {
		return deviceProfile{}
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return deviceProfile{}
	}
	profiles := map[string]deviceProfile{}
	if json.Unmarshal(raw, &profiles) != nil {
		return deviceProfile{}
	}
	return profiles[deviceID]
}

func override(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func globalSystemPrompt(ctx context.Context) string {
	const fallback = "You are StackChan, a friendly desktop robot assistant."
	encoded := g.Cfg().MustGet(ctx, "ai.system_prompt_b64", "").String()
	if raw, err := base64.StdEncoding.DecodeString(encoded); err == nil && len(raw) > 0 {
		return string(raw)
	}
	return fallback
}

// RealtimeSession is the wire-protocol-agnostic surface ws_simulator depends on.
// One session per device WebSocket connection.
type RealtimeSession interface {
	// AppendAudio sends a PCM16 16kHz mono chunk to the model's input buffer.
	AppendAudio(pcm []int16) error
	// CommitAudio explicitly flushes the input buffer. May be a no-op for
	// providers that always rely on server VAD (e.g. Gemini Live).
	CommitAudio() error
	// CancelResponse interrupts an in-progress reply (wake word / abort).
	CancelResponse() error
	// Close shuts down the underlying connection.
	Close()
}

// RealtimeCallbacks bundles the events ws_simulator wants from the provider.
// All callbacks are invoked from the provider's read goroutine — keep them
// quick and lock-free where possible.
type RealtimeCallbacks struct {
	OnSTT   func(string)  // user speech transcript
	OnText  func(string)  // model text reply (logging only)
	OnAudio func([]int16) // 24kHz PCM chunk to play back to the device
	OnStart func()        // model began speaking
	OnStop  func()        // model finished speaking
	OnClose func()        // provider connection ended (any reason — error or normal)
}

// dialProvider selects the configured backend and opens a
// session. Provider-specific config keys live under ai.* in the generated
// config.yaml (written by addon/run.sh from /data/options.json).
func dialProvider(
	ctx context.Context,
	deviceID string,
	ha *haWSClient,
	cb RealtimeCallbacks,
) (RealtimeSession, error) {
	cfg := g.Cfg()
	p := deviceProfileFor(ctx, deviceID)
	provider := override(p.Provider, cfg.MustGet(ctx, "ai.provider", "openai").String())
	sysPrompt := override(p.SystemPrompt, globalSystemPrompt(ctx))

	switch provider {
	case "openai_compatible", "tokenhub", "openrouter":
		baseURL := cfg.MustGet(ctx, "ai.compatible_base_url", "").String()
		apiKey := cfg.MustGet(ctx, "ai.compatible_api_key", "").String()
		if provider == "tokenhub" {
			baseURL = cfg.MustGet(ctx, "ai.tokenhub_base_url", baseURL).String()
			apiKey = cfg.MustGet(ctx, "ai.tokenhub_api_key", apiKey).String()
		}
		if provider == "openrouter" {
			baseURL = cfg.MustGet(ctx, "ai.openrouter_base_url", "https://openrouter.ai/api/v1").String()
			apiKey = cfg.MustGet(ctx, "ai.openrouter_api_key", apiKey).String()
		}
		if apiKey == "" || baseURL == "" {
			return nil, fmt.Errorf("a base URL and API key are required when provider=%s", provider)
		}
		return dialCompatibleSession(ctx, compatibleConfig{
			BaseURL:  baseURL,
			APIKey:   apiKey,
			Model:    override(p.CompatibleModel, cfg.MustGet(ctx, "ai.compatible_model", "").String()),
			STTModel: override(p.CompatibleSTTModel, cfg.MustGet(ctx, "ai.compatible_stt_model", "whisper-1").String()),
			TTSModel: override(p.CompatibleTTSModel, cfg.MustGet(ctx, "ai.compatible_tts_model", "tts-1").String()),
			Voice:    override(p.CompatibleTTSVoice, cfg.MustGet(ctx, "ai.compatible_tts_voice", "alloy").String()),
			Prompt:   sysPrompt,
		}, ha, cb)

	case "gemini":
		apiKey := cfg.MustGet(ctx, "ai.gemini_api_key", "").String()
		if apiKey == "" {
			return nil, fmt.Errorf("ai.gemini_api_key is required when provider=gemini")
		}
		model := override(p.GeminiModel, cfg.MustGet(ctx, "ai.gemini_model", "gemini-2.5-flash-preview-native-audio-dialog").String())
		voice := override(p.GeminiVoice, cfg.MustGet(ctx, "ai.gemini_voice", "Aoede").String())
		return dialGeminiSession(ctx, apiKey, model, voice, sysPrompt, ha, cb)

	case "openai", "":
		apiKey := cfg.MustGet(ctx, "ai.openai_api_key", "").String()
		if apiKey == "" {
			return nil, fmt.Errorf("ai.openai_api_key is required when provider=openai")
		}
		model := override(p.OpenAIRealtimeModel, cfg.MustGet(ctx, "ai.openai_realtime_model", "gpt-realtime-1.5").String())
		voice := override(p.OpenAITTSVoice, cfg.MustGet(ctx, "ai.openai_tts_voice", "alloy").String())
		return dialOpenAIRealtimeSession(ctx, apiKey, model, voice, sysPrompt, ha, cb)

	default:
		return nil, fmt.Errorf("unknown ai.provider %q", provider)
	}
}
