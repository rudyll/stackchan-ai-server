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
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// User-managed settings live outside supervisor-owned options.json. The file
// is only served through the protected settings UI.
func stackChanDataDir() string {
	if value := strings.TrimSpace(os.Getenv("STACKCHAN_DATA_DIR")); value != "" {
		return filepath.Clean(value)
	}
	return "/data"
}

func settingsFilePath() string {
	return filepath.Join(stackChanDataDir(), "stackchan-settings.json")
}

func readSettings() map[string]string {
	data, err := os.ReadFile(settingsFilePath())
	if err != nil {
		return map[string]string{}
	}
	values := map[string]string{}
	if json.Unmarshal(data, &values) != nil {
		return map[string]string{}
	}
	return values
}

func storedSetting(key string) (string, bool) {
	value, ok := readSettings()[key]
	return value, ok
}

var settingsWriteMu sync.Mutex

func writeSettings(values map[string]string) error {
	settingsWriteMu.Lock()
	defer settingsWriteMu.Unlock()

	merged := readSettings()
	for key, value := range values {
		merged[key] = value
	}
	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return err
	}
	settingsPath := settingsFilePath()
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
	if value, ok := storedSetting(key); ok {
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

func aiBool(ctx context.Context, key string, fallback bool) bool {
	// Runtime mode is owned by the add-on/container configuration. A stale
	// value from an older GUI version must never turn standalone back into HA.
	if key != "ha_enabled" {
		if value := readSettings()[key]; value != "" {
			if parsed, err := strconv.ParseBool(value); err == nil {
				return parsed
			}
		}
	}
	return g.Cfg().MustGet(ctx, "ai."+key, fallback).Bool()
}

var settingsUIKeys = []string{
	"provider", "openai_api_key", "openai_realtime_model", "openai_tts_voice",
	"gemini_api_key", "gemini_model", "gemini_voice", "gemini_enable_tools", "gemini_enable_search",
	"tokenhub_base_url", "tokenhub_api_key", "openrouter_api_key", "openrouter_base_url",
	"compatible_base_url", "compatible_api_key", "compatible_model", "compatible_stt_model", "compatible_tts_model", "compatible_tts_voice",
	"stt_base_url", "stt_api_key", "stt_model", "llm_base_url", "llm_api_key", "llm_model",
	"tts_base_url", "tts_api_key", "tts_model", "tts_voice", "device_profiles",
	"audio_prebuffer_ms", "audio_prebuffer_max_wait_ms",
	"background_tasks_enabled", "background_agent_base_url", "background_agent_api_key",
	"background_agent_model", "background_agent_timeout_seconds", "background_agent_prompt", "system_prompt",
	"standalone_ha_enabled", "standalone_ha_url", "standalone_ha_token",
	"conversation_history_enabled", "conversation_history_days", "conversation_context_messages", "conversation_idle_seconds",
}

var settingsSecretKeys = map[string]struct{}{
	"standalone_ha_token": {},
}

func settingsForUI(ctx context.Context) map[string]string {
	stored := readSettings()
	values := make(map[string]string, len(settingsUIKeys)+1)
	for _, key := range settingsUIKeys {
		if _, secret := settingsSecretKeys[key]; secret {
			continue
		}
		if value, ok := stored[key]; ok {
			values[key] = value
		}
	}
	values["ui_ha_enabled"] = strconv.FormatBool(aiBool(ctx, "ha_enabled", true))
	for _, key := range settingsUIKeys {
		if _, secret := settingsSecretKeys[key]; secret {
			continue
		}
		if _, ok := values[key]; !ok {
			if key == "device_profiles" {
				values[key] = configuredDeviceProfiles(ctx)
				continue
			}
			values[key] = g.Cfg().MustGet(ctx, "ai."+key, "").String()
		}
	}
	if values["system_prompt"] == "" {
		values["system_prompt"] = globalSystemPrompt(ctx)
	}
	values["standalone_ha_token_configured"] = strconv.FormatBool(strings.TrimSpace(aiString(ctx, "standalone_ha_token", "")) != "")
	values["conversation_history_enabled"] = strconv.FormatBool(aiBool(ctx, "conversation_history_enabled", false))
	values["conversation_history_days"] = strconv.Itoa(int(historyRetention(ctx) / (24 * time.Hour)))
	values["conversation_context_messages"] = strconv.Itoa(min(100, max(0, aiInt(ctx, "conversation_context_messages", 20))))
	values["conversation_idle_seconds"] = strconv.Itoa(conversationIdleSeconds(ctx))
	return values
}
