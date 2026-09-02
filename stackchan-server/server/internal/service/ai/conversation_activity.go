package ai

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gorilla/websocket"
)

// Silence packets and automatic listen:start messages must not keep a device
// active forever. Closing the audio channel returns stock firmware to local
// wake-word/button standby. This is not a server-side keyword recognizer.
type conversationActivity struct {
	mu          sync.Mutex
	timeout     time.Duration
	deadline    time.Time
	busy        bool
	heardSpeech bool
	closed      bool
}

func conversationIdleSeconds(ctx context.Context) int {
	fallback := 15
	if aiBool(ctx, "ha_enabled", true) {
		fallback = 0
	}
	return min(300, max(0, aiInt(ctx, "conversation_idle_seconds", fallback)))
}

func newConversationActivity(timeout time.Duration, now time.Time) *conversationActivity {
	return &conversationActivity{timeout: timeout, deadline: now.Add(timeout)}
}

func (a *conversationActivity) expiredLocked(now time.Time) bool {
	if a.timeout > 0 && !now.Before(a.deadline) {
		a.closed = true
	}
	return a.closed
}

func (a *conversationActivity) expired(now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.expiredLocked(now)
}

func (a *conversationActivity) wake(now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.closed {
		a.busy = false
		a.heardSpeech = false
		a.deadline = now.Add(a.timeout)
	}
}

func (a *conversationActivity) audio(now time.Time, pcm []int16) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.expiredLocked(now) {
		return false
	}
	// A small energy floor distinguishes microphone silence from an utterance.
	// Ambient speech/noise inside the follow-up window can still count as speech.
	var energy int64
	for _, sample := range pcm {
		energy += int64(sample) * int64(sample)
	}
	if len(pcm) > 0 && energy/int64(len(pcm)) > 400*400 {
		a.heardSpeech = true
		if !a.busy {
			a.deadline = now.Add(a.timeout)
		}
	}
	return true
}

func (a *conversationActivity) commit(now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.expiredLocked(now) {
		return false
	}
	if a.timeout > 0 && !a.heardSpeech {
		return false
	}
	a.heardSpeech = false
	a.busy = true
	a.deadline = now.Add(2 * time.Minute)
	return true
}

func (a *conversationActivity) responding(now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.expiredLocked(now) {
		return false
	}
	a.busy = true
	// Allow slow provider/tool responses, but never leave a failed response
	// holding the audio channel open indefinitely.
	a.deadline = now.Add(2 * time.Minute)
	return true
}

func (a *conversationActivity) playbackDone(now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.closed {
		a.busy = false
		a.deadline = now.Add(a.timeout)
	}
}

func (s *wsSession) idleLoop(ctx context.Context) {
	if s.activity.timeout == 0 {
		return
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.activity.expired(time.Now()) {
				g.Log().Info(ctx, "[WS] conversation idle; returning device to wake-word standby")
				atomic.StoreInt32(&s.providerClosed, 1)
				_ = s.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "conversation idle"), time.Now().Add(time.Second))
				_ = s.conn.Close()
				return
			}
		}
	}
}
