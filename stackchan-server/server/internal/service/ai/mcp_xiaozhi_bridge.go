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

	// No read deadline — the relay sends nothing until a device session is active.
	// Gorilla's default ping handler already sends pong frames automatically.

	// Write mutex — ping goroutine and message handler both write to conn.
	var writeMu sync.Mutex
	safeWrite := func(data []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteMessage(websocket.TextMessage, data)
	}

	// Send a WebSocket ping every 50 s to keep the relay from closing idle connections.
	// (Matches the interval used by the reference ha-mcp-for-xiaozhi implementation.)
	pingStop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(50 * time.Second)
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
	schema := func(props map[string]any, required ...string) map[string]any {
		s := map[string]any{"type": "object", "properties": props}
		if len(required) > 0 {
			s["required"] = required
		}
		return s
	}

	// ha_call_services item schema — one service call in a batch.
	callItemSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"domain":    str("Service domain: light, switch, climate, cover, fan, media_player, script, homeassistant, etc."),
			"service":   str("Service name: turn_on, turn_off, toggle, set_temperature, volume_set, set_cover_position, play_media, etc."),
			"entity_id": str("Target entity ID (e.g. light.living_room). Omit if using area_id."),
			"area_id":   str("Target area ID from ha_list_areas. Controls all matching entities in that area."),
			"data": map[string]any{
				"type":                 "object",
				"description":          "Extra service parameters, e.g. {\"brightness_pct\": 80} for lights, {\"temperature\": 26} for climate, {\"volume_level\": 0.5} for media_player.",
				"additionalProperties": true,
			},
		},
		"required": []string{"domain", "service"},
	}

	return []map[string]any{
		{
			"name":        "ha_list_areas",
			"description": "List all areas (rooms/zones) in Home Assistant with their IDs and names. Call this first when you need to target a room.",
			"inputSchema": schema(map[string]any{}),
		},
		{
			"name":        "ha_search_entities",
			"description": "Search entities by keyword, area, domain, or device_class. Returns entity_id, name, state, area, and attributes.",
			"inputSchema": schema(map[string]any{
				"keyword":      str("Name or entity_id substring to search (case-insensitive)"),
				"area_id":      str("Filter by area ID (from ha_list_areas)"),
				"domain":       str("Filter by domain: light, switch, sensor, climate, media_player, cover, fan, input_boolean, etc."),
				"device_class": str("Filter by device class: motion, door, window, temperature, humidity, moisture, illuminance, etc."),
			}),
		},
		{
			"name":        "ha_get_state",
			"description": "Get the current state and all attributes of a specific entity.",
			"inputSchema": schema(map[string]any{
				"entity_id": str("Entity ID (e.g. light.living_room, climate.bedroom)"),
			}, "entity_id"),
		},
		{
			"name":        "ha_call_services",
			"description": "Execute one or more Home Assistant service calls for device control: turn on/off, set brightness, adjust temperature, control covers, play media, run scripts, etc. Supports area-wide control.",
			"inputSchema": schema(map[string]any{
				"calls": map[string]any{
					"type":        "array",
					"description": "List of service calls to execute. Can include multiple calls in one request.",
					"items":       callItemSchema,
				},
			}, "calls"),
		},
	}
}

// haGeminiTools converts haToolDefs() into Gemini Live's tool schema:
// a single tool entry containing an array of functionDeclarations.
// Schema is otherwise the same JSON-schema shape OpenAI uses, just keyed as
// "parameters" and wrapped under {functionDeclarations: [...]}.
//
// Gemini's schema validator is stricter than OpenAI's: it rejects standard
// JSON-schema keys like "additionalProperties", "$schema", "$ref". We strip
// those recursively so OpenAI keeps its richer schema while Gemini gets a
// clean subset.
func haGeminiTools() []map[string]any {
	defs := haToolDefs()
	decls := make([]map[string]any, 0, len(defs))
	for _, t := range defs {
		decls = append(decls, map[string]any{
			"name":        t["name"],
			"description": t["description"],
			"parameters":  sanitizeGeminiSchema(t["inputSchema"]),
		})
	}
	return []map[string]any{{"functionDeclarations": decls}}
}

