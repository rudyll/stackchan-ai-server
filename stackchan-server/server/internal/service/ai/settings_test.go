/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import (
	"context"
	"strconv"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
)

func TestWriteSettingsPreservesFieldsNotShownByCurrentUI(t *testing.T) {
	t.Setenv("STACKCHAN_DATA_DIR", t.TempDir())
	if err := writeSettings(map[string]string{
		"compatible_base_url": "https://legacy.example/v1",
		"compatible_api_key":  "legacy-secret",
	}); err != nil {
		t.Fatalf("initial writeSettings() error = %v", err)
	}
	if err := writeSettings(map[string]string{"provider": "openai"}); err != nil {
		t.Fatalf("second writeSettings() error = %v", err)
	}

	values := readSettings()
	if values["compatible_base_url"] != "https://legacy.example/v1" || values["compatible_api_key"] != "legacy-secret" {
		t.Fatalf("legacy settings were lost: %#v", values)
	}
	if values["provider"] != "openai" {
		t.Fatalf("provider = %q, want openai", values["provider"])
	}
}

func TestGeminiFeatureFlagsUseGUISettingsForNewSessions(t *testing.T) {
	t.Setenv("STACKCHAN_DATA_DIR", t.TempDir())
	if err := writeSettings(map[string]string{
		"gemini_enable_tools":  "false",
		"gemini_enable_search": "true",
	}); err != nil {
		t.Fatalf("writeSettings() error = %v", err)
	}
	ctx := context.Background()
	if aiBool(ctx, "gemini_enable_tools", true) {
		t.Fatal("Gemini tools should be disabled by GUI settings")
	}
	if !aiBool(ctx, "gemini_enable_search", false) {
		t.Fatal("Gemini Search should be enabled by GUI settings")
	}
}

func TestConfiguredProviderUsesGUISettingsForNewSessions(t *testing.T) {
	t.Setenv("STACKCHAN_DATA_DIR", t.TempDir())
	if err := writeSettings(map[string]string{"provider": "gemini"}); err != nil {
		t.Fatalf("writeSettings() error = %v", err)
	}
	if got := configuredProvider(context.Background()); got != "gemini" {
		t.Fatalf("configuredProvider() = %q, want gemini", got)
	}
}

func TestGUICanClearAConfiguredStringSetting(t *testing.T) {
	t.Setenv("STACKCHAN_DATA_DIR", t.TempDir())
	if err := writeSettings(map[string]string{"openai_api_key": ""}); err != nil {
		t.Fatalf("writeSettings() error = %v", err)
	}
	if got := aiString(context.Background(), "openai_api_key", "fallback-key"); got != "" {
		t.Fatalf("aiString() = %q, want an explicitly saved empty value", got)
	}
	if got := settingsForUI(context.Background())["openai_api_key"]; got != "" {
		t.Fatalf("settingsForUI() returned %q, want an explicitly saved empty value", got)
	}
}

func TestSettingsUIExposesRuntimeModeAsReadOnlyMetadata(t *testing.T) {
	t.Setenv("STACKCHAN_DATA_DIR", t.TempDir())
	values := settingsForUI(context.Background())
	if values["ui_ha_enabled"] == "" {
		t.Fatal("settingsForUI() did not expose runtime mode metadata")
	}
	if _, ok := values["ha_enabled"]; ok {
		t.Fatal("settingsForUI() must not expose writable ha_enabled configuration")
	}
}

func TestStoredRuntimeModeCannotOverrideStandaloneConfiguration(t *testing.T) {
	t.Setenv("STACKCHAN_DATA_DIR", t.TempDir())
	if err := writeSettings(map[string]string{"ha_enabled": "true", "ui_ha_enabled": "true"}); err != nil {
		t.Fatalf("writeSettings() error = %v", err)
	}
	ctx := context.Background()
	fallback := true
	configured := g.Cfg().MustGet(ctx, "ai.ha_enabled", fallback).Bool()
	if got := aiBool(ctx, "ha_enabled", fallback); got != configured {
		t.Fatalf("aiBool(ha_enabled) = %t, want config value %t", got, configured)
	}
	values := settingsForUI(ctx)
	if _, ok := values["ha_enabled"]; ok {
		t.Fatal("settingsForUI() exposed stored ha_enabled")
	}
	if got := values["ui_ha_enabled"]; got != strconv.FormatBool(configured) {
		t.Fatalf("ui_ha_enabled = %q, want %q", got, strconv.FormatBool(configured))
	}
}
