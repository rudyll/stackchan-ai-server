/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

// Package ai implements a Xiaozhi WebSocket protocol v3 simulator backed by
// the OpenAI Realtime API (gpt-realtime).
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
	"sync/atomic"
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

const frameQueueSize = 600 // ~36 seconds of audio headroom

type wsSession struct {
	conn      *websocket.Conn
	deviceID  string
	sessionID string
	activity  *conversationActivity
	rt        RealtimeSession
	opusDec   *opus.Decoder // device input decoder (16kHz, reset per utterance)

	mu               sync.Mutex         // protects opusEnc and isListening
	opusEnc          *opusStreamEncoder // non-nil only while model is speaking
	isListening      bool
	playbackStarted  bool
	prebufferFrames  int
	prebufferMaxWait time.Duration
	prebufferTimer   *time.Timer

	// frameQueue carries encoded OPUS frames to pacingLoop.
	// A nil entry is a sentinel meaning "response ended — send tts:stop".
	frameQueue chan []byte

	writeMu          sync.Mutex // serialises WebSocket writes
	providerClosed   int32      // atomic: 1 when OnClose triggered conn.Close()
	listenStopped    time.Time  // latency baseline for the current user turn
	firstAudioLogged int32
}

// HandleWS upgrades the connection and runs a Xiaozhi v3 protocol session
// backed by OpenAI Realtime API.
func HandleWS(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(gctx.New())
	defer cancel()

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		g.Log().Errorf(ctx, "ws upgrade: %v", err)
		return
	}

	deviceID := r.Header.Get("Device-Id")
	provider := override(deviceProfileFor(ctx, deviceID).Provider, configuredProvider(ctx))
	var ha *haWSClient
	haEnabled, haURL, haToken := homeAssistantConnection(ctx)
	if haEnabled {
		g.Log().Infof(ctx, "[WS] device=%s connecting HA at %s", deviceID, haURL)
		ha, err = dialHAWebSocket(haURL, haToken)
		if err != nil {
			g.Log().Warningf(ctx, "[WS] device=%s HA connect failed: %v", deviceID, err)
			conn.Close()
			return
		}
		g.Log().Infof(ctx, "[WS] device=%s HA connected", deviceID)
	} else {
		g.Log().Infof(ctx, "[WS] device=%s running without Home Assistant", deviceID)
	}

	opusDec, err := newOpusDecoder()
	if err != nil {
		g.Log().Errorf(ctx, "[WS] device=%s opus decoder init: %v", deviceID, err)
		conn.Close()
		closeHAClient(ha)
		return
	}

	s := &wsSession{
		conn:             conn,
		deviceID:         deviceID,
		sessionID:        uuid.New().String(),
		activity:         newConversationActivity(time.Duration(conversationIdleSeconds(ctx))*time.Second, time.Now()),
		opusDec:          opusDec,
		frameQueue:       make(chan []byte, frameQueueSize),
		prebufferFrames:  max(0, aiInt(ctx, "audio_prebuffer_ms", 300)/frameDurationMs),
		prebufferMaxWait: time.Duration(max(0, aiInt(ctx, "audio_prebuffer_max_wait_ms", 900))) * time.Millisecond,
	}

	// Wire provider callbacks → device WebSocket writes. Same callbacks for any
	// backend (OpenAI Realtime, Gemini Live, ...) — see provider.go.
	cb := RealtimeCallbacks{
		OnClose: func() {
			// Provider session ended (error or normal). Close the device
			// connection so run() exits and the device reconnects cleanly
			// rather than waiting forever for a response that won't arrive.
			g.Log().Infof(ctx, "[WS] device=%s provider closed, dropping device connection", deviceID)
			atomic.StoreInt32(&s.providerClosed, 1)
			s.conn.Close()
		},
		OnSTT: func(text string) {
			if err := appendConversation(ctx, deviceID, s.sessionID, provider, "user", text); err != nil {
				g.Log().Warning(ctx, "[HISTORY] could not save user transcript")
			}
			s.logTurnLatency(ctx, "stt")
			_ = s.sendJSON(map[string]any{"type": "stt", "text": text})
		},

		OnText: func(text string) {
			if err := appendConversation(ctx, deviceID, s.sessionID, provider, "assistant", text); err != nil {
				g.Log().Warning(ctx, "[HISTORY] could not save assistant transcript")
			}
			s.logTurnLatency(ctx, "llm")
			g.Log().Infof(ctx, "[WS] device=%s LLM: %q", deviceID, text)
		},

		OnAudio: func(pcm []int16) { // encode and enqueue; pacingLoop sends at 60ms
			if !s.activity.responding(time.Now()) {
				return
			}
			if atomic.CompareAndSwapInt32(&s.firstAudioLogged, 0, 1) {
				s.logTurnLatency(ctx, "first_audio")
			}
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
			s.startPlayback(false)
		},

		OnStart: func() {
			if !s.activity.responding(time.Now()) {
				return
			}
			if aware, ok := s.rt.(PlaybackStateAware); ok {
				aware.SetPlaybackBusy(true)
			}
			s.logTurnLatency(ctx, "tts_start")
			g.Log().Infof(ctx, "[WS] device=%s TTS start", deviceID)
			s.drainFrameQueue() // clear any leftover frames from previous response
			enc, err := newOpusStreamEncoder()
			if err != nil {
				g.Log().Warningf(ctx, "[WS] device=%s encoder init: %v", deviceID, err)
				if aware, ok := s.rt.(PlaybackStateAware); ok {
					aware.InterruptPlayback()
				}
				return
			}
			s.mu.Lock()
			s.opusEnc = enc
			s.playbackStarted = false
			s.mu.Unlock()
			if s.prebufferMaxWait > 0 {
				s.mu.Lock()
				s.prebufferTimer = time.AfterFunc(s.prebufferMaxWait, func() { s.startPlayback(true) })
				s.mu.Unlock()
			}
			if s.prebufferFrames == 0 {
				s.startPlayback(true)
			}
		},

		OnStop: func() { // flush encoder tail, push nil sentinel; pacingLoop sends tts:stop
			g.Log().Infof(ctx, "[WS] device=%s TTS response done, draining queue", deviceID)
			s.mu.Lock()
			enc := s.opusEnc
			s.mu.Unlock()
			if enc == nil {
				s.activity.playbackDone(time.Now())
				if aware, ok := s.rt.(PlaybackStateAware); ok {
					aware.InterruptPlayback()
				}
				return
			}
			// Flush remaining PCM that didn't fill a complete 60ms frame.
			for _, frame := range enc.Flush() {
				select {
				case s.frameQueue <- frame:
				default:
				}
			}
			select {
			case s.frameQueue <- nil: // pacingLoop confirms physical playback completion
			case <-ctx.Done():
				return
			}
			s.startPlayback(true)
			s.mu.Lock()
			s.opusEnc = nil
			s.mu.Unlock()
		},
	}

	rt, err := dialProvider(ctx, deviceID, ha, cb)
	if err != nil {
		g.Log().Errorf(ctx, "[WS] device=%s provider connect: %v", deviceID, err)
		conn.Close()
		closeHAClient(ha)
		return
	}
	s.rt = rt
	defer func() {
		s.mu.Lock()
		if s.prebufferTimer != nil {
			s.prebufferTimer.Stop()
		}
		s.mu.Unlock()
	}()
	g.Log().Infof(ctx, "[WS] device=%s realtime session ready provider=%s", deviceID, provider)

	go s.pacingLoop(ctx)
	go s.pingLoop(ctx)
	go s.idleLoop(ctx)
	s.run(ctx)
	cancel()

	g.Log().Infof(ctx, "[WS] device=%s session closed", deviceID)
	rt.Close()
	closeHAClient(ha)
}

