/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConfigUIRequiresBearerTokenWhenEnabled(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := configUIAuth(next, "test-token", true)

	for _, test := range []struct {
		name   string
		header string
		want   int
	}{
		{name: "missing", want: http.StatusUnauthorized},
		{name: "wrong", header: "Bearer wrong-token", want: http.StatusUnauthorized},
		{name: "valid", header: "Bearer test-token", want: http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if test.header != "" {
				req.Header.Set("Authorization", test.header)
			}
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, req)
			if resp.Code != test.want {
				t.Fatalf("status = %d, want %d", resp.Code, test.want)
			}
		})
	}
}

func TestConfigUIKeepsIngressCompatibilityWithoutRequiredAuth(t *testing.T) {
	handler := configUIAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), "", false)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/", nil))
	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusNoContent)
	}
}

func TestConfigUIIncludesProviderCatalogControls(t *testing.T) {
	for _, text := range []string{
		"OpenAI Realtime",
		"Gemini Live",
		"gemini_enable_search",
		"Tencent TokenHub",
		"OpenRouter",
		"OpenAI-compatible",
		"system_prompt",
		"/api/provider-catalog",
		"检测 Provider、模型和声音",
	} {
		if !strings.Contains(configUIHTML, text) {
			t.Fatalf("config UI is missing %q", text)
		}
	}
}

func TestProviderCatalogRouteDoesNotPersistUnsupportedCheck(t *testing.T) {
	payload, err := json.Marshal(providerCatalogSettings{Provider: "unknown", Settings: map[string]string{}})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/provider-catalog", bytes.NewReader(payload))
	response := httptest.NewRecorder()
	configUIHandler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	result := providerCatalog{}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected unsupported provider error")
	}
}
