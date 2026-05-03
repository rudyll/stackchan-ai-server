/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

// Package ai implements a Xiaozhi WebSocket protocol v3 simulator backed by
// the OpenAI Realtime API (gpt-realtime-1.5).
//
// Audio path (streaming, no batching):
//
//	device OPUS (16kHz) → PCM → Realtime WS input buffer
//	                              ↓  server VAD detects speech end
//	                         Realtime WS output (PCM 24kHz, streaming)
//	                              ↓
//	                    opusStreamEncoder → device OPUS frames
package ai

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	opus "gopkg.in/hraban/opus.v2"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type wsSession struct {
	conn     *websocket.Conn
	deviceID string
	rt       *realtimeSession
	opusDec  *opus.Decoder // device input decoder (16kHz, reset per utterance)

	mu          sync.Mutex         // protects opusEnc and isListening
	opusEnc     *opusStreamEncoder // non-nil only while model is speaking
	isListening bool

	writeMu sync.Mutex // serialises WebSocket writes
}

// HandleWS upgrades the connection and runs a Xiaozhi v3 protocol session
// backed by OpenAI Realtime API.
func HandleWS(w http.ResponseWriter, r *http.Request) {
	ctx := gctx.New()
	cfg := g.Cfg()

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		g.Log().Errorf(ctx, "ws upgrade: %v", err)
		return
	}

	haURL := cfg.MustGet(ctx, "ai.ha_ws_url", "ws://homeassistant:8123/api/websocket").String()
	haToken := cfg.MustGet(ctx, "ai.ha_mcp_token", "").String()
	apiKey := cfg.MustGet(ctx, "ai.openai_api_key", "").String()
	rtModel := cfg.MustGet(ctx, "ai.openai_realtime_model", "gpt-realtime-1.5").String()
	voice := cfg.MustGet(ctx, "ai.openai_tts_voice", "alloy").String()
	sysPrompt := cfg.MustGet(ctx, "ai.system_prompt", "You are StackChan, a friendly robot assistant.").String()

	deviceID := r.Header.Get("Device-Id")
	g.Log().Infof(ctx, "[WS] device=%s connecting HA at %s", deviceID, haURL)

	ha, err := dialHAWebSocket(haURL, haToken)
	if err != nil {
		g.Log().Warningf(ctx, "[WS] device=%s HA connect failed: %v", deviceID, err)
		conn.Close()
		return
	}
	g.Log().Infof(ctx, "[WS] device=%s HA connected", deviceID)

	opusDec, err := newOpusDecoder()
	if err != nil {
		g.Log().Errorf(ctx, "[WS] device=%s opus decoder init: %v", deviceID, err)
		conn.Close()
		ha.Close()
		return
	}

	s := &wsSession{
		conn:     conn,
		deviceID: deviceID,
		opusDec:  opusDec,
	}

	// Wire Realtime callbacks → device WebSocket writes.
	rt, err := dialRealtimeSession(ctx, apiKey, rtModel, voice, sysPrompt, ha,

		func(text string) { // onSTT: display what the user said on the device
			_ = s.sendJSON(map[string]any{"type": "stt", "text": text})
		},

		func(text string) { // onText: LLM text reply (logged, device shows via TTS)
			g.Log().Infof(ctx, "[WS] device=%s LLM: %q", deviceID, text)
		},

		func(pcm []int16) { // onAudio: stream 24kHz PCM → OPUS → device
			s.mu.Lock()
			enc := s.opusEnc
			s.mu.Unlock()
			if enc == nil {
				return
			}
			frames, err := enc.Encode(pcm)
			if err != nil {
				g.Log().Warningf(ctx, "[WS] device=%s encode: %v", deviceID, err)
				return
			}
			for _, frame := range frames {
				if err := s.sendAudio(frame); err != nil {
					return
				}
			}
		},

		func() { // onStart: model started generating
			g.Log().Infof(ctx, "[WS] device=%s TTS start", deviceID)
			enc, err := newOpusStreamEncoder()
			if err != nil {
				g.Log().Warningf(ctx, "[WS] device=%s encoder init: %v", deviceID, err)
				return
			}
			s.mu.Lock()
			s.opusEnc = enc
			s.mu.Unlock()
			_ = s.sendJSON(map[string]any{"type": "llm", "emotion": "neutral"})
			_ = s.sendJSON(map[string]any{"type": "tts", "state": "start"})
		},

		func() { // onStop: model finished
			g.Log().Infof(ctx, "[WS] device=%s TTS stop", deviceID)
			s.mu.Lock()
			s.opusEnc = nil
			s.mu.Unlock()
			_ = s.sendJSON(map[string]any{"type": "tts", "state": "stop"})
		},
	)
	if err != nil {
		g.Log().Errorf(ctx, "[WS] device=%s realtime connect: %v", deviceID, err)
		conn.Close()
		ha.Close()
		return
	}
	s.rt = rt
	g.Log().Infof(ctx, "[WS] device=%s realtime session ready model=%s", deviceID, rtModel)

	go s.pingLoop(ctx)
	s.run(ctx)

	g.Log().Infof(ctx, "[WS] device=%s session closed", deviceID)
	rt.Close()
	ha.Close()
}

