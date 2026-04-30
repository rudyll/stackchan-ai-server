/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
)

var otaCtx = gctx.New()

// HandleOTA forwards the device's OTA check to the real Xiaozhi server,
// extracts the WebSocket credentials, stores them keyed by Device-Id,
// then rewrites the websocket URL to point at our local proxy before responding.
func HandleOTA(w http.ResponseWriter, r *http.Request) {
	cfg := g.Cfg()
	upstreamURL := cfg.MustGet(otaCtx, "ai.upstream_ota_url", "https://api.tenclass.net/xiaozhi/ota/").String()
	localHost := cfg.MustGet(otaCtx, "ai.local_host", "127.0.0.1").String()
	localPort := cfg.MustGet(otaCtx, "ai.local_port", 12800).Int()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusInternalServerError)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, io.NopCloser(bytesReader(body)))
	if err != nil {
		http.Error(w, "failed to build upstream request", http.StatusInternalServerError)
		return
	}
	// Forward all device headers so the real server can identify the device.
	for key, vals := range r.Header {
		for _, v := range vals {
			req.Header.Add(key, v)
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "upstream OTA request failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "failed to read upstream response", http.StatusInternalServerError)
		return
	}

	// Parse JSON to extract and override the websocket section.
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(respBody, &payload); err != nil {
		// If the upstream response isn't JSON, pass it through unchanged.
		copyHeaders(w, resp)
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(respBody)
		return
	}

	if wsRaw, ok := payload["websocket"]; ok {
		var wsCfg map[string]json.RawMessage
		if err := json.Unmarshal(wsRaw, &wsCfg); err == nil {
			// Extract real credentials for later use in the WS proxy.
			deviceID := r.Header.Get("Device-Id")
			if deviceID != "" {
				creds := upstreamCreds{Version: 3}
				if urlRaw, ok := wsCfg["url"]; ok {
					_ = json.Unmarshal(urlRaw, &creds.URL)
				}
				if tokRaw, ok := wsCfg["token"]; ok {
					_ = json.Unmarshal(tokRaw, &creds.Token)
				}
				if verRaw, ok := wsCfg["version"]; ok {
					_ = json.Unmarshal(verRaw, &creds.Version)
				}
				storeDeviceCreds(deviceID, creds)
			}

			// Replace the websocket URL with our local proxy URL.
			localWsURL := fmt.Sprintf("ws://%s:%d/xiaozhi/ws", localHost, localPort)
			urlBytes, _ := json.Marshal(localWsURL)
			wsCfg["url"] = urlBytes

			newWsRaw, _ := json.Marshal(wsCfg)
			payload["websocket"] = newWsRaw
		}
	}

	modified, err := json.Marshal(payload)
	if err != nil {
		copyHeaders(w, resp)
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(respBody)
		return
	}

	copyHeaders(w, resp)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(modified)
}

func copyHeaders(w http.ResponseWriter, resp *http.Response) {
	for key, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(key, v)
		}
	}
}

// bytesReader wraps a byte slice as an io.Reader (avoids importing bytes package alias).
type bytesBuf struct {
	data []byte
	pos  int
}

func (b *bytesBuf) Read(p []byte) (n int, err error) {
	if b.pos >= len(b.data) {
		return 0, io.EOF
	}
	n = copy(p, b.data[b.pos:])
	b.pos += n
	return n, nil
}

func bytesReader(data []byte) io.Reader {
	return &bytesBuf{data: data}
}
