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
	if raw := readSettings()["device_profiles"]; raw != "" {
		profiles := map[string]deviceProfile{}
		if json.Unmarshal([]byte(raw), &profiles) == nil {
			return profiles[deviceID]
		}
	}
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
	if prompt := aiString(ctx, "system_prompt", ""); prompt != "" {
		return prompt
	}
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

// PlaybackStateAware lets a provider postpone asynchronous announcements
// until the device has physically drained its audio queue.
type PlaybackStateAware interface {
	SetPlaybackBusy(bool)
	InterruptPlayback()
}

// AsyncDeliveryAware gates persisted result announcements until the Xiaozhi
// hello handshake has completed on a newly connected device.
type AsyncDeliveryAware interface {
	SetDeliveryReady(bool)
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
	p := deviceProfileFor(ctx, deviceID)
	provider := override(p.Provider, aiString(ctx, "provider", "openai"))
	sysPrompt := override(p.SystemPrompt, globalSystemPrompt(ctx))

	switch provider {
	case "openai_compatible", "tokenhub", "openrouter":
		compatibleBaseURL := aiString(ctx, "compatible_base_url", "")
		compatibleAPIKey := aiString(ctx, "compatible_api_key", "")
		llmBaseURL, llmAPIKey := compatibleBaseURL, compatibleAPIKey
		if provider == "tokenhub" {
			llmBaseURL = aiString(ctx, "tokenhub_base_url", llmBaseURL)
			llmAPIKey = aiString(ctx, "tokenhub_api_key", llmAPIKey)
		}
		if provider == "openrouter" {
			llmBaseURL = aiString(ctx, "openrouter_base_url", "https://openrouter.ai/api/v1")
			llmAPIKey = aiString(ctx, "openrouter_api_key", llmAPIKey)
		}
		// Stage-specific values override the legacy compatible endpoint. This
		// enables e.g. domestic STT + TokenHub LLM + local TTS without breaking
		// existing single-endpoint configurations.
		sttBaseURL := override(aiString(ctx, "stt_base_url", ""), compatibleBaseURL)
		sttAPIKey := override(aiString(ctx, "stt_api_key", ""), compatibleAPIKey)
		ttsBaseURL := override(aiString(ctx, "tts_base_url", ""), compatibleBaseURL)
		ttsAPIKey := override(aiString(ctx, "tts_api_key", ""), compatibleAPIKey)
		llmBaseURL = override(aiString(ctx, "llm_base_url", ""), llmBaseURL)
		llmAPIKey = override(aiString(ctx, "llm_api_key", ""), llmAPIKey)
		sttModel := override(p.CompatibleSTTModel, override(aiString(ctx, "stt_model", ""), aiString(ctx, "compatible_stt_model", "whisper-1")))
		llmModel := override(p.CompatibleModel, override(aiString(ctx, "llm_model", ""), aiString(ctx, "compatible_model", "")))
		ttsModel := override(p.CompatibleTTSModel, override(aiString(ctx, "tts_model", ""), aiString(ctx, "compatible_tts_model", "tts-1")))
		voice := override(p.CompatibleTTSVoice, override(aiString(ctx, "tts_voice", ""), aiString(ctx, "compatible_tts_voice", "alloy")))
		return dialCompatibleSession(ctx, compatibleConfig{
			STTBaseURL: sttBaseURL,
			STTAPIKey:  sttAPIKey,
			STTModel:   sttModel,
			LLMBaseURL: llmBaseURL,
			LLMAPIKey:  llmAPIKey,
			LLMModel:   llmModel,
			TTSBaseURL: ttsBaseURL,
			TTSAPIKey:  ttsAPIKey,
			TTSModel:   ttsModel,
			Voice:      voice,
			Prompt:     sysPrompt,
		}, ha, cb)

	case "gemini":
		apiKey := aiString(ctx, "gemini_api_key", "")
		if apiKey == "" {
			return nil, fmt.Errorf("ai.gemini_api_key is required when provider=gemini")
		}
		model := override(p.GeminiModel, aiString(ctx, "gemini_model", "gemini-2.5-flash-preview-native-audio-dialog"))
		voice := override(p.GeminiVoice, aiString(ctx, "gemini_voice", "Aoede"))
		return dialGeminiSession(ctx, apiKey, model, voice, sysPrompt, ha, cb)

	case "openai", "":
		apiKey := aiString(ctx, "openai_api_key", "")
		if apiKey == "" {
			return nil, fmt.Errorf("ai.openai_api_key is required when provider=openai")
		}
		model := override(p.OpenAIRealtimeModel, aiString(ctx, "openai_realtime_model", "gpt-realtime-1.5"))
		voice := override(p.OpenAITTSVoice, aiString(ctx, "openai_tts_voice", "alloy"))
		return dialOpenAIRealtimeSession(ctx, deviceID, apiKey, model, voice, sysPrompt, ha, cb)

	default:
		return nil, fmt.Errorf("unknown ai.provider %q", provider)
	}
}
