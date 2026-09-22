/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

// Server-side end-of-turn detection for providers without their own VAD.
//
// Xiaozhi firmware that listens in `auto` (or `realtime`) mode leaves the
// end-of-turn decision to the server and never sends `listen:stop`. The
// turn-based STT → Chat Completions → TTS pipeline used to wait for that
// message, so a device in auto mode and the server waited for each other and no
// turn ever started. serverVAD closes that gap: it watches decoded microphone
// frames and reports the end of an utterance after a configurable silence.
//
// Realtime providers (OpenAI Realtime, Gemini Live) keep using the VAD on their
// own side, so no detector is created for them.
package ai

import "sync"

const (
	// serverVADSilenceMsDefault is the trailing silence that ends a turn.
	serverVADSilenceMsDefault = 800
	// serverVADMinSpeechMs keeps a door slam or a single noise burst from
	// triggering an empty transcription request.
	serverVADMinSpeechMs = 300
)

type serverVAD struct {
	mu          sync.Mutex
	silenceMs   int
	minSpeechMs int
	speechMs    int
	quietMs     int
	armed       bool // heard enough speech for a silence to mean "turn over"
}

func newServerVAD(silenceMs, minSpeechMs int) *serverVAD {
	if silenceMs <= 0 {
		silenceMs = serverVADSilenceMsDefault
	}
	if minSpeechMs < 0 {
		minSpeechMs = 0
	}
	return &serverVAD{silenceMs: silenceMs, minSpeechMs: minSpeechMs}
}

// observe feeds one decoded 16kHz microphone frame and reports whether the
// utterance just ended. Frame durations are derived from the sample count rather
// than the clock, so network jitter cannot cut a turn short. A nil detector
// never fires, which keeps the caller free of provider checks.
func (v *serverVAD) observe(pcm []int16) bool {
	if v == nil || len(pcm) == 0 {
		return false
	}
	durationMs := len(pcm) * 1000 / deviceSampleRate
	v.mu.Lock()
	defer v.mu.Unlock()
	if frameIsSpeech(pcm) {
		v.speechMs += durationMs
		v.quietMs = 0
		if v.speechMs >= v.minSpeechMs {
			v.armed = true
		}
		return false
	}
	v.quietMs += durationMs
	if v.quietMs < v.silenceMs {
		// Short pauses inside a sentence must not end the turn.
		return false
	}
	ended := v.armed
	v.resetLocked()
	return ended
}

// reset discards the current utterance. Callers use it when a turn is committed
// by other means (device `listen:stop`, wake word, abort) so leftover state
// cannot end the next turn early.
func (v *serverVAD) reset() {
	if v == nil {
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	v.resetLocked()
}

func (v *serverVAD) resetLocked() {
	v.speechMs, v.quietMs, v.armed = 0, 0, false
}
