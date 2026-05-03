/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

// Package ai — OpenAI Realtime API client.
// Bridges Xiaozhi WS protocol ↔ OpenAI Realtime WS (gpt-realtime-1.5 or compatible).
// Audio flow: device OPUS (16kHz) → PCM → Realtime → PCM (24kHz) → device OPUS.
package ai

import (
	"context"
	"encoding/base64"
	"encoding/binary"
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

const realtimeEndpoint = "wss://api.openai.com/v1/realtime"

// realtimeSession manages one OpenAI Realtime WS connection.
// It is long-lived: one per device WS connection, so conversation history is
// maintained automatically by the model across multiple speak-listen cycles.
type realtimeSession struct {
	conn    *websocket.Conn
	ha      *haWSClient
	writeMu sync.Mutex
	logCtx  context.Context

	// Callbacks are set once at creation and called from the readLoop goroutine.
	onSTT   func(string) // transcription complete
	onText  func(string) // LLM text reply (for logging)
	onAudio func([]int16) // 24kHz PCM chunk from model
	onStart func()        // model began responding (send tts:start)
	onStop  func()        // model finished responding (send tts:stop)
}

// dialRealtimeSession connects to the OpenAI Realtime API, configures the session,
// and starts the background read loop. Callbacks drive the wsSession output.
func dialRealtimeSession(
	ctx context.Context,
	apiKey, model, voice, sysPrompt string,
	ha *haWSClient,
	onSTT, onText func(string),
	onAudio func([]int16),
	onStart, onStop func(),
) (*realtimeSession, error) {
	hdr := http.Header{
		"Authorization": []string{"Bearer " + apiKey},
		"OpenAI-Beta":   []string{"realtime=v1"},
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, realtimeEndpoint+"?model="+model, hdr)
	if err != nil {
		return nil, fmt.Errorf("realtime dial: %w", err)
	}

	s := &realtimeSession{
		conn:    conn,
		ha:      ha,
		logCtx:  gctx.New(),
		onSTT:   onSTT,
		onText:  onText,
		onAudio: onAudio,
		onStart: onStart,
		onStop:  onStop,
	}

	// Inject current time so the model can answer time/date queries.
	now := time.Now().Format("2006-01-02 15:04:05 MST")
	instructions := fmt.Sprintf("Current date/time: %s\n\n%s", now, sysPrompt)

	if err := s.send(map[string]any{
		"type": "session.update",
		"session": map[string]any{
			"modalities":          []string{"text", "audio"},
			"instructions":        instructions,
			"voice":               voice,
			"input_audio_format":  "pcm16",
			"output_audio_format": "pcm16",
			// Enable transcription so the device can display what was said.
			"input_audio_transcription": map[string]any{
				"model": "whisper-1",
			},
			// Server-side VAD: model detects end-of-speech, no client timeout needed.
			"turn_detection": map[string]any{
				"type":                "server_vad",
				"threshold":           0.5,
				"prefix_padding_ms":   300,
				"silence_duration_ms": 600,
			},
			"tools":       haRealtimeTools(),
			"tool_choice": "auto",
		},
	}); err != nil {
		conn.Close()
		return nil, err
	}

	go s.readLoop(ctx)
	return s, nil
}

// AppendAudio sends a PCM16 16kHz mono chunk to the Realtime input buffer.
func (s *realtimeSession) AppendAudio(pcm []int16) error {
	buf := make([]byte, len(pcm)*2)
	for i, v := range pcm {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(v))
	}
	return s.send(map[string]any{
		"type":  "input_audio_buffer.append",
		"audio": base64.StdEncoding.EncodeToString(buf),
	})
}

// CommitAudio manually commits the audio buffer.
// Called on listen:stop as belt-and-suspenders alongside server VAD.
func (s *realtimeSession) CommitAudio() error {
	return s.send(map[string]any{"type": "input_audio_buffer.commit"})
}

// CancelResponse interrupts an in-progress response (e.g. on wake word / abort).
func (s *realtimeSession) CancelResponse() error {
	return s.send(map[string]any{"type": "response.cancel"})
}

