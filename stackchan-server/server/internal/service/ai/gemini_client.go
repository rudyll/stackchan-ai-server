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

const geminiEndpoint = "wss://generativelanguage.googleapis.com/ws/google.ai.generativelanguage.v1alpha.GenerativeService.BidiGenerateContent"

// geminiSession manages one Gemini Live WS connection.
type geminiSession struct {
	conn    *websocket.Conn
	ha      *haWSClient
	writeMu sync.Mutex
	logCtx  context.Context

	cb RealtimeCallbacks

	// Tracks model speech start so OnStart fires once per turn even though
	// audio arrives in many small modelTurn chunks.
	mu       sync.Mutex
	speaking bool
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
		conn:   conn,
		ha:     ha,
		logCtx: gctx.New(),
		cb:     cb,
	}

	// Setup message — single shot; must be the first frame on the socket.
	setup := map[string]any{
		"setup": map[string]any{
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
			"tools": haGeminiTools(),
		},
	}
	if err := s.send(setup); err != nil {
		conn.Close()
		return nil, fmt.Errorf("gemini setup: %w", err)
	}

	go s.readLoop(ctx)
	return s, nil
}

// AppendAudio sends a 16kHz PCM16 chunk as a realtimeInput media chunk.
func (s *geminiSession) AppendAudio(pcm []int16) error {
	buf := make([]byte, len(pcm)*2)
	for i, v := range pcm {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(v))
	}
	return s.send(map[string]any{
		"realtimeInput": map[string]any{
			"mediaChunks": []map[string]any{{
				"mimeType": "audio/pcm;rate=16000",
				"data":     base64.StdEncoding.EncodeToString(buf),
			}},
		},
	})
}

// CommitAudio is a no-op for Gemini — server VAD always handles end-of-speech.
func (s *geminiSession) CommitAudio() error { return nil }

// CancelResponse interrupts the current model turn. Gemini Live treats a
// new realtimeInput audio chunk as user-initiated barge-in, but if the device
// detects a wake word with no audio yet we emit an explicit clientContent
// turnComplete to stop the model.
func (s *geminiSession) CancelResponse() error {
	return s.send(map[string]any{
		"clientContent": map[string]any{
			"turns":         []map[string]any{},
			"turnComplete":  true,
		},
	})
}

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
func (s *geminiSession) readLoop(ctx context.Context) {
	for {
		_, raw, err := s.conn.ReadMessage()
		if err != nil {
			if ctx.Err() == nil {
				g.Log().Warningf(s.logCtx, "[GM] read error: %v", err)
			}
			return
		}

		var evt map[string]any
		if err := json.Unmarshal(raw, &evt); err != nil {
			continue
		}

		// setupComplete: connection ready.
		if _, ok := evt["setupComplete"]; ok {
			g.Log().Infof(s.logCtx, "[GM] setup complete")
			continue
		}

		// serverContent: model audio / text / turn lifecycle.
		if sc, ok := evt["serverContent"].(map[string]any); ok {
			s.handleServerContent(sc)
			continue
		}

		// toolCall: HA tool invocations.
		if tc, ok := evt["toolCall"].(map[string]any); ok {
			s.handleToolCall(tc)
			continue
		}

		// goAway / errors — treat as fatal-ish.
		if _, ok := evt["goAway"]; ok {
			g.Log().Warningf(s.logCtx, "[GM] server sent goAway")
			return
		}
	}
}

func (s *geminiSession) handleServerContent(sc map[string]any) {
	// modelTurn carries audio/text deltas.
	if mt, ok := sc["modelTurn"].(map[string]any); ok {
		s.startSpeakingOnce()
		parts, _ := mt["parts"].([]any)
		for _, p := range parts {
			part, _ := p.(map[string]any)
			if part == nil {
				continue
			}

			if inline, ok := part["inlineData"].(map[string]any); ok {
				mime, _ := inline["mimeType"].(string)
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

	// inputTranscription: user STT (best-effort, may not be present in all models).
	if it, ok := sc["inputTranscription"].(map[string]any); ok {
		if text, _ := it["text"].(string); text != "" {
			if s.cb.OnSTT != nil {
				s.cb.OnSTT(text)
			}
		}
	}

	// turnComplete: model finished speaking → fire OnStop.
	if done, _ := sc["turnComplete"].(bool); done {
		s.endSpeaking()
	}

	// interrupted: barge-in. Treat as turn end so device gets tts:stop.
	if intr, _ := sc["interrupted"].(bool); intr {
		s.endSpeaking()
	}
}

func (s *geminiSession) handleToolCall(tc map[string]any) {
	calls, _ := tc["functionCalls"].([]any)
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
		args, _ := call["args"].(map[string]any)
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
