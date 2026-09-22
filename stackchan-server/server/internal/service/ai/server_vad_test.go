/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import "testing"

// frames of 60ms at 16kHz, the size the device actually sends.
func speechFrame() []int16 {
	pcm := make([]int16, deviceFrameSamples)
	for i := range pcm {
		if i%2 == 0 {
			pcm[i] = 6000
		} else {
			pcm[i] = -6000
		}
	}
	return pcm
}

func silentFrame() []int16 {
	return make([]int16, deviceFrameSamples)
}

func feed(t *testing.T, vad *serverVAD, frame []int16, count int) int {
	t.Helper()
	ended := 0
	for i := 0; i < count; i++ {
		if vad.observe(frame) {
			ended++
		}
	}
	return ended
}

func TestServerVADEndsTurnAfterTrailingSilence(t *testing.T) {
	vad := newServerVAD(600, serverVADMinSpeechMs)
	if ended := feed(t, vad, speechFrame(), 10); ended != 0 {
		t.Fatalf("speech ended the turn %d times, want 0", ended)
	}
	// 600ms of silence = 10 frames; the turn must end exactly once.
	if ended := feed(t, vad, silentFrame(), 10); ended != 1 {
		t.Fatalf("turn ended %d times after trailing silence, want 1", ended)
	}
	if ended := feed(t, vad, silentFrame(), 30); ended != 0 {
		t.Fatalf("continued silence ended the turn %d more times, want 0", ended)
	}
}

func TestServerVADIgnoresSilenceWithoutSpeech(t *testing.T) {
	// A device that streams an idle microphone must never trigger a turn: the
	// transcription request would be empty and the reply meaningless.
	vad := newServerVAD(600, serverVADMinSpeechMs)
	if ended := feed(t, vad, silentFrame(), 100); ended != 0 {
		t.Fatalf("idle microphone ended the turn %d times, want 0", ended)
	}
}

func TestServerVADIgnoresShortNoiseBurst(t *testing.T) {
	// One 60ms burst is below serverVADMinSpeechMs (300ms).
	vad := newServerVAD(600, serverVADMinSpeechMs)
	if ended := feed(t, vad, speechFrame(), 1); ended != 0 {
		t.Fatalf("burst ended the turn during speech, want 0")
	}
	if ended := feed(t, vad, silentFrame(), 40); ended != 0 {
		t.Fatalf("short burst plus silence ended the turn %d times, want 0", ended)
	}
}

func TestServerVADKeepsTurnOpenAcrossShortPause(t *testing.T) {
	vad := newServerVAD(800, serverVADMinSpeechMs)
	feed(t, vad, speechFrame(), 8)
	// ~360ms pause inside a sentence stays below the 800ms threshold.
	if ended := feed(t, vad, silentFrame(), 6); ended != 0 {
		t.Fatalf("mid-sentence pause ended the turn %d times, want 0", ended)
	}
	feed(t, vad, speechFrame(), 5)
	if ended := feed(t, vad, silentFrame(), 6); ended != 0 {
		t.Fatalf("pause after resumed speech ended the turn too early")
	}
	if ended := feed(t, vad, silentFrame(), 8); ended != 1 {
		t.Fatalf("turn ended %d times after the full silence window, want 1", ended)
	}
}

func TestServerVADResetDropsPendingUtterance(t *testing.T) {
	// listen:stop, wake word and abort all reset the detector; leftover speech
	// must not make the next turn end on its very first silent frame.
	vad := newServerVAD(600, serverVADMinSpeechMs)
	feed(t, vad, speechFrame(), 10)
	vad.reset()
	if ended := feed(t, vad, silentFrame(), 20); ended != 0 {
		t.Fatalf("turn ended %d times after reset, want 0", ended)
	}
}

func TestServerVADNilNeverFires(t *testing.T) {
	// Realtime providers leave vad nil; the read loop calls it unconditionally.
	var vad *serverVAD
	if vad.observe(speechFrame()) {
		t.Fatal("nil detector reported an end of turn")
	}
	vad.reset()
}

func TestServerVADDefaultsSilenceWindow(t *testing.T) {
	if vad := newServerVAD(0, serverVADMinSpeechMs); vad.silenceMs != serverVADSilenceMsDefault {
		t.Fatalf("silenceMs = %d, want %d", vad.silenceMs, serverVADSilenceMsDefault)
	}
}

func TestFrameIsSpeechSeparatesSilenceFromSpeech(t *testing.T) {
	if frameIsSpeech(silentFrame()) {
		t.Fatal("silent frame counted as speech")
	}
	if !frameIsSpeech(speechFrame()) {
		t.Fatal("loud frame did not count as speech")
	}
	if frameIsSpeech(nil) {
		t.Fatal("empty frame counted as speech")
	}
}
