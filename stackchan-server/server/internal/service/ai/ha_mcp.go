/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gorilla/websocket"
)

// haMCPClient is a minimal MCP-over-WebSocket client for ha-mcp-for-stackchan.
type haMCPClient struct {
	conn      *websocket.Conn
	nextID    atomic.Int64
	mu        sync.Mutex
	pending   map[int64]chan json.RawMessage
	toolNames map[string]struct{} // cached set of tool names from ha-mcp-for-stackchan
}

type jsonRPC struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

// newHAMCPClient dials the HA MCP WebSocket and performs the MCP initialize handshake.
func newHAMCPClient(ctx context.Context) (*haMCPClient, error) {
	cfg := g.Cfg()
	haURL := cfg.MustGet(ctx, "ai.ha_mcp_url", "").String()
	haToken := cfg.MustGet(ctx, "ai.ha_mcp_token", "").String()

	if haURL == "" {
		return nil, fmt.Errorf("ai.ha_mcp_url not configured")
	}

	hdr := http.Header{}
	if haToken != "" {
		hdr.Set("Authorization", "Bearer "+haToken)
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, haURL, hdr)
	if err != nil {
		return nil, fmt.Errorf("dial ha mcp: %w", err)
	}

	c := &haMCPClient{
		conn:    conn,
		pending: make(map[int64]chan json.RawMessage),
	}

	go c.readLoop()

	initParams, _ := json.Marshal(map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "stackchan-proxy", "version": "1.0"},
	})
	if _, err := c.call(ctx, "initialize", initParams); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ha mcp initialize: %w", err)
	}

	return c, nil
}

func (c *haMCPClient) readLoop() {
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			c.mu.Lock()
			for _, ch := range c.pending {
				close(ch)
			}
			c.pending = nil
			c.mu.Unlock()
			return
		}
		var rpc jsonRPC
		if err := json.Unmarshal(msg, &rpc); err != nil {
			continue
		}
		c.mu.Lock()
		ch, ok := c.pending[rpc.ID]
		if ok {
			result := rpc.Result
			if rpc.Error != nil {
				result = rpc.Error
			}
			ch <- result
			delete(c.pending, rpc.ID)
		}
		c.mu.Unlock()
	}
}

func (c *haMCPClient) call(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	ch := make(chan json.RawMessage, 1)

	c.mu.Lock()
	if c.pending == nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("ha mcp connection closed")
	}
	c.pending[id] = ch
	c.mu.Unlock()

	msg, _ := json.Marshal(jsonRPC{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	})
	if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}

	select {
	case result, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("ha mcp connection closed")
		}
		return result, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case <-time.After(10 * time.Second):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("ha mcp call timeout")
	}
}

// ListTools fetches tools from ha-mcp-for-stackchan and caches their names.
func (c *haMCPClient) ListTools(ctx context.Context) ([]json.RawMessage, error) {
	result, err := c.call(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Tools []json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse tools/list response: %w", err)
	}

	// Cache tool names so we can identify HA tools during tools/call routing.
	names := make(map[string]struct{}, len(resp.Tools))
	for _, raw := range resp.Tools {
		var t struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(raw, &t); err == nil && t.Name != "" {
			names[t.Name] = struct{}{}
		}
	}
	c.mu.Lock()
	c.toolNames = names
	c.mu.Unlock()

	return resp.Tools, nil
}

// IsHATool reports whether the named tool was listed by ha-mcp-for-stackchan.
func (c *haMCPClient) IsHATool(name string) bool {
	c.mu.Lock()
	_, ok := c.toolNames[name]
	c.mu.Unlock()
	return ok
}

// CallTool calls a tool on ha-mcp-for-stackchan and returns the raw JSON-RPC result.
func (c *haMCPClient) CallTool(ctx context.Context, name string, arguments json.RawMessage) (json.RawMessage, error) {
	params, _ := json.Marshal(map[string]any{
		"name":      name,
		"arguments": arguments,
	})
	return c.call(ctx, "tools/call", params)
}

func (c *haMCPClient) Close() {
	c.conn.Close()
}
