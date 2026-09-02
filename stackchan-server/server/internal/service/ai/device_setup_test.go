package ai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
)

func TestDeviceSetupUsesRuntimeEndpoint(t *testing.T) {
	t.Setenv("STACKCHAN_DATA_DIR", t.TempDir())
	previous := g.Cfg().GetAdapter()
	t.Cleanup(func() { g.Cfg().SetAdapter(previous) })
	for _, test := range []struct {
		name, config, host, url string
		port                    int
		ha                      bool
	}{
		{"addon", `{"ha_enabled":true,"local_host":"ha.example","local_port":12800}`, "ha.example", "http://ha.example:12800/xiaozhi/ota/", 12800, true},
		{"standalone custom port", `{"ha_enabled":false,"local_host":"stackchan.local","local_port":12801,"standalone_ha_enabled":true,"standalone_ha_url":"http://ha.example:8123"}`, "stackchan.local", "http://stackchan.local:12801/xiaozhi/ota/", 12801, false},
		{"missing host", `{}`, "", "", 12800, true},
		{"loopback", `{"ha_enabled":false,"local_host":"127.0.0.1"}`, "127.0.0.1", "", 12800, false},
		{"invalid port", `{"local_host":"stackchan.local","local_port":65536}`, "stackchan.local", "", 65536, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			adapter, err := gcfg.NewAdapterContent(`{"ai":` + test.config + `}`)
			if err != nil {
				t.Fatal(err)
			}
			g.Cfg().SetAdapter(adapter)
			handler := configUIAuth(configUIHandler(), "test-token", !test.ha)
			req := httptest.NewRequest(http.MethodGet, "http://browser.example:8099/api/device-setup", nil)
			req.Header.Set("Authorization", "Bearer test-token")
			req.Header.Set("X-Forwarded-Host", "untrusted.example:8123")
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, req)
			var got deviceSetup
			if resp.Code != http.StatusOK || json.Unmarshal(resp.Body.Bytes(), &got) != nil {
				t.Fatalf("response = %d %s", resp.Code, resp.Body.String())
			}
			if got != (deviceSetup{test.ha, test.host, test.port, test.url}) {
				t.Fatalf("setup = %#v", got)
			}
			var fields map[string]any
			if err := json.Unmarshal(resp.Body.Bytes(), &fields); err != nil || len(fields) != 4 {
				t.Fatalf("expected only device endpoint fields: %s", resp.Body.String())
			}
			frame, ancestor := "DENY", "frame-ancestors 'none'"
			if test.ha {
				frame, ancestor = "SAMEORIGIN", "frame-ancestors 'self'"
			}
			if resp.Header().Get("X-Frame-Options") != frame || !strings.Contains(resp.Header().Get("Content-Security-Policy"), ancestor) || resp.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("unexpected security headers: %v", resp.Header())
			}
		})
	}
}

func TestDeviceSetupRequiresAuthAndIsReadOnly(t *testing.T) {
	handler := configUIAuth(configUIHandler(), "test-token", true)
	for _, test := range []struct {
		method, token string
		status        int
	}{
		{http.MethodGet, "", http.StatusUnauthorized},
		{http.MethodGet, "wrong-token", http.StatusUnauthorized},
		{http.MethodPut, "test-token", http.StatusMethodNotAllowed},
		{http.MethodPost, "test-token", http.StatusMethodNotAllowed},
		{http.MethodDelete, "test-token", http.StatusMethodNotAllowed},
	} {
		req := httptest.NewRequest(test.method, "/api/device-setup", nil)
		req.Header.Set("Authorization", "Bearer "+test.token)
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if resp.Code != test.status {
			t.Fatalf("%s status = %d, want %d", test.method, resp.Code, test.status)
		}
		if test.status == http.StatusMethodNotAllowed && resp.Header().Get("Allow") != http.MethodGet {
			t.Fatal("setup route must allow only GET")
		}
	}
	if settingsUpdateError(map[string]string{"local_host": "changed.example"}) == "" || settingsUpdateError(map[string]string{"local_port": "12801"}) == "" {
		t.Fatal("setup guide must not make runtime endpoints editable through settings")
	}
}

func TestValidNVSHost(t *testing.T) {
	for _, host := range []string{"192.0.2.10", "stackchan.local", "stackchan", "ha-server.example"} {
		if !validNVSHost(host) {
			t.Errorf("validNVSHost(%q) = false", host)
		}
	}
	for _, host := range []string{"", "localhost", "LOCALHOST", "robot.localhost", "127.0.0.1", "127.2.3.4", "127.1", "0.0.0.0", "::", "::1", "2001:db8::1", "::ffff:192.0.2.10", "224.0.0.1", "255.255.255.255", "999.999.999.999", "http://stackchan.local", "stackchan.local:12800", "bad/path", "<script>", "a..b", "-bad.example", "bad_.example", strings.Repeat("a", 64) + ".local"} {
		if validNVSHost(host) {
			t.Errorf("validNVSHost(%q) = true", host)
		}
	}
}
