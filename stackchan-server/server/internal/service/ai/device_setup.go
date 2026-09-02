package ai

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
)

// Only expose the device-facing endpoint, never settings credentials or the
// browser's Host header (which can point to HA Ingress or a loopback UI port).
type deviceSetup struct {
	HAEnabled bool   `json:"ha_enabled"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	OTAURL    string `json:"ota_url"`
}

func deviceSetupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	setup := deviceSetup{
		HAEnabled: aiBool(ctx, "ha_enabled", true),
		Host:      strings.TrimSpace(g.Cfg().MustGet(ctx, "ai.local_host", "").String()),
		Port:      g.Cfg().MustGet(ctx, "ai.local_port", 12800).Int(),
	}
	if validNVSHost(setup.Host) && setup.Port >= 1 && setup.Port <= 65535 {
		setup.OTAURL = fmt.Sprintf("http://%s:%d/xiaozhi/ota/", setup.Host, setup.Port)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(setup)
}

var nvsHostnameLabel = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)

func validNVSHost(host string) bool {
	// The current injector supports IPv4 and DNS/mDNS-style hostnames, not
	// IPv6 or URL input. Do not offer loopback/listen-only addresses for devices.
	if host == "" || len(host) > 253 || strings.Contains(host, ":") || strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.To4() != nil && ip.IsGlobalUnicast()
	}
	if strings.Trim(host, "0123456789.") == "" {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if !nvsHostnameLabel.MatchString(label) {
			return false
		}
	}
	return true
}
