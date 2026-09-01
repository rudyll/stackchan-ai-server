/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import "testing"

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
