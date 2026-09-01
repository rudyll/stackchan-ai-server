/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import (
	"context"
	"testing"
)

func TestCompatibleConfigForTokenHubUsesProviderAndStageSettings(t *testing.T) {
	t.Setenv("STACKCHAN_DATA_DIR", t.TempDir())
	if err := writeSettings(map[string]string{
		"compatible_base_url": "https://legacy.example/v1",
		"compatible_api_key":  "legacy-key",
		"tokenhub_base_url":   "https://tokenhub.example/v1",
		"tokenhub_api_key":    "tokenhub-key",
		"stt_base_url":        "https://stt.example/v1",
		"stt_api_key":         "stt-key",
		"stt_model":           "stt-model",
		"llm_model":           "llm-model",
		"tts_base_url":        "https://tts.example/v1",
		"tts_api_key":         "tts-key",
		"tts_model":           "tts-model",
		"tts_voice":           "voice-name",
	}); err != nil {
		t.Fatalf("writeSettings() error = %v", err)
	}

	got := compatibleConfigFor(context.Background(), deviceProfile{}, "tokenhub", "prompt")
	if got.STTBaseURL != "https://stt.example/v1" || got.STTAPIKey != "stt-key" || got.STTModel != "stt-model" {
		t.Fatalf("STT config = %#v, want stage-specific settings", got)
	}
	if got.LLMBaseURL != "https://tokenhub.example/v1" || got.LLMAPIKey != "tokenhub-key" || got.LLMModel != "llm-model" {
		t.Fatalf("LLM config = %#v, want TokenHub settings", got)
	}
	if got.TTSBaseURL != "https://tts.example/v1" || got.TTSAPIKey != "tts-key" || got.TTSModel != "tts-model" || got.Voice != "voice-name" {
		t.Fatalf("TTS config = %#v, want stage-specific settings", got)
	}
}

func TestCompatibleConfigForOpenRouterUsesDefaultLLMEndpoint(t *testing.T) {
	t.Setenv("STACKCHAN_DATA_DIR", t.TempDir())
	if err := writeSettings(map[string]string{
		"openrouter_api_key":  "router-key",
		"compatible_base_url": "https://compat.example/v1",
		"compatible_api_key":  "compat-key",
		"compatible_model":    "compat-model",
	}); err != nil {
		t.Fatalf("writeSettings() error = %v", err)
	}

	got := compatibleConfigFor(context.Background(), deviceProfile{}, "openrouter", "prompt")
	if got.LLMBaseURL != "https://openrouter.ai/api/v1" || got.LLMAPIKey != "router-key" || got.LLMModel != "compat-model" {
		t.Fatalf("LLM config = %#v, want OpenRouter default endpoint and settings", got)
	}
	if got.STTBaseURL != "https://compat.example/v1" || got.STTAPIKey != "compat-key" || got.TTSBaseURL != "https://compat.example/v1" || got.TTSAPIKey != "compat-key" {
		t.Fatalf("fallback pipeline config = %#v, want legacy compatible endpoint", got)
	}
}

func TestCompatibleConfigForProfileOverridesModelAndVoice(t *testing.T) {
	t.Setenv("STACKCHAN_DATA_DIR", t.TempDir())
	if err := writeSettings(map[string]string{
		"compatible_base_url":  "https://compat.example/v1",
		"compatible_api_key":   "compat-key",
		"compatible_model":     "global-model",
		"compatible_tts_voice": "global-voice",
	}); err != nil {
		t.Fatalf("writeSettings() error = %v", err)
	}

	got := compatibleConfigFor(context.Background(), deviceProfile{CompatibleModel: "device-model", CompatibleTTSVoice: "device-voice"}, "openai_compatible", "prompt")
	if got.LLMModel != "device-model" || got.Voice != "device-voice" {
		t.Fatalf("profile overrides = %#v, want device-specific model and voice", got)
	}
}
