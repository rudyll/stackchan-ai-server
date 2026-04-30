/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

// Package ai implements a transparent WebSocket proxy between StackChan devices
// and the upstream Xiaozhi AI server, with MCP tool injection for Home Assistant.
//
// Message flow:
//
//	Device ↔ [this proxy] ↔ Xiaozhi cloud
//
// The proxy intercepts in two places:
//  1. Device→Upstream: tools/list RESPONSE — inject HA tools into the list.
//  2. Upstream→Device: tools/call REQUEST for HA tools — handle locally via ha-mcp-for-stackchan.
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// HandleWS upgrades the incoming HTTP connection to WebSocket, then runs the
// bidirectional proxy between the device and the real Xiaozhi server.
func HandleWS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := g.Log()

	deviceID := r.Header.Get("Device-Id")

	creds, ok := loadDeviceCreds(deviceID)
	if !ok || creds.URL == "" {
		cfg := g.Cfg()
		creds.URL = cfg.MustGet(ctx, "ai.upstream_ota_url",
			"wss://api.tenclass.net/xiaozhi/ws").String()
		log.Warningf(ctx, "no upstream creds for device %s, using fallback URL", deviceID)
	}

	deviceConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorf(ctx, "ws upgrade device: %v", err)
		return
	}
	defer deviceConn.Close()

	upstreamHdr := http.Header{}
	for key, vals := range r.Header {
		for _, v := range vals {
			upstreamHdr.Add(key, v)
		}
	}
	if creds.Token != "" {
		tok := creds.Token
		if !strings.Contains(tok, " ") {
			tok = "Bearer " + tok
		}
		upstreamHdr.Set("Authorization", tok)
	}

	upstreamConn, _, err := websocket.DefaultDialer.DialContext(ctx, creds.URL, upstreamHdr)
	if err != nil {
		log.Errorf(ctx, "dial upstream xiaozhi: %v", err)
		deviceConn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "upstream unavailable"))
		return
	}
	defer upstreamConn.Close()

	var haClient *haMCPClient
	haClient, err = newHAMCPClient(context.Background())
	if err != nil {
		log.Warningf(ctx, "ha mcp client unavailable, HA tools will not be injected: %v", err)
	} else {
		defer haClient.Close()
		// Pre-fetch HA tool list so IsHATool works immediately.
		if _, err := haClient.ListTools(context.Background()); err != nil {
			log.Warningf(ctx, "failed to prefetch HA tools: %v", err)
		}
	}

	errc := make(chan error, 2)

	// device → upstream: intercept tools/list responses to inject HA tools.
	go func() {
		errc <- pipeDeviceToUpstream(ctx, deviceConn, upstreamConn, haClient)
	}()

	// upstream → device: intercept tools/call requests for HA tools.
	// When a tools/call for an HA tool is detected, the result is sent back
	// to upstreamConn (not deviceConn) since the upstream is the MCP client.
	go func() {
		errc <- pipeUpstreamToDevice(ctx, upstreamConn, deviceConn, haClient)
	}()

	if err := <-errc; err != nil {
		log.Infof(ctx, "ws proxy closed for device %s: %v", deviceID, err)
	}
}

// pipeDeviceToUpstream relays device→upstream messages, injecting HA tools into
// tools/list responses before forwarding.
func pipeDeviceToUpstream(ctx context.Context, device, upstream *websocket.Conn, ha *haMCPClient) error {
	log := g.Log()
	for {
		msgType, msg, err := device.ReadMessage()
		if err != nil {
			return err
		}

		if msgType == websocket.TextMessage && ha != nil {
			if modified, injected := injectHAToolsIntoListResponse(ctx, msg, ha); injected {
				log.Debugf(ctx, "injected HA tools into tools/list response")
				msg = modified
			}
		}

		if err := upstream.WriteMessage(msgType, msg); err != nil {
			return err
		}
	}
}