func closeHAClient(ha *haWSClient) {
	if ha != nil {
		ha.Close()
	}
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
			if !s.isPlaybackStarted() {
				continue
			}
			select {
			case frame := <-s.frameQueue:
				if frame == nil {
					// All frames delivered — tell device TTS is done.
					_ = s.sendJSON(map[string]any{"type": "tts", "state": "stop"})
					s.activity.playbackDone(time.Now())
					if aware, ok := s.rt.(PlaybackStateAware); ok {
						aware.SetPlaybackBusy(false)
					}
				} else {
					s.activity.responding(time.Now())
					_ = s.sendAudio(frame)
				}
			default:
				// Queue empty this tick — nothing to send.
			}
		}
	}
}

func (s *wsSession) isPlaybackStarted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.playbackStarted
}

// startPlayback waits for a small, configurable jitter buffer before sending
// tts:start. It prevents short upstream delivery gaps from becoming audible.
func (s *wsSession) startPlayback(force bool) {
	s.mu.Lock()
	if s.playbackStarted || s.opusEnc == nil || (!force && len(s.frameQueue) < s.prebufferFrames) {
		s.mu.Unlock()
		return
	}
	s.playbackStarted = true
	if s.prebufferTimer != nil {
		s.prebufferTimer.Stop()
		s.prebufferTimer = nil
	}
	s.mu.Unlock()
	_ = s.sendJSON(map[string]any{"type": "llm", "emotion": "neutral"})
	_ = s.sendJSON(map[string]any{"type": "tts", "state": "start"})
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
			// Suppress the "use of closed network connection" error that fires
			// when OnClose intentionally calls conn.Close() — it is expected.
			if atomic.LoadInt32(&s.providerClosed) == 0 {
				g.Log().Infof(ctx, "[WS] device=%s read error: %v", s.deviceID, err)
			}
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
			if !s.activity.audio(time.Now(), pcm) {
				return
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
			s.activity.playbackDone(time.Now())
			_ = s.rt.CancelResponse()
			s.mu.Lock()
			s.opusEnc = nil
			s.isListening = false
			s.playbackStarted = false
			if s.prebufferTimer != nil {
				s.prebufferTimer.Stop()
				s.prebufferTimer = nil
			}
			s.mu.Unlock()
			s.drainFrameQueue()
			if aware, ok := s.rt.(PlaybackStateAware); ok {
				aware.InterruptPlayback()
			}
			_ = s.sendJSON(map[string]any{"type": "tts", "state": "stop"})
		}
	}
}

