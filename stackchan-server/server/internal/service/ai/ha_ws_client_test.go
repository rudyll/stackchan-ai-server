/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
)

func TestNormalizeHAWebSocketURL(t *testing.T) {
	for _, test := range []struct {
		input, want string
	}{
		{"http://homeassistant.local:8123", "ws://homeassistant.local:8123/api/websocket"},
		{"https://ha.example/api/websocket/", "wss://ha.example/api/websocket"},
		{"ws://ha.example:8123/api/websocket", "ws://ha.example:8123/api/websocket"},
	} {
		if got := normalizeHAWebSocketURL(test.input); got != test.want {
			t.Errorf("normalizeHAWebSocketURL(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestStandaloneHomeAssistantConnectionUsesStoredSettings(t *testing.T) {
	t.Setenv("STACKCHAN_DATA_DIR", t.TempDir())
	adapter, err := gcfg.NewAdapterContent(`{"ai":{"ha_enabled":false}}`)
	if err != nil {
		t.Fatal(err)
	}
	previousAdapter := g.Cfg().GetAdapter()
	g.Cfg().SetAdapter(adapter)
	t.Cleanup(func() { g.Cfg().SetAdapter(previousAdapter) })
	if err := writeSettings(map[string]string{
		"standalone_ha_enabled": "true",
		"standalone_ha_url":     "http://ha.local:8123/",
		"standalone_ha_token":   "test-token",
	}); err != nil {
		t.Fatal(err)
	}

	enabled, url, token := homeAssistantConnection(context.Background())
	if !enabled || url != "ws://ha.local:8123/api/websocket" || token != "test-token" {
		t.Fatalf("homeAssistantConnection() = (%t, %q, %q)", enabled, url, token)
	}
}

func TestStandaloneHomeAssistantConnectionRequiresOptIn(t *testing.T) {
	t.Setenv("STACKCHAN_DATA_DIR", t.TempDir())
	adapter, err := gcfg.NewAdapterContent(`{"ai":{"ha_enabled":false,"standalone_ha_url":"http://ha.local:8123","standalone_ha_token":"test-token"}}`)
	if err != nil {
		t.Fatal(err)
	}
	previousAdapter := g.Cfg().GetAdapter()
	g.Cfg().SetAdapter(adapter)
	t.Cleanup(func() { g.Cfg().SetAdapter(previousAdapter) })

	enabled, _, _ := homeAssistantConnection(context.Background())
	if enabled {
		t.Fatal("standalone HA must stay disabled without explicit opt-in")
	}
}
