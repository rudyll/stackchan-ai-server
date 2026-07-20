/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gogf/gf/v2/frame/g"
)

// User-managed settings live outside supervisor-owned options.json. The file
// is only served through the add-on's unexposed ingress port.
const settingsPath = "/data/stackchan-settings.json"

func readSettings() map[string]string {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return map[string]string{}
	}
	values := map[string]string{}
	if json.Unmarshal(data, &values) != nil {
		return map[string]string{}
	}
	return values
}

func writeSettings(values map[string]string) error {
	data, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return err
	}
	tmp := settingsPath + ".tmp"
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, settingsPath)
}

func aiString(ctx context.Context, key, fallback string) string {
	if value := readSettings()[key]; value != "" {
		return value
	}
	return g.Cfg().MustGet(ctx, "ai."+key, fallback).String()
}

func aiInt(ctx context.Context, key string, fallback int) int {
	if value := readSettings()[key]; value != "" {
		if n, err := strconv.Atoi(value); err == nil {
			return n
		}
	}
	return g.Cfg().MustGet(ctx, "ai."+key, fallback).Int()
}

func settingsForUI(ctx context.Context) map[string]string {
	values := readSettings()
	keys := []string{
		"provider", "openai_api_key", "openai_realtime_model", "openai_tts_voice",
		"gemini_api_key", "gemini_model", "gemini_voice", "tokenhub_base_url", "tokenhub_api_key",
		"openrouter_api_key", "compatible_base_url", "compatible_api_key", "compatible_model",
		"stt_base_url", "stt_api_key", "stt_model", "llm_base_url", "llm_api_key", "llm_model",
		"tts_base_url", "tts_api_key", "tts_model", "tts_voice", "device_profiles",
		"audio_prebuffer_ms", "audio_prebuffer_max_wait_ms",
	}
	for _, key := range keys {
		if values[key] == "" {
			values[key] = g.Cfg().MustGet(ctx, "ai."+key, "").String()
		}
	}
	if values["system_prompt"] == "" {
		values["system_prompt"] = globalSystemPrompt(ctx)
	}
	return values
}
