/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

// The OpenAI-compatible pipeline intentionally waits for listen:stop before
// uploading audio. Most compatible endpoints expose HTTP/SSE chat APIs rather
// than the bidirectional Realtime WebSocket protocol, so pretending otherwise
// would make interruption and latency behaviour unreliable.

import (
	"context"
	"fmt"
	"sync"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
)

type compatibleConfig struct {
	BaseURL, APIKey, Model, STTModel, TTSModel, Voice, Prompt string
}

type compatibleSession struct {
	client     *openAIClient
	ha         *haWSClient
	cb         RealtimeCallbacks
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.Mutex
	pcm        []int16
	history    []chatMessage
	turnCancel context.CancelFunc
	closed     bool
}

func dialCompatibleSession(ctx context.Context, cfg compatibleConfig, ha *haWSClient, cb RealtimeCallbacks) (RealtimeSession, error) {
	if cfg.Model == "" {
		return nil, fmt.Errorf("ai.compatible_model is required for OpenAI-compatible providers")
	}
	childCtx, cancel := context.WithCancel(ctx)
	return &compatibleSession{
		client: newOpenAIClient(cfg.BaseURL, cfg.APIKey, cfg.Model, cfg.STTModel, cfg.TTSModel, cfg.Voice, cfg.Prompt),
		ha:     ha, cb: cb, ctx: childCtx, cancel: cancel,
	}, nil
}

func (s *compatibleSession) AppendAudio(pcm []int16) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return context.Canceled
	}
	s.pcm = append(s.pcm, pcm...)
	return nil
}

func (s *compatibleSession) CommitAudio() error {
	s.mu.Lock()
	if s.closed || len(s.pcm) == 0 {
		s.mu.Unlock()
		return nil
	}
	pcm := append([]int16(nil), s.pcm...)
	s.pcm = s.pcm[:0]
	if s.turnCancel != nil {
		s.turnCancel()
	}
	turnCtx, turnCancel := context.WithCancel(s.ctx)
	s.turnCancel = turnCancel
	s.mu.Unlock()
	go s.completeTurn(turnCtx, pcm)
	return nil
}

func (s *compatibleSession) completeTurn(ctx context.Context, pcm []int16) {
	text, err := s.client.Transcribe(ctx, pcmToWAV(pcm, 16000))
	if err != nil {
		g.Log().Warningf(gctx.New(), "[COMPAT] transcription: %v", err)
		return
	}
	if text == "" {
		return
	}
	if s.cb.OnSTT != nil {
		s.cb.OnSTT(text)
	}

	s.mu.Lock()
	history := append([]chatMessage(nil), s.history...)
	history = append(history, chatMessage{Role: "user", Content: text})
	s.mu.Unlock()
	reply, err := s.client.Chat(ctx, history, s.ha)
	if err != nil {
		g.Log().Warningf(gctx.New(), "[COMPAT] chat: %v", err)
		return
	}
	if reply == "" {
		return
	}
	if s.cb.OnText != nil {
		s.cb.OnText(reply)
	}
	if s.cb.OnStart != nil {
		s.cb.OnStart()
	}
	pcmReply, err := s.client.Speak(ctx, reply)
	if err != nil {
		g.Log().Warningf(gctx.New(), "[COMPAT] TTS: %v", err)
	} else if len(pcmReply) > 0 && s.cb.OnAudio != nil {
		s.cb.OnAudio(pcmReply)
	}
	if s.cb.OnStop != nil {
		s.cb.OnStop()
	}

	s.mu.Lock()
	s.history = append(history, chatMessage{Role: "assistant", Content: reply})
	s.mu.Unlock()
}

func (s *compatibleSession) CancelResponse() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.turnCancel != nil {
		s.turnCancel()
		s.turnCancel = nil
	}
	return nil
}

func (s *compatibleSession) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	if s.turnCancel != nil {
		s.turnCancel()
	}
	s.mu.Unlock()
	s.cancel()
}