// sanitizeGeminiSchema deep-copies a JSON-schema-shaped value, dropping keys
// Gemini's tool validator does not accept.
func sanitizeGeminiSchema(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			switch k {
			case "additionalProperties", "$schema", "$ref", "definitions", "$defs":
				continue
			}
			out[k] = sanitizeGeminiSchema(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = sanitizeGeminiSchema(e)
		}
		return out
	case []map[string]any:
		out := make([]map[string]any, len(x))
		for i, e := range x {
			if m, ok := sanitizeGeminiSchema(e).(map[string]any); ok {
				out[i] = m
			}
		}
		return out
	default:
		return v
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
		deviceClassFilter := strings.ToLower(strVal("device_class"))

		// Build area_id → area_name map.
		areaNames := map[string]string{}
		if areas, err := ha.GetAreas(); err == nil {
			for _, a := range areas {
				id, _ := a["area_id"].(string)
				nm, _ := a["name"].(string)
				areaNames[id] = nm
			}
		}

		// Build entity_id → area_id map from entity registry.
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
			if deviceClassFilter != "" {
				dc, _ := attrs["device_class"].(string)
				if strings.ToLower(dc) != deviceClassFilter {
					continue
				}
			}
			if keyword != "" {
				haystack := strings.ToLower(entityID + " " + friendlyName)
				if !strings.Contains(haystack, keyword) {
					continue
				}
			}

			// Include a small subset of useful attributes so the LLM can reason about device state.
			usefulAttrs := map[string]any{}
			for _, k := range []string{"brightness", "color_temp_kelvin", "rgb_color", "temperature",
				"current_temperature", "hvac_mode", "fan_mode", "volume_level", "media_title",
				"position", "percentage", "device_class", "unit_of_measurement"} {
				if v, ok := attrs[k]; ok {
					usefulAttrs[k] = v
				}
			}

			results = append(results, map[string]any{
				"entity_id":  entityID,
				"name":       friendlyName,
				"state":      state,
				"domain":     domain,
				"area_id":    areaID,
				"area_name":  areaNames[areaID],
				"attributes": usefulAttrs,
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

	case "ha_call_services":
		// Re-marshal and parse the calls array (LLM sends it as []interface{}).
		callsJSON, _ := json.Marshal(args["calls"])
		var calls []struct {
			Domain   string         `json:"domain"`
			Service  string         `json:"service"`
			EntityID string         `json:"entity_id"`
			AreaID   string         `json:"area_id"`
			Data     map[string]any `json:"data"`
		}
		if err := json.Unmarshal(callsJSON, &calls); err != nil || len(calls) == 0 {
			return "", fmt.Errorf("calls must be a non-empty array")
		}

		results := make([]string, 0, len(calls))
		for _, call := range calls {
			if call.Domain == "" || call.Service == "" {
				results = append(results, "error: domain and service required")
				continue
			}
			target := map[string]any{}
			if call.EntityID != "" {
				target["entity_id"] = call.EntityID
			}
			if call.AreaID != "" {
				target["area_id"] = call.AreaID
			}
			_, err := ha.CallServiceWithTarget(call.Domain, call.Service, target, call.Data)
			if err != nil {
				results = append(results, fmt.Sprintf("%s.%s: error: %v", call.Domain, call.Service, err))
			} else {
				label := call.EntityID
				if label == "" {
					label = "area:" + call.AreaID
				}
				results = append(results, fmt.Sprintf("%s.%s %s: OK", call.Domain, call.Service, label))
			}
		}
		return strings.Join(results, "\n"), nil

	// Keep legacy single-call tools for backward compatibility with MCP bridge.
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
		value, ok := args["value"]
		if entityID == "" || !ok {
			return "", fmt.Errorf("entity_id and value required")
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
