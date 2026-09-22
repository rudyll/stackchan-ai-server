/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type stubRealtimeSession struct {
	commits  int32
	cancels  int32
	needsVAD bool
}

func (s *stubRealtimeSession) AppendAudio([]int16) error { return nil }
func (s *stubRealtimeSession) CommitAudio() error        { atomic.AddInt32(&s.commits, 1); return nil }
func (s *stubRealtimeSession) CancelResponse() error     { atomic.AddInt32(&s.cancels, 1); return nil }
func (s *stubRealtimeSession) Close()                    {}
func (s *stubRealtimeSession) NeedsServerVAD() bool      { return s.needsVAD }

func sessionForTurnTest(t *testing.T, idle time.Duration, needsVAD bool) (*wsSession, *stubRealtimeSession) {
	t.Helper()
	rt := &stubRealtimeSession{needsVAD: needsVAD}
	s := &wsSession{
		activity:    newConversationActivity(idle, time.Now()),
		rt:          rt,
		isListening: true,
	}
	if needsVAD {
		s.vad = newServerVAD(600, serverVADMinSpeechMs)
	}
	return s, rt
}

// Whichever end-of-turn signal arrives first must commit exactly once: the
// server detector and a late device listen:stop must not start two turns.
func TestCommitTurnIsIdempotentAcrossTriggers(t *testing.T) {
	s, rt := sessionForTurnTest(t, 0, true)
	ctx := context.Background()

	s.commitTurn(ctx, "server_vad")
	s.commitTurn(ctx, "listen_stop")

	if got := atomic.LoadInt32(&rt.commits); got != 1 {
		t.Fatalf("CommitAudio called %d times, want 1", got)
	}
	s.mu.Lock()
	listening := s.isListening
	s.mu.Unlock()
	if listening {
		t.Fatal("session still listening after the turn was committed")
	}
}

// A turn is committed only after the microphone actually carried speech, so an
// idle device streaming silence never triggers an empty transcription request.
func TestCommitTurnSkipsSilentUtterance(t *testing.T) {
	s, rt := sessionForTurnTest(t, 15*time.Second, true)
	s.commitTurn(context.Background(), "server_vad")
	if got := atomic.LoadInt32(&rt.commits); got != 0 {
		t.Fatalf("CommitAudio called %d times for silence, want 0", got)
	}

	s.mu.Lock()
	s.isListening = true
	s.mu.Unlock()
	if !s.activity.audio(time.Now(), speechFrame()) {
		t.Fatal("activity closed the channel on a speech frame")
	}
	s.commitTurn(context.Background(), "server_vad")
	if got := atomic.LoadInt32(&rt.commits); got != 1 {
		t.Fatalf("CommitAudio called %d times after real speech, want 1", got)
	}
}

func TestRearmListeningOnlyForServerVADProviders(t *testing.T) {
	// Turn-based devices in auto mode may not announce listen:start again, so
	// the server resumes listening itself once playback has finished.
	withVAD, _ := sessionForTurnTest(t, 0, true)
	withVAD.commitTurn(context.Background(), "server_vad")
	withVAD.rearmListening(context.Background())
	withVAD.mu.Lock()
	resumed := withVAD.isListening
	withVAD.mu.Unlock()
	if !resumed {
		t.Fatal("server-VAD session did not resume listening after playback")
	}

	// Devices that drive their own turns keep waiting for the next listen:start.
	deviceDriven, _ := sessionForTurnTest(t, 0, false)
	deviceDriven.commitTurn(context.Background(), "listen_stop")
	deviceDriven.rearmListening(context.Background())
	deviceDriven.mu.Lock()
	resumed = deviceDriven.isListening
	deviceDriven.mu.Unlock()
	if resumed {
		t.Fatal("device-driven session resumed listening on its own")
	}
}

func TestRearmListeningStopsAfterIdleTimeout(t *testing.T) {
	// An expired conversation must return the device to wake-word standby
	// instead of being re-armed behind the idle loop's back.
	s, _ := sessionForTurnTest(t, time.Millisecond, true)
	if !s.activity.audio(time.Now(), speechFrame()) {
		t.Fatal("activity closed the channel on a speech frame")
	}
	s.commitTurn(context.Background(), "server_vad")
	// Playback restarts the idle countdown; the deadline then passes.
	s.activity.playbackDone(time.Now())
	time.Sleep(10 * time.Millisecond)
	if !s.activity.expired(time.Now()) {
		t.Fatal("idle deadline did not elapse")
	}
	s.rearmListening(context.Background())
	s.mu.Lock()
	listening := s.isListening
	s.mu.Unlock()
	if listening {
		t.Fatal("expired conversation resumed listening")
	}
}
