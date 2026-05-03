/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

// Package ai implements a Xiaozhi WebSocket protocol v3 simulator that routes
// audio through OpenAI (Whisper → GPT → TTS) and device control through Home Assistant.
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
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type wsSession struct {
	conn     *websocket.Conn
	deviceID string
	opusIn   [][]byte       // accumulated OPUS frames from device
	history  []chatMessage  // conversation history
	ha       *haWSClient
	ai       *openAIClient
	mu       sync.Mutex     // protects opusIn, history, cancelAI
	writeMu  sync.Mutex     // serialises WebSocket writes
	cancelAI context.CancelFunc
}

// HandleWS upgrades the connection and runs a Xiaozhi v3 protocol session.
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
	model := cfg.MustGet(ctx, "ai.openai_model", "gpt-4o-mini").String()
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

	s := &wsSession{
		conn:     conn,
		deviceID: deviceID,
		ha:       ha,
		ai:       newOpenAIClient(apiKey, model, voice, sysPrompt),
	}

	go s.pingLoop(ctx)
	s.run(ctx)
	g.Log().Infof(ctx, "[WS] device=%s session closed", deviceID)
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
			if payloadSize > 0 && len(data) >= 4+payloadSize {
				s.mu.Lock()
				s.opusIn = append(s.opusIn, data[4:4+payloadSize])
				s.mu.Unlock()
			}
			continue
		}

		var msg map[string]any
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		msgTypeStr, _ := msg["type"].(string)
		g.Log().Infof(ctx, "[WS] device=%s recv type=%s", s.deviceID, msgTypeStr)

		switch msgTypeStr {
		case "hello":
			s.handleHello(ctx)
		case "listen":
			s.handleListen(ctx, msg)
		case "abort":
			s.cancelInFlight()
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
	case "start":
		s.mu.Lock()
		s.opusIn = nil
		s.mu.Unlock()
	case "stop":
		s.mu.Lock()
		frames := make([][]byte, len(s.opusIn))
		copy(frames, s.opusIn)
		s.opusIn = nil
		s.mu.Unlock()

		s.cancelInFlight()

		aiCtx, cancel := context.WithCancel(ctx)
		s.mu.Lock()
		s.cancelAI = cancel
		s.mu.Unlock()

		go s.runAIPipeline(aiCtx, frames)
	}
}

func (s *wsSession) cancelInFlight() {
	s.mu.Lock()
	if s.cancelAI != nil {
		s.cancelAI()
		s.cancelAI = nil
	}
	s.mu.Unlock()
}

func (s *wsSession) runAIPipeline(ctx context.Context, frames [][]byte) {
	if len(frames) == 0 {
		return
	}
	g.Log().Infof(ctx, "[AI] device=%s pipeline start frames=%d", s.deviceID, len(frames))

	// STT
	pcm, err := decodeFramesToPCM(frames)
	if err != nil || len(pcm) == 0 {
		return
	}
	wav := pcmToWAV(pcm, deviceSampleRate)
	text, err := s.ai.Transcribe(ctx, wav)
	if err != nil || text == "" {
		if ctx.Err() == nil {
			g.Log().Warningf(ctx, "STT error: %v", err)
		}
		return
	}
	_ = s.sendJSON(map[string]any{"type": "stt", "text": text})

	// LLM
	s.mu.Lock()
	s.history = append(s.history, chatMessage{Role: "user", Content: text})
	history := make([]chatMessage, len(s.history))
	copy(history, s.history)
	s.mu.Unlock()

	reply, err := s.ai.Chat(ctx, history, s.ha)
	if err != nil {
		if ctx.Err() == nil {
			g.Log().Warningf(ctx, "LLM error: %v", err)
		}
		return
	}
	if reply == "" {
		return
	}

	s.mu.Lock()
	s.history = append(s.history, chatMessage{Role: "assistant", Content: reply})
	s.mu.Unlock()

	_ = s.sendJSON(map[string]any{"type": "llm", "emotion": "neutral"})

	// TTS
	_ = s.sendJSON(map[string]any{"type": "tts", "state": "start"})
	_ = s.sendJSON(map[string]any{"type": "tts", "state": "sentence_start", "text": reply})

	tpcm, err := s.ai.Speak(ctx, reply)
	if err != nil {
		if ctx.Err() == nil {
			g.Log().Warningf(ctx, "TTS error: %v", err)
		}
		_ = s.sendJSON(map[string]any{"type": "tts", "state": "stop"})
		return
	}

	opusFrames, err := encodeOpusFrames(tpcm)
	if err != nil {
		g.Log().Warningf(ctx, "OPUS encode error: %v", err)
		_ = s.sendJSON(map[string]any{"type": "tts", "state": "stop"})
		return
	}

	for _, frame := range opusFrames {
		if ctx.Err() != nil {
			break
		}
		if err := s.sendAudio(frame); err != nil {
			break
		}
	}

	_ = s.sendJSON(map[string]any{"type": "tts", "state": "stop"})
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
	// BinaryProtocol3: [type=0x00][reserved=0x00][payload_size hi][payload_size lo][payload]
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
