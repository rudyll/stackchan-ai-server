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
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gorilla/websocket"
)

var bridgeCtx = gctx.New()

// StartXiaozhiBridge connects to the Xiaozhi MCP relay endpoint as an MCP server,
// exposing 7 slim HA tools. Runs in a background goroutine with auto-reconnect.
func StartXiaozhiBridge() {
	go func() {
		for {
			ctx := bridgeCtx
			cfg := g.Cfg()
			xiaozhiURL := cfg.MustGet(ctx, "ai.xiaozhi_mcp_url", "").String()
			haWSURL := cfg.MustGet(ctx, "ai.ha_ws_url", "ws://homeassistant:8123/api/websocket").String()
			haToken := cfg.MustGet(ctx, "ai.ha_mcp_token", "").String()

			if xiaozhiURL == "" {
				time.Sleep(30 * time.Second)
				continue
			}

			if err := runBridgeSession(ctx, xiaozhiURL, haWSURL, haToken); err != nil {
				g.Log().Warningf(ctx, "Xiaozhi MCP bridge ended: %v — reconnecting in 30s", err)
			}
			time.Sleep(30 * time.Second)
		}
	}()
}

func runBridgeSession(ctx context.Context, xiaozhiURL, haWSURL, haToken string) error {
	ha, err := dialHAWebSocket(haWSURL, haToken)
	if err != nil {
		return fmt.Errorf("connect HA: %w", err)
	}
	defer ha.Close()

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, xiaozhiURL, http.Header{})
	if err != nil {
		return fmt.Errorf("connect Xiaozhi MCP relay: %w", err)
	}
	defer conn.Close()
	g.Log().Infof(ctx, "Xiaozhi MCP bridge connected to %s", xiaozhiURL)

	// Respond to server pings automatically (gorilla doesn't do this by default).
	conn.SetPingHandler(func(data string) error {
		return conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(5*time.Second))
	})
	// Reset read deadline on each pong so the ping loop below keeps things alive.
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	// Write mutex — ping goroutine and message handler both write to conn.
	var writeMu sync.Mutex
	safeWrite := func(data []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteMessage(websocket.TextMessage, data)
	}

	// Send a WebSocket ping every 20 s to keep the relay from closing idle connections.
	pingStop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				writeMu.Lock()
				_ = conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
				writeMu.Unlock()
			case <-pingStop:
				return
			}
		}
	}()
	defer close(pingStop)

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      any             `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(msgBytes, &req); err != nil {
			continue
		}

		// Notifications (no id) — no response needed.
		if req.ID == nil {
			if req.Method == "notifications/initialized" {
				g.Log().Debugf(ctx, "Xiaozhi MCP: client initialized")
			}
			continue
		}

		var resp []byte
		switch req.Method {
		case "initialize":
			resp = buildRPCResult(req.ID, map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "ha-bridge", "version": "1.0"},
			})
		case "tools/list":
			resp = buildRPCResult(req.ID, map[string]any{"tools": haToolDefs()})
		case "tools/call":
			var p struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			if err := json.Unmarshal(req.Params, &p); err != nil {
				resp = buildRPCError(req.ID, "invalid params")
			} else {
				content, toolErr := dispatchHATool(ha, p.Name, p.Arguments)
				if toolErr != nil {
					resp = buildRPCResult(req.ID, map[string]any{
						"content": []any{toolErrorContent(toolErr.Error())},
						"isError": true,
					})
				} else {
					resp = buildRPCResult(req.ID, map[string]any{
						"content": []any{toolTextContent(content)},
					})
				}
			}
		default:
			resp = buildRPCError(req.ID, fmt.Sprintf("unknown method: %s", req.Method))
		}

		if err := safeWrite(resp); err != nil {
			return fmt.Errorf("write: %w", err)
		}
	}
}

func haToolDefs() []map[string]any {
	str := func(desc string) map[string]any {
		return map[string]any{"type": "string", "description": desc}
	}
	num := func(desc string) map[string]any {
		return map[string]any{"type": "number", "description": desc}
	}
	schema := func(props map[string]any, required ...string) map[string]any {
		s := map[string]any{"type": "object", "properties": props}
		if len(required) > 0 {
			s["required"] = required
		}
		return s
	}

	return []map[string]any{
		{
			"name":        "ha_list_areas",
			"description": "List all areas (rooms/zones) in Home Assistant with their IDs and names.",
			"inputSchema": schema(map[string]any{}),
		},
		{
			"name":        "ha_search_entities",
			"description": "Search entities by keyword, area, or domain. Returns entity_id, friendly name, state, and area.",
			"inputSchema": schema(map[string]any{
				"keyword": str("Name or entity_id substring to search (case-insensitive)"),
				"area_id": str("Filter by area ID (from ha_list_areas)"),
				"domain":  str("Filter by domain: light, switch, sensor, climate, media_player, cover, fan, etc."),
			}),
		},
		{
			"name":        "ha_get_state",
			"description": "Get current state and attributes of a specific entity.",
			"inputSchema": schema(map[string]any{
				"entity_id": str("Entity ID (e.g. light.living_room)"),
			}, "entity_id"),
		},
		{
			"name":        "ha_turn_on",
			"description": "Turn on a device (light, switch, fan, script, etc.).",
			"inputSchema": schema(map[string]any{
				"entity_id": str("Entity ID to turn on"),
			}, "entity_id"),
		},
		{
			"name":        "ha_turn_off",
			"description": "Turn off a device.",
			"inputSchema": schema(map[string]any{
				"entity_id": str("Entity ID to turn off"),
			}, "entity_id"),
		},
		{
			"name":        "ha_set_value",
			"description": "Set a numeric value: light brightness (0-255), climate temperature (°C), media_player volume (0-1), cover position (0-100), fan speed percentage (0-100).",
			"inputSchema": schema(map[string]any{
				"entity_id": str("Entity ID"),
				"value":     num("Numeric value appropriate for the entity type"),
			}, "entity_id", "value"),
		},
		{
			"name":        "ha_call_service",
			"description": "Call any Home Assistant service directly.",
			"inputSchema": schema(map[string]any{
				"domain":    str("Service domain (e.g. light, media_player)"),
				"service":   str("Service name (e.g. turn_on, play_media)"),
				"entity_id": str("Target entity ID (optional)"),
				"data":      str("JSON string of additional service data (optional)"),
			}, "domain", "service"),
		},
	}
}

func dispatchHATool(ha *haWSClient, name string, args map[string]any) (string, error) {
	strVal := func(key string) string {
		v, _ := args[key].(string)
		return v
	}

	switch name {
	case "ha_list_areas":
		areas, err := ha.GetAreas()
		if err != nil {
			return "", err
		}
		out := make([]map[string]any, 0, len(areas))
		for _, a := range areas {
			out = append(out, map[string]any{
				"area_id": a["area_id"],
				"name":    a["name"],
			})
		}
		return mustJSONStr(out), nil

	case "ha_search_entities":
		keyword := strings.ToLower(strVal("keyword"))
		areaFilter := strVal("area_id")
		domainFilter := strVal("domain")

		// Build area_id → area_name map
		areaNames := map[string]string{}
		if areas, err := ha.GetAreas(); err == nil {
			for _, a := range areas {
				id, _ := a["area_id"].(string)
				nm, _ := a["name"].(string)
				areaNames[id] = nm
			}
		}

		// Build entity_id → area_id map from entity registry
		entityArea := map[string]string{}
		if reg, err := ha.GetEntityRegistry(); err == nil {
			for _, e := range reg {
				eid, _ := e["entity_id"].(string)
				aid, _ := e["area_id"].(string)
				if eid != "" {
					entityArea[eid] = aid
				}
			}
		}

		states, err := ha.GetStates()
		if err != nil {
			return "", err
		}

		var results []map[string]any
		for _, s := range states {
			entityID, _ := s["entity_id"].(string)
			attrs, _ := s["attributes"].(map[string]any)
			friendlyName, _ := attrs["friendly_name"].(string)
			state, _ := s["state"].(string)
			domain := strings.SplitN(entityID, ".", 2)[0]
			areaID := entityArea[entityID]

			if domainFilter != "" && domain != domainFilter {
				continue
			}
			if areaFilter != "" && areaID != areaFilter {
				continue
			}
			if keyword != "" {
				haystack := strings.ToLower(entityID + " " + friendlyName)
				if !strings.Contains(haystack, keyword) {
					continue
				}
			}

			results = append(results, map[string]any{
				"entity_id": entityID,
				"name":      friendlyName,
				"state":     state,
				"domain":    domain,
				"area_id":   areaID,
				"area_name": areaNames[areaID],
			})
		}
		return mustJSONStr(results), nil

	case "ha_get_state":
		entityID := strVal("entity_id")
		if entityID == "" {
			return "", fmt.Errorf("entity_id required")
		}
		state, err := ha.GetState(entityID)
		if err != nil {
			return "", err
		}
		return mustJSONStr(state), nil

	case "ha_turn_on":
		entityID := strVal("entity_id")
		if entityID == "" {
			return "", fmt.Errorf("entity_id required")
		}
		_, err := ha.CallService("homeassistant", "turn_on", entityID, nil)
		if err != nil {
			return "", err
		}
		return "OK", nil

	case "ha_turn_off":
		entityID := strVal("entity_id")
		if entityID == "" {
			return "", fmt.Errorf("entity_id required")
		}
		_, err := ha.CallService("homeassistant", "turn_off", entityID, nil)
		if err != nil {
			return "", err
		}
		return "OK", nil

	case "ha_set_value":
		entityID := strVal("entity_id")
		if entityID == "" {
			return "", fmt.Errorf("entity_id required")
		}
		value, ok := args["value"]
		if !ok {
			return "", fmt.Errorf("value required")
		}
		domain := strings.SplitN(entityID, ".", 2)[0]
		var svcDomain, svcName string
		data := map[string]any{}
		switch domain {
		case "light":
			svcDomain, svcName = "light", "turn_on"
			data["brightness"] = value
		case "climate":
			svcDomain, svcName = "climate", "set_temperature"
			data["temperature"] = value
		case "media_player":
			svcDomain, svcName = "media_player", "volume_set"
			data["volume_level"] = value
		case "cover":
			svcDomain, svcName = "cover", "set_cover_position"
			data["position"] = value
		case "fan":
			svcDomain, svcName = "fan", "set_percentage"
			data["percentage"] = value
		default:
			return "", fmt.Errorf("ha_set_value: unsupported domain %s", domain)
		}
		_, err := ha.CallService(svcDomain, svcName, entityID, data)
		if err != nil {
			return "", err
		}
		return "OK", nil

	case "ha_call_service":
		domain := strVal("domain")
		service := strVal("service")
		if domain == "" || service == "" {
			return "", fmt.Errorf("domain and service required")
		}
		entityID := strVal("entity_id")
		var data map[string]any
		if rawData := strVal("data"); rawData != "" {
			_ = json.Unmarshal([]byte(rawData), &data)
		}
		result, err := ha.CallService(domain, service, entityID, data)
		if err != nil {
			return "", err
		}
		if result == nil {
			return "OK", nil
		}
		return string(result), nil

	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func buildRPCResult(id any, result any) []byte {
	return mustJSON(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
}

func buildRPCError(id any, msg string) []byte {
	return mustJSON(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   map[string]any{"code": -32600, "message": msg},
	})
}

func toolTextContent(text string) map[string]any {
	return map[string]any{"type": "text", "text": text}
}

func toolErrorContent(errMsg string) map[string]any {
	return map[string]any{"type": "text", "text": errMsg}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func mustJSONStr(v any) string {
	return string(mustJSON(v))
}
