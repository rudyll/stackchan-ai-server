/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

// Package ai — Google Gemini Live API client.
// Bridges Xiaozhi WS protocol ↔ Gemini Live (BidiGenerateContent).
// Audio flow: device OPUS (16kHz) → PCM → Gemini → PCM (24kHz) → device OPUS.
//
// Implements RealtimeSession (provider.go) so ws_simulator.go can talk to
// Gemini through the same interface as OpenAI Realtime.
package ai

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gorilla/websocket"
)

// v1beta hosts the Gemini Live model family
// (gemini-2.5-flash-preview-native-audio-dialog and friends). v1alpha returns
// "model not found ... or not supported for bidiGenerate" for these models.
const geminiEndpoint = "wss://generativelanguage.googleapis.com/ws/google.ai.generativelanguage.v1beta.GenerativeService.BidiGenerateContent"

// geminiSession manages one Gemini Live WS connection.
type geminiSession struct {
	conn    *websocket.Conn
	ha      *haWSClient
	writeMu sync.Mutex
	logCtx  context.Context

	cb RealtimeCallbacks

	// Tracks model speech start so OnStart fires once per turn even though
	// audio arrives in many small modelTurn chunks.
	mu        sync.Mutex
	speaking  bool
	audioOnce sync.Once // logs the first AppendAudio call for diagnostics

	// setupDone is closed when Gemini acknowledges the setup message.
	// Any message sent before setupComplete causes a 1007 "invalid argument"
	// because the device can send listen:detect→CancelResponse before
	// setupComplete arrives. Guard all outbound messages except setup itself.
	setupDone chan struct{}
	setupOnce sync.Once // ensures setupDone is closed exactly once
}

// dialGeminiSession opens a Gemini Live websocket, sends the setup message,
// and starts the read loop. Server VAD is always on.
func dialGeminiSession(
	ctx context.Context,
	apiKey, model, voice, sysPrompt string,
	ha *haWSClient,
	cb RealtimeCallbacks,
) (RealtimeSession, error) {
	u := geminiEndpoint + "?key=" + url.QueryEscape(apiKey)
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u, nil)
	if err != nil {
		return nil, fmt.Errorf("gemini dial: %w", err)
	}

	s := &geminiSession{
		conn:      conn,
		ha:        ha,
		logCtx:    gctx.New(),
		cb:        cb,
		setupDone: make(chan struct{}),
	}

	// Setup message — single shot; must be the first frame on the socket.
	// Wire format is camelCase per https://ai.google.dev/api/live.
	cfg := g.Cfg()
	enableTools := cfg.MustGet(gctx.New(), "ai.gemini_enable_tools", true).Bool()
	enableSearch := cfg.MustGet(gctx.New(), "ai.gemini_enable_search", true).Bool()

	setupBody := map[string]any{
		"model": "models/" + model,
		"generationConfig": map[string]any{
			"responseModalities": []string{"AUDIO"},
			"speechConfig": map[string]any{
				"voiceConfig": map[string]any{
					"prebuiltVoiceConfig": map[string]any{
						"voiceName": voice,
					},
				},
			},
		},
		"systemInstruction": map[string]any{
			"parts": []map[string]any{{"text": sysPrompt}},
		},
	}
	if enableTools || enableSearch {
		tools := []map[string]any{}
		if enableTools {
			tools = append(tools, haGeminiTools()...)
		} else {
			g.Log().Infof(s.logCtx, "[GM] HA tools disabled by ai.gemini_enable_tools=false")
		}
		if enableSearch {
			tools = append(tools, map[string]any{"googleSearch": map[string]any{}})
			g.Log().Infof(s.logCtx, "[GM] Google Search enabled")
		}
		setupBody["tools"] = tools
	}

	setup := map[string]any{"setup": setupBody}

	// Dump the exact payload at INFO so it always appears in HA logs —
	// invaluable for diagnosing 1007 "invalid argument" rejections.
	if pretty, err := json.MarshalIndent(setup, "", "  "); err == nil {
		g.Log().Infof(s.logCtx, "[GM] setup payload:\n%s", string(pretty))
	}

	if err := s.send(setup); err != nil {
		conn.Close()
		return nil, fmt.Errorf("gemini setup: %w", err)
	}

	go s.readLoop(ctx)
	return s, nil
}