func (s *wsSession) handleHello(ctx context.Context) {
	sessionID := s.sessionID
	s.activity.wake(time.Now())
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
	if aware, ok := s.rt.(AsyncDeliveryAware); ok {
		aware.SetDeliveryReady(true)
	}
	g.Log().Infof(ctx, "[WS] device=%s session=%s hello OK", s.deviceID, sessionID)
}

func (s *wsSession) handleListen(ctx context.Context, msg map[string]any) {
	state, _ := msg["state"].(string)
	switch state {

	case "detect":
		s.activity.wake(time.Now())
		// Wake word: cancel in-progress response and unblock device VAD.
		_ = s.rt.CancelResponse()
		s.mu.Lock()
		s.opusEnc = nil
		s.playbackStarted = false
		if s.prebufferTimer != nil {
			s.prebufferTimer.Stop()
			s.prebufferTimer = nil
		}
		s.mu.Unlock()
		s.drainFrameQueue()
		if aware, ok := s.rt.(PlaybackStateAware); ok {
			aware.InterruptPlayback()
		}
		_ = s.sendJSON(map[string]any{"type": "tts", "state": "stop"})

	case "start":
		if mode, _ := msg["mode"].(string); mode == "manual" {
			s.activity.wake(time.Now())
		}
		if s.activity.expired(time.Now()) {
			s.conn.Close()
			return
		}
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
		wasListening := s.isListening
		s.isListening = false
		s.mu.Unlock()
		if !wasListening {
			return
		}
		if !s.activity.commit(time.Now()) {
			return
		}
		s.mu.Lock()
		s.listenStopped = time.Now()
		s.mu.Unlock()
		atomic.StoreInt32(&s.firstAudioLogged, 0)
		_ = s.rt.CommitAudio()
		g.Log().Infof(ctx, "[WS] device=%s listening stopped (committed)", s.deviceID)
	}
}

// logTurnLatency emits cumulative timings from the device's listen:stop event.
// It is intentionally provider-neutral so logs can compare realtime and HTTP
// pipelines without exposing audio or credentials.
func (s *wsSession) logTurnLatency(ctx context.Context, stage string) {
	s.mu.Lock()
	stopped := s.listenStopped
	s.mu.Unlock()
	if stopped.IsZero() {
		return
	}
	g.Log().Infof(ctx, "[LAT] device=%s stage=%s since_listen_stop_ms=%d", s.deviceID, stage, time.Since(stopped).Milliseconds())
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