// pipeUpstreamToDevice relays upstream→device messages, intercepting tools/call
// requests for HA tools and handling them locally instead of forwarding.
// When a tools/call for an HA tool is found, the result is written back to upstream
// (the MCP client side) so the LLM gets the response.
func pipeUpstreamToDevice(ctx context.Context, upstream, device *websocket.Conn, ha *haMCPClient) error {
	log := g.Log()
	for {
		msgType, msg, err := upstream.ReadMessage()
		if err != nil {
			return err
		}

		if msgType == websocket.TextMessage && ha != nil {
			handled, handledErr := handleHAToolCall(ctx, msg, upstream, ha)
			if handledErr != nil {
				log.Warningf(ctx, "ha tool call error: %v", handledErr)
			}
			if handled {
				continue // result sent back to upstream; don't forward request to device
			}
		}

		if err := device.WriteMessage(msgType, msg); err != nil {
			return err
		}
	}
}

// mcpEnvelope is the top-level wrapper for MCP messages on the Xiaozhi WebSocket.
//
//	{"session_id":"...","type":"mcp","payload":{<jsonrpc>}}
type mcpEnvelope struct {
	SessionID string          `json:"session_id,omitempty"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

// injectHAToolsIntoListResponse checks if msg is a tools/list RESULT from the device,
// and if so appends HA tools to the list. Returns the modified message and whether injection happened.
func injectHAToolsIntoListResponse(ctx context.Context, raw []byte, ha *haMCPClient) ([]byte, bool) {
	var env mcpEnvelope
	if err := json.Unmarshal(raw, &env); err != nil || env.Type != "mcp" {
		return raw, false
	}

	var rpc rpcMessage
	if err := json.Unmarshal(env.Payload, &rpc); err != nil {
		return raw, false
	}

	// A tools/list response has no Method and a Result containing a "tools" array.
	if rpc.Method != "" || rpc.Result == nil {
		return raw, false
	}

	var result struct {
		Tools []json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(rpc.Result, &result); err != nil || result.Tools == nil {
		return raw, false
	}

	// Refresh HA tool list (may return cached result from ha_mcp.go).
	haTools, err := ha.ListTools(ctx)
	if err != nil || len(haTools) == 0 {
		return raw, false
	}

	result.Tools = append(result.Tools, haTools...)
	newResult, err := json.Marshal(result)
	if err != nil {
		return raw, false
	}

	rpc.Result = newResult
	newPayload, _ := json.Marshal(rpc)
	env.Payload = newPayload
	modified, err := json.Marshal(env)
	if err != nil {
		return raw, false
	}
	return modified, true
}

// handleHAToolCall checks if msg is a tools/call REQUEST from the upstream for an HA tool.
// If so, calls ha-mcp-for-stackchan and sends the result back to upstream (the MCP client).
// Returns (handled, error).
func handleHAToolCall(ctx context.Context, raw []byte, upstream *websocket.Conn, ha *haMCPClient) (bool, error) {
	var env mcpEnvelope
	if err := json.Unmarshal(raw, &env); err != nil || env.Type != "mcp" {
		return false, nil
	}

	var rpc rpcMessage
	if err := json.Unmarshal(env.Payload, &rpc); err != nil {
		return false, nil
	}

	if rpc.Method != "tools/call" {
		return false, nil
	}

	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(rpc.Params, &params); err != nil {
		return false, nil
	}

	if !ha.IsHATool(params.Name) {
		return false, nil
	}

	// It's an HA tool — call ha-mcp-for-stackchan instead of forwarding to device.
	result, err := ha.CallTool(ctx, params.Name, params.Arguments)
	if err != nil {
		result, _ = json.Marshal(map[string]any{
			"content": []map[string]any{{"type": "text", "text": "HA tool error: " + err.Error()}},
			"isError": true,
		})
	}

	// Send the result back to the upstream (Xiaozhi cloud / LLM) so it
	// receives the tool output in the same session.
	replyRPC := rpcMessage{JSONRPC: "2.0", ID: rpc.ID, Result: result}
	replyPayload, _ := json.Marshal(replyRPC)
	replyEnv := mcpEnvelope{SessionID: env.SessionID, Type: "mcp", Payload: replyPayload}
	replyMsg, _ := json.Marshal(replyEnv)
	return true, upstream.WriteMessage(websocket.TextMessage, replyMsg)
}

// LocalWSURL returns the WebSocket URL for this proxy (used in OTA response).
func LocalWSURL() string {
	cfg := g.Cfg()
	host := cfg.MustGet(context.Background(), "ai.local_host", "127.0.0.1").String()
	port := cfg.MustGet(context.Background(), "ai.local_port", 12800).Int()
	return fmt.Sprintf("ws://%s:%d/xiaozhi/ws", host, port)
}