// AppendAudio sends a 16kHz PCM16 chunk to Gemini's realtime input.
//
// The current RealtimeInput proto exposes typed fields (audio / video / text);
// the older `mediaChunks` array form was an early-preview shape and is now
// rejected with 1007 "invalid argument" — which is exactly what we hit after
// setup completed. Use the typed `audio` Blob.
func (s *geminiSession) AppendAudio(pcm []int16) error {
	// Drop audio that arrives before Gemini acknowledges setup; sending any
	// message before setupComplete causes a 1007 "invalid argument" close.
	select {
	case <-s.setupDone:
	default:
		return nil
	}
	buf := make([]byte, len(pcm)*2)
	for i, v := range pcm {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(v))
	}
	s.audioOnce.Do(func() {
		g.Log().Infof(s.logCtx, "[GM] first audio chunk: samples=%d bytes=%d", len(pcm), len(buf))
	})
	return s.send(map[string]any{
		"realtimeInput": map[string]any{
			"audio": map[string]any{
				"mimeType": "audio/pcm;rate=16000",
				"data":     base64.StdEncoding.EncodeToString(buf),
			},
		},
	})
}

// CommitAudio signals end-of-speech to Gemini via activityEnd.
// Without this hint, Gemini relies solely on server VAD silence detection,
// adding ~500ms latency after the device sends listen:stop.
// activityEnd is the correct per-turn signal (audioStreamEnd closes the whole
// session and causes 1006 unexpected EOF).
func (s *geminiSession) CommitAudio() error {
	select {
	case <-s.setupDone:
	default:
		return nil
	}
	return s.send(map[string]any{
		"realtimeInput": map[string]any{
			"activityEnd": map[string]any{},
		},
	})
}

// CancelResponse is a no-op for Gemini Live.
//
// Gemini has no explicit cancel/interrupt API. The server VAD automatically
// interrupts the model the moment new user audio arrives via AppendAudio.
// Sending clientContent with an empty turns array causes a 1007 "invalid
// argument" close, so we do nothing here and let the next audio chunk
// handle the barge-in.
func (s *geminiSession) CancelResponse() error { return nil }

func (s *geminiSession) Close() {
	s.conn.Close()
}

func (s *geminiSession) send(v any) error {
	data, _ := json.Marshal(v)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteMessage(websocket.TextMessage, data)
}

// readLoop dispatches Gemini server messages onto the generic callbacks.
// Deferred cleanup fires on any exit (normal or error):
//   - endSpeaking sends tts:stop if the model was mid-response
//   - OnClose signals ws_simulator to close the device connection so the
//     device doesn't get stuck waiting for a reply that will never arrive
func (s *geminiSession) readLoop(ctx context.Context) {
	defer func() {
		s.endSpeaking()
		if s.cb.OnClose != nil {
			s.cb.OnClose()
		}
	}()
	for {
		_, raw, err := s.conn.ReadMessage()
		if err != nil {
			if ctx.Err() == nil {
				// Gemini close frames carry a one-line reason that is often
				// the only thing telling us which setup field was rejected.
				// Surface code + verbatim text so users can paste it back.
				if ce, ok := err.(*websocket.CloseError); ok {
					g.Log().Warningf(s.logCtx,
						"[GM] socket closed code=%d reason=%q", ce.Code, ce.Text)
				} else {
					g.Log().Warningf(s.logCtx, "[GM] read error: %v", err)
				}
			}
			return
		}

		var evt map[string]any
		if err := json.Unmarshal(raw, &evt); err != nil {
			continue
		}

		// Server may use either snake_case or camelCase depending on proto JSON
		// flavour the gateway picks. Accept both via geminiField below.

		// setup_complete: connection ready. Ungate all subsequent sends.
		if geminiField(evt, "setup_complete", "setupComplete") != nil {
			g.Log().Infof(s.logCtx, "[GM] setup complete")
			s.setupOnce.Do(func() { close(s.setupDone) })
			continue
		}

		// server_content: model audio / text / turn lifecycle.
		if sc, ok := geminiField(evt, "server_content", "serverContent").(map[string]any); ok {
			s.handleServerContent(sc)
			continue
		}

		// tool_call: HA tool invocations.
		if tc, ok := geminiField(evt, "tool_call", "toolCall").(map[string]any); ok {
			s.handleToolCall(tc)
			continue
		}

		// go_away / errors — treat as fatal-ish.
		if geminiField(evt, "go_away", "goAway") != nil {
			g.Log().Warningf(s.logCtx, "[GM] server sent go_away")
			return
		}

		// Log any other messages for diagnostics — helps diagnose delayed
		// setup rejections that arrive after setupComplete.
		keys := make([]string, 0, len(evt))
		for k := range evt {
			keys = append(keys, k)
		}
		preview := string(raw)
		if len(preview) > 300 {
			preview = preview[:300] + "..."
		}
		g.Log().Warningf(s.logCtx, "[GM] unrecognized message keys=%v raw=%s", keys, preview)
	}
}

// geminiField looks up the first non-nil value at one of the given keys.
// Used to accept both snake_case and camelCase forms in incoming messages.
func geminiField(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return nil
}

