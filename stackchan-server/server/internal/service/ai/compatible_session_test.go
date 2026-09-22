/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// compatibleSessionForTest wires the three pipeline stages to one fake transport
// so no test needs a listening socket.
func compatibleSessionForTest(t *testing.T, cb RealtimeCallbacks, respond func(path string) (*http.Response, error)) *compatibleSession {
	t.Helper()
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return respond(request.URL.Path)
	})
	newStage := func(client *openAIClient) *openAIClient {
		client.http = &http.Client{Transport: transport}
		return client
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &compatibleSession{
		sttClient: newStage(newOpenAIClient("https://stage.test/v1", "key", "", "whisper-1", "", "", "")),
		llmClient: newStage(newOpenAIClient("https://stage.test/v1", "key", "chat-model", "", "", "", "prompt")),
		ttsClient: newStage(newOpenAIClient("https://stage.test/v1", "key", "", "", "tts-1", "voice", "")),
		cb:        cb,
		ctx:       ctx,
		cancel:    cancel,
	}
}

func pcmResponse(samples int) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(strings.Repeat("\x01\x02", samples))),
	}
}

// A turn that produces no audio still has to report its end, otherwise the
// protocol layer waits for a playback completion that never arrives and the
// device stays deaf for the rest of the session.
func TestCompatibleSessionReportsEndOfTurnWithoutAudio(t *testing.T) {
	tests := []struct {
		name    string
		respond func(path string) (*http.Response, error)
	}{
		{
			name: "empty transcript",
			respond: func(path string) (*http.Response, error) {
				return jsonResponse(`{"text":""}`), nil
			},
		},
		{
			name: "transcription error",
			respond: func(path string) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusBadGateway, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("upstream down"))}, nil
			},
		},
		{
			name: "empty model reply",
			respond: func(path string) (*http.Response, error) {
				if strings.HasSuffix(path, "/audio/transcriptions") {
					return jsonResponse(`{"text":"你好"}`), nil
				}
				return jsonResponse(`{"choices":[{"message":{"content":""}}]}`), nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var mu sync.Mutex
			starts, stops := 0, 0
			cb := RealtimeCallbacks{
				OnStart: func() { mu.Lock(); starts++; mu.Unlock() },
				OnStop:  func() { mu.Lock(); stops++; mu.Unlock() },
			}
			session := compatibleSessionForTest(t, cb, test.respond)
			session.completeTurn(session.ctx, make([]int16, deviceFrameSamples))

			mu.Lock()
			defer mu.Unlock()
			if starts != 0 {
				t.Fatalf("OnStart called %d times for a turn without audio, want 0", starts)
			}
			if stops != 1 {
				t.Fatalf("OnStop called %d times, want exactly 1", stops)
			}
		})
	}
}

func TestCompatibleSessionCompletesFullTurnOnce(t *testing.T) {
	var mu sync.Mutex
	var events []string
	cb := RealtimeCallbacks{
		OnSTT:   func(string) { mu.Lock(); events = append(events, "stt"); mu.Unlock() },
		OnText:  func(string) { mu.Lock(); events = append(events, "text"); mu.Unlock() },
		OnStart: func() { mu.Lock(); events = append(events, "start"); mu.Unlock() },
		OnAudio: func([]int16) { mu.Lock(); events = append(events, "audio"); mu.Unlock() },
		OnStop:  func() { mu.Lock(); events = append(events, "stop"); mu.Unlock() },
	}
	session := compatibleSessionForTest(t, cb, func(path string) (*http.Response, error) {
		switch {
		case strings.HasSuffix(path, "/audio/transcriptions"):
			return jsonResponse(`{"text":"开灯"}`), nil
		case strings.HasSuffix(path, "/chat/completions"):
			return jsonResponse(`{"choices":[{"message":{"content":"好的"}}]}`), nil
		case strings.HasSuffix(path, "/audio/speech"):
			return pcmResponse(480), nil
		}
		return nil, fmt.Errorf("unexpected path %s", path)
	})
	session.completeTurn(session.ctx, make([]int16, deviceFrameSamples))

	mu.Lock()
	defer mu.Unlock()
	want := "stt,text,start,audio,stop"
	if got := strings.Join(events, ","); got != want {
		t.Fatalf("callback order = %q, want %q", got, want)
	}
}

func TestCompatibleSessionNeedsServerVAD(t *testing.T) {
	// The protocol layer keys its own end-of-turn detection off this interface.
	var session RealtimeSession = &compatibleSession{}
	aware, ok := session.(ServerVADAware)
	if !ok {
		t.Fatal("compatibleSession does not implement ServerVADAware")
	}
	if !aware.NeedsServerVAD() {
		t.Fatal("turn-based pipeline reported that it has its own VAD")
	}
}
