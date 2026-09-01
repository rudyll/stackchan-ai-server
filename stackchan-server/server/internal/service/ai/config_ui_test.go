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
	"regexp"
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
			req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
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

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/", nil))
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "Settings token") {
		t.Fatalf("unauthenticated root status/body = %d/%q, want login page", resp.Code, resp.Body.String())
	}
	if resp.Header().Get("Cache-Control") != "no-store" || resp.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("security headers = %#v, want no-store and DENY", resp.Header())
	}
	if !strings.Contains(resp.Header().Get("Content-Security-Policy"), "connect-src 'self'") {
		t.Fatalf("CSP = %q, want same-origin API connections allowed", resp.Header().Get("Content-Security-Policy"))
	}
}

func TestConfigUILoginCookieAuthorizesBrowserRequests(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := configUIAuth(next, "test-token", true)

	login := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("token=test-token"))
	login.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, login)
	if loginResponse.Code != http.StatusSeeOther {
		t.Fatalf("login status = %d, want %d", loginResponse.Code, http.StatusSeeOther)
	}
	cookies := loginResponse.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != settingsSessionCookie || cookies[0].Value != "test-token" || !cookies[0].HttpOnly {
		t.Fatalf("login cookies = %#v, want HttpOnly settings session cookie", cookies)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	request.AddCookie(cookies[0])
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("cookie-authenticated status = %d, want %d", response.Code, http.StatusNoContent)
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
		"syncGeminiFlags",
		"syncRuntimeMode",
		"ui_ha_enabled",
		"delete values.ui_ha_enabled",
		"Tencent TokenHub",
		"OpenRouter",
		"OpenAI-compatible",
		"system_prompt",
		"/api/provider-catalog",
		"检测 Provider、模型和声音",
		"保存失败：",
		"response.text()",
		"/logout",
		"href=\"/logout\" hidden",
		"logout.hidden=!standalone",
		"运行模式：Standalone（不连接 Home Assistant）",
		"运行模式：Home Assistant add-on（由 Ingress 保护设置页）",
	} {
		if !strings.Contains(configUIHTML, text) {
			t.Fatalf("config UI is missing %q", text)
		}
	}
}

func TestConfigUIFieldsHaveSettingsPersistenceCoverage(t *testing.T) {
	namePattern := regexp.MustCompile(`<(?:input|select|textarea)[^>]*\sname="([^"]+)"`)
	allowed := make(map[string]struct{}, len(settingsUIKeys))
	for _, key := range settingsUIKeys {
		allowed[key] = struct{}{}
	}
	for _, match := range namePattern.FindAllStringSubmatch(configUIHTML, -1) {
		name := match[1]
		if _, ok := allowed[name]; !ok {
			t.Fatalf("GUI field %q is missing from settingsUIKeys", name)
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

func TestConfigUISettingsRoundTrip(t *testing.T) {
	t.Setenv("STACKCHAN_DATA_DIR", t.TempDir())
	handler := configUIHandler()
	request := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(`{"provider":"gemini","gemini_model":"gemini-live-test"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d", response.Code, http.StatusOK)
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/settings", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", response.Code, http.StatusOK)
	}
	values := map[string]string{}
	if err := json.NewDecoder(response.Body).Decode(&values); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	if values["provider"] != "gemini" || values["gemini_model"] != "gemini-live-test" {
		t.Fatalf("settings = %#v, want saved provider and model", values)
	}
}

func TestConfigUIRejectsRuntimeModeWrites(t *testing.T) {
	t.Setenv("STACKCHAN_DATA_DIR", t.TempDir())
	for _, key := range []string{"ha_enabled", "ui_ha_enabled"} {
		t.Run(key, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(`{"`+key+`":"true"}`))
			response := httptest.NewRecorder()
			configUIHandler().ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("PUT status = %d, want %d", response.Code, http.StatusBadRequest)
			}
			if _, ok := readSettings()[key]; ok {
				t.Fatalf("read-only setting %q was persisted", key)
			}
		})
	}
}

func TestConfigUIRejectsUnknownSettingWrites(t *testing.T) {
	t.Setenv("STACKCHAN_DATA_DIR", t.TempDir())
	request := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(`{"provider":"openai","typo_setting":"ignored"}`))
	response := httptest.NewRecorder()
	configUIHandler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("PUT status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if values := readSettings(); len(values) != 0 {
		t.Fatalf("unknown settings were persisted: %#v", values)
	}
}

func TestConfigUIRejectsUnknownProviderWrites(t *testing.T) {
	t.Setenv("STACKCHAN_DATA_DIR", t.TempDir())
	request := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(`{"provider":"not-a-provider"}`))
	response := httptest.NewRecorder()
	configUIHandler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("PUT status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if !strings.Contains(response.Body.String(), "unsupported provider") {
		t.Fatalf("response = %q, want provider validation error", response.Body.String())
	}
	if values := readSettings(); len(values) != 0 {
		t.Fatalf("unknown provider was persisted: %#v", values)
	}
}

func TestConfigUIRejectsInvalidDeviceProfiles(t *testing.T) {
	t.Setenv("STACKCHAN_DATA_DIR", t.TempDir())
	request := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(`{"device_profiles":"{invalid"}`))
	response := httptest.NewRecorder()
	configUIHandler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("PUT status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if !strings.Contains(response.Body.String(), "device_profiles must be valid JSON") {
		t.Fatalf("response = %q, want validation error", response.Body.String())
	}
	if values := readSettings(); len(values) != 0 {
		t.Fatalf("invalid device profiles were persisted: %#v", values)
	}
}
