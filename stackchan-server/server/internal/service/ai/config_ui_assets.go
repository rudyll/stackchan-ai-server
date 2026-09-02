package ai

import (
	"bytes"
	_ "embed"
	"net/http"
	"time"
)

//go:embed assets/stackchan-mark.png
var configUIIcon []byte

const configUIIconPath = "/assets/stackchan-icon.png"

// This one non-sensitive, embedded image is public so the login page can use
// the same brand mark. No filesystem or arbitrary asset path is exposed.
func configUIIconHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	http.ServeContent(w, r, "stackchan-icon.png", time.Time{}, bytes.NewReader(configUIIcon))
}
