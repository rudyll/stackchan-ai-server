package ai

import (
	"bytes"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestBrandImagesHaveTransparentCornersAndConsistentSizes(t *testing.T) {
	for _, asset := range []struct {
		path string
		size int
	}{
		{"assets/stackchan-icon.png", 0}, // High-resolution macOS master.
		{"assets/stackchan-mark.png", 256},
		{"../../../../logo.png", 256},
		{"../../../../icon.png", 128},
	} {
		t.Run(asset.path, func(t *testing.T) {
			data, err := os.ReadFile(asset.path)
			if err != nil {
				t.Fatal(err)
			}
			img, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			bounds := img.Bounds()
			if bounds.Dx() != bounds.Dy() || (asset.size > 0 && bounds.Dx() != asset.size) || (asset.size == 0 && bounds.Dx() < 1024) {
				t.Fatalf("unexpected image dimensions: %v", bounds)
			}
			for _, x := range []int{0, bounds.Dx() - 1} {
				for _, y := range []int{0, bounds.Dy() - 1} {
					// Allow one 8-bit alpha step from extraction/resampling, not an opaque backdrop.
					if _, _, _, alpha := img.At(x, y).RGBA(); alpha > 257 {
						t.Fatalf("corner (%d, %d) must be visually transparent, got alpha %d", x, y, alpha)
					}
				}
			}
			if _, _, _, alpha := img.At(bounds.Dx()/2, bounds.Dy()/2).RGBA(); alpha < 250*257 {
				t.Fatal("the dark screen inside the icon must remain at least 98% opaque")
			}
			if asset.path == "../../../../logo.png" && !bytes.Equal(data, configUIIcon) {
				t.Fatal("HA logo and embedded GUI mark must use the same artwork; run scripts/sync-brand-assets.sh")
			}
		})
	}
}
