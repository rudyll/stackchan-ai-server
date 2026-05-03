/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

type haWSClient struct {
	conn    *websocket.Conn
	nextID  atomic.Int64
	writeMu sync.Mutex
	mu      sync.Mutex
	pending map[int64]chan json.RawMessage
}

type haError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func dialHAWebSocket(url, token string) (*haWSClient, error) {
	conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{})
	if err != nil {
		return nil, fmt.Errorf("connect HA WebSocket: %w", err)
	}

	client := &haWSClient{
		conn:    conn,
		pending: make(map[int64]chan json.RawMessage),
	}

	var authReq struct{ Type string `json:"type"` }
	if err := conn.ReadJSON(&authReq); err != nil || authReq.Type != "auth_required" {
		conn.Close()
		return nil, fmt.Errorf("expected auth_required, got: %s", authReq.Type)
	}

	if err := conn.WriteJSON(map[string]string{"type": "auth", "access_token": token}); err != nil {
		conn.Close()
		return nil, fmt.Errorf("send auth: %w", err)
	}

	var authResp struct{ Type string `json:"type"` }
	if err := conn.ReadJSON(&authResp); err != nil || authResp.Type != "auth_ok" {
		conn.Close()
		return nil, fmt.Errorf("HA auth failed (got: %s)", authResp.Type)
	}

	go client.readLoop()
	return client, nil
}

func (c *haWSClient) readLoop() {
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			c.mu.Lock()
			for _, ch := range c.pending {
				close(ch)
			}
			c.pending = nil
			c.mu.Unlock()
			return
		}
		var msg struct {
			ID int64 `json:"id"`
		}
		_ = json.Unmarshal(raw, &msg)
		if msg.ID > 0 {
			c.mu.Lock()
			ch, ok := c.pending[msg.ID]
			if ok {
				delete(c.pending, msg.ID)
			}
			c.mu.Unlock()
			if ok {
				ch <- raw
			}
		}
	}
}

func (c *haWSClient) send(cmd map[string]any) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	cmd["id"] = id

	ch := make(chan json.RawMessage, 1)
	c.mu.Lock()
	if c.pending == nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("HA WebSocket connection closed")
	}
	c.pending[id] = ch
	c.mu.Unlock()

	c.writeMu.Lock()
	err := c.conn.WriteJSON(cmd)
	c.writeMu.Unlock()
	if err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}

	raw, ok := <-ch
	if !ok {
		return nil, fmt.Errorf("HA WebSocket connection closed")
	}

	var resp struct {
		Success bool      `json:"success"`
		Error   *haError  `json:"error"`
		Result  json.RawMessage `json:"result"`
	}
	_ = json.Unmarshal(raw, &resp)
	if !resp.Success && resp.Error != nil {
		return nil, fmt.Errorf("HA %s: %s", resp.Error.Code, resp.Error.Message)
	}
	return resp.Result, nil
}

func (c *haWSClient) Close() {
	c.conn.Close()
}

func (c *haWSClient) GetAreas() ([]map[string]any, error) {
	raw, err := c.send(map[string]any{"type": "config/area_registry/list"})
	if err != nil {
		return nil, err
	}
	var areas []map[string]any
	return areas, json.Unmarshal(raw, &areas)
}

func (c *haWSClient) GetEntityRegistry() ([]map[string]any, error) {
	raw, err := c.send(map[string]any{"type": "config/entity_registry/list"})
	if err != nil {
		return nil, err
	}
	var entities []map[string]any
	return entities, json.Unmarshal(raw, &entities)
}

func (c *haWSClient) GetStates() ([]map[string]any, error) {
	raw, err := c.send(map[string]any{"type": "get_states"})
	if err != nil {
		return nil, err
	}
	var states []map[string]any
	return states, json.Unmarshal(raw, &states)
}

func (c *haWSClient) GetState(entityID string) (map[string]any, error) {
	states, err := c.GetStates()
	if err != nil {
		return nil, err
	}
	for _, s := range states {
		if id, _ := s["entity_id"].(string); id == entityID {
			return s, nil
		}
	}
	return nil, fmt.Errorf("entity not found: %s", entityID)
}

func (c *haWSClient) CallService(domain, service, entityID string, data map[string]any) (json.RawMessage, error) {
	svcData := make(map[string]any)
	for k, v := range data {
		svcData[k] = v
	}
	if entityID != "" {
		svcData["entity_id"] = entityID
	}
	cmd := map[string]any{
		"type":         "call_service",
		"domain":       domain,
		"service":      service,
		"service_data": svcData,
	}
	return c.send(cmd)
}

// CallServiceWithTarget uses the modern HA target field, supporting entity_id, area_id, or device_id.
func (c *haWSClient) CallServiceWithTarget(domain, service string, target map[string]any, data map[string]any) (json.RawMessage, error) {
	cmd := map[string]any{
		"type":    "call_service",
		"domain":  domain,
		"service": service,
	}
	if len(target) > 0 {
		cmd["target"] = target
	}
	if len(data) > 0 {
		cmd["service_data"] = data
	}
	return c.send(cmd)
}
