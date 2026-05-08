/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

// ha_tools.go — Home Assistant tool definitions and dispatcher shared by
// both the OpenAI Realtime and Gemini Live providers.
package ai

import (
	"encoding/json"
	"fmt"
	"strings"
)

// haToolDefs returns the canonical HA tool definitions in JSON-Schema form.
// Used by the OpenAI provider directly and by haGeminiTools() for Gemini.
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
			"name":        "ha_list_scenes",
			"description": "List all activatable items in Home Assistant: scenes, scripts, and automations. Call this when the user wants to activate a scene, run a script, or trigger an automation by name. Returns entity_id, name, domain, and current state.",
			"inputSchema": schema(map[string]any{}),
		},
		{
			"name":        "ha_call_services",
			"description": "Execute one or more Home Assistant service calls for device control: turn on/off, set brightness, adjust temperature, control covers, play media, run scripts, activate scenes, trigger automations, etc. Supports area-wide control.",
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

// sanitizeGeminiSchema deep-copies a JSON-schema-shaped value into the form
// Gemini's Schema validator accepts:
//
//   - drops keys Gemini does not model (additionalProperties, $schema, $ref, ...)
//   - upper-cases `type` values to the proto enum form ("OBJECT", "STRING",
//     "NUMBER", "INTEGER", "BOOLEAN", "ARRAY"). Lowercase JSON-Schema literals
//     ("object", "string") are rejected with a 1007 "invalid argument" close.
//   - rewrites `{type: OBJECT}` nodes that have no `properties` (open object)
//     into `{type: STRING}` with a hint to send a JSON-encoded literal.
//     dispatchHATool then parses the string back to a map.
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
		// Upper-case the type literal Gemini's enum demands.
		if t, ok := out["type"].(string); ok {
			out["type"] = strings.ToUpper(t)
		}
		// Open-object rewrite: OBJECT with no properties → STRING holding JSON.
		if t, _ := out["type"].(string); t == "OBJECT" {
			if _, hasProps := out["properties"]; !hasProps {
				desc, _ := out["description"].(string)
				if desc == "" {
					desc = "JSON object encoded as a string."
				} else {
					desc += " (Send as a JSON-encoded string, e.g. \"{\\\"brightness_pct\\\": 80}\".)"
				}
				return map[string]any{"type": "STRING", "description": desc}
			}
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

// dispatchHATool executes a named HA tool call and returns a result string.
// Called by both the OpenAI and Gemini providers.
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

	case "ha_list_scenes":
		states, err := ha.GetStates()
		if err != nil {
			return "", err
		}
		var results []map[string]any
		for _, s := range states {
			entityID, _ := s["entity_id"].(string)
			domain := strings.SplitN(entityID, ".", 2)[0]
			if domain != "scene" && domain != "script" && domain != "automation" {
				continue
			}
			attrs, _ := s["attributes"].(map[string]any)
			name, _ := attrs["friendly_name"].(string)
			state, _ := s["state"].(string)
			results = append(results, map[string]any{
				"entity_id": entityID,
				"name":      name,
				"domain":    domain,
				"state":     state,
			})
		}
		return mustJSONStr(results), nil

	case "ha_call_services":
		// Re-marshal and parse the calls array (LLM sends it as []interface{}).
		// `data` may arrive as either a map (OpenAI rich schema) or as a JSON
		// string (Gemini, where open-object schemas are rewritten to type:string).
		callsJSON, _ := json.Marshal(args["calls"])
		var calls []struct {
			Domain   string `json:"domain"`
			Service  string `json:"service"`
			EntityID string `json:"entity_id"`
			AreaID   string `json:"area_id"`
			Data     any    `json:"data"`
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
			// Coerce data: accept map directly, or parse JSON string.
			var data map[string]any
			switch d := call.Data.(type) {
			case map[string]any:
				data = d
			case string:
				if d != "" {
					_ = json.Unmarshal([]byte(d), &data)
				}
			}
			_, err := ha.CallServiceWithTarget(call.Domain, call.Service, target, data)
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

	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func mustJSONStr(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
