/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
)

var otaCtx = gctx.New()

// HandleOTA responds to device OTA checks with a synthetic response pointing to
// our local WebSocket simulator. No upstream Xiaozhi contact is made.
func HandleOTA(w http.ResponseWriter, r *http.Request) {
	cfg := g.Cfg()
	localHost := cfg.MustGet(otaCtx, "ai.local_host", "127.0.0.1").String()
	localPort := cfg.MustGet(otaCtx, "ai.local_port", 12800).Int()

	resp := otaResponse(localHost, localPort)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func otaResponse(localHost string, localPort int) map[string]any {
	return map[string]any{
		"websocket": map[string]any{
			"url":     fmt.Sprintf("ws://%s:%d/xiaozhi/ws", localHost, localPort),
			"token":   "",
			"version": 3,
		},
	}
}
