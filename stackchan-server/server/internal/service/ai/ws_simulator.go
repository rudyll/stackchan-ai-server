/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

// Package ai implements a Xiaozhi WebSocket protocol v3 simulator backed by
// the OpenAI Realtime API (gpt-realtime-1.5).
//
// Audio path:
//
//	device OPUS (16kHz) → PCM → Realtime WS input buffer
//	                              ↓  server VAD detects speech end
//	                         Realtime WS output (PCM 24kHz, streaming)
//	                              ↓
//	                    opusStreamEncoder → frameQueue (chan)
//	                              ↓
//	                    pacingLoop (60ms ticker) → device OPUS frames
//
// pacingLoop paces frame delivery at exactly one frame per 60ms, preventing
// burst delivery that causes audio stuttering on the device.
// A nil sentinel in frameQueue signals the end of a response; pacingLoop
// sends tts:stop only after all queued frames have been delivered.
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

const frameQueueSize = 200 // ~12 seconds of audio headroom

type wsSession struct {
	conn     *websocket.Conn
	deviceID string
	rt       *realtimeSession
	opusDec  *opus.Decoder // device input decoder (16kHz, reset per utterance)

	mu          sync.Mutex         // protects opusEnc and isListening
	opusEnc     *opusStreamEncoder // non-nil only while model is speaking
	isListening bool

	// frameQueue carries encoded OPUS frames to pacingLoop.
	// A nil entry is a sentinel meaning "response ended — send tts:stop".
	frameQueue chan []byte

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
		conn:       conn,
		deviceID:   deviceID,
		opusDec:    opusDec,
		frameQueue: make(chan []byte, frameQueueSize),
	}

	// Wire Realtime callbacks → device WebSocket writes.
	rt, err := dialRealtimeSession(ctx, apiKey, rtModel, voice, sysPrompt, ha,

		func(text string) { // onSTT
			_ = s.sendJSON(map[string]any{"type": "stt", "text": text})
		},

		func(text string) { // onText
			g.Log().Infof(ctx, "[WS] device=%s LLM: %q", deviceID, text)
		},

		func(pcm []int16) { // onAudio — encode and enqueue; pacingLoop sends at 60ms
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
				select {
				case s.frameQueue <- frame:
				default:
					g.Log().Warningf(ctx, "[WS] device=%s frame queue full, dropping frame", deviceID)
				}
			}
		},

		func() { // onStart — model began generating
			g.Log().Infof(ctx, "[WS] device=%s TTS start", deviceID)
			s.drainFrameQueue() // clear any leftover frames from previous response
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

		func() { // onStop — push nil sentinel; pacingLoop sends tts:stop after queue drains
			g.Log().Infof(ctx, "[WS] device=%s TTS response done, draining queue", deviceID)
			s.mu.Lock()
			s.opusEnc = nil
			s.mu.Unlock()
			select {
			case s.frameQueue <- nil: // sentinel
			default:
			}
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

	go s.pacingLoop(ctx)
	go s.pingLoop(ctx)
	s.run(ctx)

	g.Log().Infof(ctx, "[WS] device=%s session closed", deviceID)
	rt.Close()
	ha.Close()
}

// pacingLoop delivers OPUS frames to the device at a steady 60ms per frame.
// A nil frame is a sentinel: send tts:stop and resume idle.
func (s *wsSession) pacingLoop(ctx context.Context) {
	ticker := time.NewTicker(frameDurationMs * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			select {
			case frame := <-s.frameQueue:
				if frame == nil {
					// All frames delivered — tell device TTS is done.
					_ = s.sendJSON(map[string]any{"type": "tts", "state": "stop"})
				} else {
					_ = s.sendAudio(frame)
				}
			default:
				// Queue empty this tick — nothing to send.
			}
		}
	}
}

// drainFrameQueue discards all pending frames (called on abort or new response start).
func (s *wsSession) drainFrameQueue() {
	for {
		select {
		case <-s.frameQueue:
		default:
			return
		}
	}
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
			_ = s.rt.CancelResponse()
			s.mu.Lock()
			s.opusEnc = nil
			s.isListening = false
			s.mu.Unlock()
			s.drainFrameQueue()
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
		// Wake word: cancel in-progress response and unblock device VAD.
		_ = s.rt.CancelResponse()
		s.mu.Lock()
		s.opusEnc = nil
		s.mu.Unlock()
		s.drainFrameQueue()
		_ = s.sendJSON(map[string]any{"type": "tts", "state": "stop"})

	case "start":
		// New utterance: reset the OPUS decoder for a clean stream.
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
