package ai

import (
	"bytes"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConfigUIEmbeddedIconIsPublicButSettingsRemainProtected(t *testing.T) {
	handler := configUIAuth(configUIHandler(), "test-token", true)
	for _, test := range []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, configUIIconPath, http.StatusOK},
		{http.MethodHead, configUIIconPath, http.StatusOK},
		{http.MethodPut, configUIIconPath, http.StatusMethodNotAllowed},
		{http.MethodGet, "/assets/private.png", http.StatusUnauthorized},
		{http.MethodGet, "/api/settings", http.StatusUnauthorized},
		{http.MethodGet, "/api/device-setup", http.StatusUnauthorized},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
		if response.Code != test.want {
			t.Errorf("%s %s: got %d, want %d", test.method, test.path, response.Code, test.want)
		}
		if test.method == http.MethodGet && test.want == http.StatusOK {
			if response.Header().Get("Content-Type") != "image/png" || !bytes.Equal(response.Body.Bytes(), configUIIcon) {
				t.Fatal("icon route must serve only the embedded PNG")
			}
		}
		if !strings.Contains(response.Header().Get("Content-Security-Policy"), "img-src 'self'") {
			t.Fatal("CSP must allow local images")
		}
	}
	icon, err := png.DecodeConfig(bytes.NewReader(configUIIcon))
	if err != nil || icon.Width != 256 || icon.Height != 256 {
		t.Fatalf("unexpected GUI icon dimensions: %v, %v", icon, err)
	}
	for _, page := range []string{configUIHTML, configUILoginHTML} {
		if !strings.Contains(page, `src="./assets/stackchan-icon.png"`) || !strings.Contains(page, `rel="icon"`) {
			t.Fatal("both settings and login must use Ingress-relative brand icons")
		}
	}
}