func (s *geminiSession) handleServerContent(sc map[string]any) {
	// model_turn carries audio/text deltas.
	if mt, ok := geminiField(sc, "model_turn", "modelTurn").(map[string]any); ok {
		s.startSpeakingOnce()
		parts, _ := mt["parts"].([]any)
		for _, p := range parts {
			part, _ := p.(map[string]any)
			if part == nil {
				continue
			}

			if inline, ok := geminiField(part, "inline_data", "inlineData").(map[string]any); ok {
				mime, _ := geminiField(inline, "mime_type", "mimeType").(string)
				b64, _ := inline["data"].(string)
				if !strings.HasPrefix(mime, "audio/pcm") || b64 == "" {
					continue
				}
				pcmBytes, err := base64.StdEncoding.DecodeString(b64)
				if err != nil || len(pcmBytes) < 2 {
					continue
				}
				pcm := make([]int16, len(pcmBytes)/2)
				for i := range pcm {
					pcm[i] = int16(binary.LittleEndian.Uint16(pcmBytes[i*2:]))
				}
				if s.cb.OnAudio != nil {
					s.cb.OnAudio(pcm)
				}
			}

			if text, ok := part["text"].(string); ok && text != "" {
				if s.cb.OnText != nil {
					s.cb.OnText(text)
				}
			}
		}
	}

	// input_transcription: user STT (best-effort, may not be present in all models).
	if it, ok := geminiField(sc, "input_transcription", "inputTranscription").(map[string]any); ok {
		if text, _ := it["text"].(string); text != "" {
			if s.cb.OnSTT != nil {
				s.cb.OnSTT(text)
			}
		}
	}

	// turn_complete: model finished speaking → fire OnStop.
	if done, _ := geminiField(sc, "turn_complete", "turnComplete").(bool); done {
		s.endSpeaking()
	}

	// interrupted: barge-in. Treat as turn end so device gets tts:stop.
	if intr, _ := sc["interrupted"].(bool); intr {
		s.endSpeaking()
	}
}

func (s *geminiSession) handleToolCall(tc map[string]any) {
	calls, _ := geminiField(tc, "function_calls", "functionCalls").([]any)
	if len(calls) == 0 {
		return
	}
	responses := make([]map[string]any, 0, len(calls))
	for _, c := range calls {
		call, _ := c.(map[string]any)
		if call == nil {
			continue
		}
		callID, _ := call["id"].(string)
		name, _ := call["name"].(string)
		// Gemini sometimes wraps property names in extra quotes
		// (key "\"domain\"" instead of "domain"). Strip them before dispatch
		// so JSON re-marshalling inside dispatchHATool finds the right fields.
		raw, _ := call["args"].(map[string]any)
		args := stripArgKeyQuotes(raw)
		g.Log().Infof(s.logCtx, "[GM] tool=%s args=%v", name, args)

		result, dispErr := dispatchHATool(s.ha, name, args)
		if dispErr != nil {
			result = "error: " + dispErr.Error()
		}
		g.Log().Infof(s.logCtx, "[GM] tool=%s result=%s", name, result)

		responses = append(responses, map[string]any{
			"id":       callID,
			"name":     name,
			"response": map[string]any{"result": result},
		})
	}
	_ = s.send(map[string]any{
		"toolResponse": map[string]any{"functionResponses": responses},
	})
}

// stripArgKeyQuotes recursively strips surrounding double-quote characters from
// map keys in a tool-call args tree.  Gemini Live inconsistently wraps some
// property names in extra quotes (e.g. key `"domain"` instead of `domain`),
// which causes json.Unmarshal in dispatchHATool to see empty struct fields.
func stripArgKeyQuotes(v any) map[string]any {
	m, _ := v.(map[string]any)
	if m == nil {
		return nil
	}
	return stripAnyKeyQuotes(m).(map[string]any)
}

func stripAnyKeyQuotes(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[strings.Trim(k, `"`)] = stripAnyKeyQuotes(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = stripAnyKeyQuotes(e)
		}
		return out
	default:
		return v
	}
}

func (s *geminiSession) startSpeakingOnce() {
	s.mu.Lock()
	first := !s.speaking
	s.speaking = true
	s.mu.Unlock()
	if first {
		g.Log().Infof(s.logCtx, "[GM] response started")
		if s.cb.OnStart != nil {
			s.cb.OnStart()
		}
	}
}

func (s *geminiSession) endSpeaking() {
	s.mu.Lock()
	wasSpeaking := s.speaking
	s.speaking = false
	s.mu.Unlock()
	if wasSpeaking {
		g.Log().Infof(s.logCtx, "[GM] response done")
		if s.cb.OnStop != nil {
			s.cb.OnStop()
		}
	}
}
