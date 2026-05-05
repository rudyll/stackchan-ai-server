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
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
)

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
}

// dialProvider selects the configured backend (openai | gemini) and opens a
// session. Provider-specific config keys live under ai.* in the generated
// config.yaml (written by addon/run.sh from /data/options.json).
func dialProvider(
	ctx context.Context,
	ha *haWSClient,
	cb RealtimeCallbacks,
) (RealtimeSession, error) {
	cfg := g.Cfg()
	provider := cfg.MustGet(ctx, "ai.provider", "openai").String()
	sysPrompt := cfg.MustGet(ctx, "ai.system_prompt",
		"You are StackChan, a friendly desktop robot assistant.").String()

	switch provider {
	case "gemini":
		apiKey := cfg.MustGet(ctx, "ai.gemini_api_key", "").String()
		if apiKey == "" {
			return nil, fmt.Errorf("ai.gemini_api_key is required when provider=gemini")
		}
		model := cfg.MustGet(ctx, "ai.gemini_model",
			"gemini-2.5-flash-preview-native-audio-dialog").String()
		voice := cfg.MustGet(ctx, "ai.gemini_voice", "Aoede").String()
		return dialGeminiSession(ctx, apiKey, model, voice, sysPrompt, ha, cb)

	case "openai", "":
		apiKey := cfg.MustGet(ctx, "ai.openai_api_key", "").String()
		if apiKey == "" {
			return nil, fmt.Errorf("ai.openai_api_key is required when provider=openai")
		}
		model := cfg.MustGet(ctx, "ai.openai_realtime_model", "gpt-realtime-1.5").String()
		voice := cfg.MustGet(ctx, "ai.openai_tts_voice", "alloy").String()
		return dialOpenAIRealtimeSession(ctx, apiKey, model, voice, sysPrompt, ha, cb)

	default:
		return nil, fmt.Errorf("unknown ai.provider %q (expected openai|gemini)", provider)
	}
}
