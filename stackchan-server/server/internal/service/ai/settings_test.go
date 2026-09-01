/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"sync"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
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

func TestGUICanClearConfiguredDeviceProfiles(t *testing.T) {
	t.Setenv("STACKCHAN_DATA_DIR", t.TempDir())
	profiles := map[string]deviceProfile{
		"device-1": {Provider: "gemini", GeminiModel: "profile-model"},
	}
	raw, err := json.Marshal(profiles)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(raw)
	adapter, err := gcfg.NewAdapterContent(`{"ai":{"device_profiles_b64":"` + encoded + `"}}`)
	if err != nil {
		t.Fatalf("gcfg.NewAdapterContent() error = %v", err)
	}
	previousAdapter := g.Cfg().GetAdapter()
	g.Cfg().SetAdapter(adapter)
	t.Cleanup(func() { g.Cfg().SetAdapter(previousAdapter) })
	if err := writeSettings(map[string]string{"device_profiles": ""}); err != nil {
		t.Fatalf("writeSettings() error = %v", err)
	}
	if got := deviceProfileFor(context.Background(), "device-1"); got != (deviceProfile{}) {
		t.Fatalf("deviceProfileFor() = %#v, want cleared profile", got)
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

func TestSettingsUIDoesNotExposeUnknownOrSupervisorFields(t *testing.T) {
	t.Setenv("STACKCHAN_DATA_DIR", t.TempDir())
	if err := writeSettings(map[string]string{
		"provider":            "openai",
		"ha_mcp_token":        "should-not-leak",
		"settings_auth_token": "should-not-leak",
		"unknown_secret":      "should-not-leak",
	}); err != nil {
		t.Fatalf("writeSettings() error = %v", err)
	}
	values := settingsForUI(context.Background())
	for _, key := range []string{"ha_mcp_token", "settings_auth_token", "unknown_secret"} {
		if _, ok := values[key]; ok {
			t.Fatalf("settingsForUI() exposed %q", key)
		}
	}
	if values["provider"] != "openai" {
		t.Fatalf("provider = %q, want persisted UI setting", values["provider"])
	}
}

func TestWriteSettingsPreservesConcurrentUpdates(t *testing.T) {
	t.Setenv("STACKCHAN_DATA_DIR", t.TempDir())
	const writers = 12
	var wait sync.WaitGroup
	wait.Add(writers)
	for i := 0; i < writers; i++ {
		i := i
		go func() {
			defer wait.Done()
			if err := writeSettings(map[string]string{"concurrent_key_" + strconv.Itoa(i): strconv.Itoa(i)}); err != nil {
				t.Errorf("writeSettings() error = %v", err)
			}
		}()
	}
	wait.Wait()

	values := readSettings()
	for i := 0; i < writers; i++ {
		key := "concurrent_key_" + strconv.Itoa(i)
		if values[key] != strconv.Itoa(i) {
			t.Fatalf("%s = %q, want %q; values = %#v", key, values[key], strconv.Itoa(i), values)
		}
	}
}