// Close shuts down the Realtime WS connection.
func (s *realtimeSession) Close() {
	s.conn.Close()
}

func (s *realtimeSession) send(v any) error {
	data, _ := json.Marshal(v)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteMessage(websocket.TextMessage, data)
}

func (s *realtimeSession) readLoop(ctx context.Context) {
	var textBuf strings.Builder

	for {
		_, raw, err := s.conn.ReadMessage()
		if err != nil {
			if ctx.Err() == nil {
				g.Log().Warningf(s.logCtx, "[RT] read error: %v", err)
			}
			return
		}

		var evt map[string]any
		if err := json.Unmarshal(raw, &evt); err != nil {
			continue
		}

		switch evtType, _ := evt["type"].(string); evtType {

		case "session.created", "session.updated":
			g.Log().Infof(s.logCtx, "[RT] %s", evtType)

		case "input_audio_buffer.speech_started":
			g.Log().Infof(s.logCtx, "[RT] speech detected")

		case "input_audio_buffer.speech_stopped":
			g.Log().Infof(s.logCtx, "[RT] speech ended")

		case "conversation.item.input_audio_transcription.completed":
			transcript, _ := evt["transcript"].(string)
			if t := strings.TrimSpace(transcript); t != "" {
				g.Log().Infof(s.logCtx, "[RT] STT: %q", t)
				if s.onSTT != nil {
					s.onSTT(t)
				}
			}

		case "response.created":
			g.Log().Infof(s.logCtx, "[RT] response started")
			textBuf.Reset()
			if s.onStart != nil {
				s.onStart()
			}

		case "response.audio.delta":
			b64, _ := evt["delta"].(string)
			if b64 == "" {
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
			if s.onAudio != nil {
				s.onAudio(pcm)
			}

		case "response.text.delta":
			chunk, _ := evt["delta"].(string)
			textBuf.WriteString(chunk)

		case "response.text.done":
			if text := strings.TrimSpace(textBuf.String()); text != "" {
				g.Log().Infof(s.logCtx, "[RT] LLM: %q", text)
				if s.onText != nil {
					s.onText(text)
				}
			}
			textBuf.Reset()

		case "response.output_item.done":
			// Handle HA tool calls.
			item, _ := evt["item"].(map[string]any)
			if item == nil || item["type"] != "function_call" {
				continue
			}
			callID, _ := item["call_id"].(string)
			funcName, _ := item["name"].(string)
			argsStr, _ := item["arguments"].(string)

			var args map[string]any
			_ = json.Unmarshal([]byte(argsStr), &args)
			g.Log().Infof(s.logCtx, "[RT] tool=%s args=%v", funcName, args)

			result, dispErr := dispatchHATool(s.ha, funcName, args)
			if dispErr != nil {
				result = "error: " + dispErr.Error()
			}
			g.Log().Infof(s.logCtx, "[RT] tool=%s result=%s", funcName, result)

			// Return result to model and request it to continue responding.
			_ = s.send(map[string]any{
				"type": "conversation.item.create",
				"item": map[string]any{
					"type":    "function_call_output",
					"call_id": callID,
					"output":  result,
				},
			})
			_ = s.send(map[string]any{"type": "response.create"})

		case "response.done":
			g.Log().Infof(s.logCtx, "[RT] response done")
			if s.onStop != nil {
				s.onStop()
			}

		case "error":
			errObj, _ := evt["error"].(map[string]any)
			g.Log().Warningf(s.logCtx, "[RT] API error: %v", errObj)
		}
	}
}

// haRealtimeTools converts tool defs to the Realtime API flat format
// (different from Chat Completions which wraps them under "function": {...}).
func haRealtimeTools() []map[string]any {
	defs := haToolDefs()
	tools := make([]map[string]any, 0, len(defs))
	for _, t := range defs {
		tools = append(tools, map[string]any{
			"type":        "function",
			"name":        t["name"],
			"description": t["description"],
			"parameters":  t["inputSchema"],
		})
	}
	return tools
}