func (s *wsSession) run(ctx context.Context) {
	defer s.conn.Close()
	for {
		msgType, data, err := s.conn.ReadMessage()
		if err != nil {
			g.Log().Infof(ctx, "[WS] device=%s read error: %v", s.deviceID, err)
			return
		}

		if msgType == websocket.BinaryMessage {
			// BinaryProtocol3: [type][reserved][payload_size_hi][payload_size_lo][payload]
			if len(data) < 4 {
				continue
			}
			payloadSize := int(binary.BigEndian.Uint16(data[2:4]))
			if payloadSize == 0 || len(data) < 4+payloadSize {
				continue
			}
			s.mu.Lock()
			listening := s.isListening
			s.mu.Unlock()
			if !listening {
				continue
			}
			// Decode the OPUS frame with the persistent decoder and stream to Realtime.
			pcm, err := decodeOpusFrame(s.opusDec, data[4:4+payloadSize])
			if err != nil {
				continue
			}
			_ = s.rt.AppendAudio(pcm)
			continue
		}

		var msg map[string]any
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		msgTypeStr, _ := msg["type"].(string)
		if state, _ := msg["state"].(string); state != "" {
			g.Log().Infof(ctx, "[WS] device=%s recv type=%s state=%s", s.deviceID, msgTypeStr, state)
		} else {
			g.Log().Infof(ctx, "[WS] device=%s recv type=%s", s.deviceID, msgTypeStr)
		}

		switch msgTypeStr {
		case "hello":
			s.handleHello(ctx)
		case "listen":
			s.handleListen(ctx, msg)
		case "abort":
			// Wake word mid-response: cancel generation, unblock device.
			_ = s.rt.CancelResponse()
			s.mu.Lock()
			s.opusEnc = nil
			s.isListening = false
			s.mu.Unlock()
			_ = s.sendJSON(map[string]any{"type": "tts", "state": "stop"})
		}
	}
}

func (s *wsSession) handleHello(ctx context.Context) {
	sessionID := uuid.New().String()
	err := s.sendJSON(map[string]any{
		"type":       "hello",
		"transport":  "websocket",
		"session_id": sessionID,
		"audio_params": map[string]any{
			"sample_rate":    serverSampleRate,
			"frame_duration": frameDurationMs,
		},
	})
	if err != nil {
		g.Log().Warningf(ctx, "[WS] device=%s hello send failed: %v", s.deviceID, err)
		return
	}
	g.Log().Infof(ctx, "[WS] device=%s session=%s hello OK", s.deviceID, sessionID)
}

func (s *wsSession) handleListen(ctx context.Context, msg map[string]any) {
	state, _ := msg["state"].(string)
	switch state {

	case "detect":
		// Wake word detected while model may be speaking: cancel and unblock device VAD.
		_ = s.rt.CancelResponse()
		s.mu.Lock()
		s.opusEnc = nil
		s.mu.Unlock()
		_ = s.sendJSON(map[string]any{"type": "tts", "state": "stop"})

	case "start":
		// New utterance: reset the OPUS decoder to clear inter-frame state.
		dec, err := newOpusDecoder()
		if err != nil {
			g.Log().Warningf(ctx, "[WS] device=%s decoder reset: %v", s.deviceID, err)
		} else {
			s.opusDec = dec
		}
		s.mu.Lock()
		s.isListening = true
		s.mu.Unlock()
		g.Log().Infof(ctx, "[WS] device=%s listening started", s.deviceID)

	case "stop":
		s.mu.Lock()
		s.isListening = false
		s.mu.Unlock()
		// Belt-and-suspenders commit in case server VAD hasn't fired yet.
		_ = s.rt.CommitAudio()
		g.Log().Infof(ctx, "[WS] device=%s listening stopped (committed)", s.deviceID)
	}
}

func (s *wsSession) sendJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteMessage(websocket.TextMessage, data)
}

func (s *wsSession) sendAudio(frame []byte) error {
	// BinaryProtocol3: [0x00][0x00][payload_size hi][payload_size lo][payload]
	header := [4]byte{0x00, 0x00, byte(len(frame) >> 8), byte(len(frame))}
	msg := append(header[:], frame...)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteMessage(websocket.BinaryMessage, msg)
}

func (s *wsSession) pingLoop(ctx context.Context) {
	ticker := time.NewTicker(50 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.writeMu.Lock()
			_ = s.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
			s.writeMu.Unlock()
		case <-ctx.Done():
			return
		}
	}
}
