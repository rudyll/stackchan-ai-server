/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
)

var otaCtx = gctx.New()

// HandleOTA forwards the device's OTA check to the real Xiaozhi server,
// then strips the firmware URL to prevent unwanted OTA updates.
func HandleOTA(w http.ResponseWriter, r *http.Request) {
	cfg := g.Cfg()
	upstreamURL := cfg.MustGet(otaCtx, "ai.upstream_ota_url", "https://api.tenclass.net/xiaozhi/ota/").String()

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

	// Strip firmware URL to prevent unwanted OTA updates.
	if fwRaw, ok := payload["firmware"]; ok {
		var fwCfg map[string]json.RawMessage
		if err := json.Unmarshal(fwRaw, &fwCfg); err == nil {
			emptyURL, _ := json.Marshal("")
			fwCfg["url"] = emptyURL
			newFwRaw, _ := json.Marshal(fwCfg)
			payload["firmware"] = newFwRaw
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
