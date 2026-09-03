// SPDX-FileCopyrightText: 2026 rudyll
// SPDX-License-Identifier: AGPL-3.0-only

package ai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConfigUILegalLinksAvailableBeforeAndAfterLogin(t *testing.T) {
	t.Setenv("STACKCHAN_DATA_DIR", t.TempDir())
	handler := configUIAuth(configUIHandler(), "legal-test-token", true)
	for _, token := range []string{"", "legal-test-token"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != http.StatusOK {
			t.Fatalf("page status = %d", response.Code)
		}
		for _, expected := range []string{"源码 / Source", "AGPL-3.0-only", "SPONSORING.md", "Sponsorship is optional."} {
			if !strings.Contains(response.Body.String(), expected) {
				t.Errorf("page missing %q", expected)
			}
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/settings", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatal("public legal links must not expose protected settings")
	}
}
