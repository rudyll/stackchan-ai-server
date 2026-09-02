package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
	"github.com/gorilla/websocket"
)

func TestConversationSilenceDoesNotExtendFollowUpWindow(t *testing.T) {
	now := time.Now()
	a := newConversationActivity(15*time.Second, now)
	for seconds := 1; seconds < 15; seconds++ {
		if !a.audio(now.Add(time.Duration(seconds)*time.Second), []int16{0, 1, -1}) {
			t.Fatal("window expired too soon")
		}
	}
	if a.commit(now.Add(14 * time.Second)) {
		t.Fatal("silence-only listen:stop extended the window")
	}
	if a.audio(now.Add(15*time.Second), []int16{3000}) {
		t.Fatal("speech after expiry reopened the microphone")
	}
	if a.responding(now.Add(16 * time.Second)) {
		t.Fatal("late provider response reopened expired conversation")
	}
}

func TestConversationAllowsFollowUpAndWaitsForPlayback(t *testing.T) {
	now := time.Now()
	a := newConversationActivity(15*time.Second, now)
	if !a.audio(now.Add(14*time.Second), []int16{1000, -1000}) {
		t.Fatal("follow-up rejected")
	}
	if a.expired(now.Add(20 * time.Second)) {
		t.Fatal("active utterance was cut off")
	}
	if !a.commit(now.Add(21 * time.Second)) {
		t.Fatal("speech was not committed")
	}
	if !a.responding(now.Add(30 * time.Second)) {
		t.Fatal("provider response rejected")
	}
	if a.expired(now.Add(50 * time.Second)) {
		t.Fatal("expired while assistant was speaking")
	}
	a.playbackDone(now.Add(60 * time.Second))
	if a.expired(now.Add(74*time.Second)) || !a.expired(now.Add(75*time.Second)) {
		t.Fatal("follow-up window did not start at playback completion")
	}
}

func TestConversationAutoListenDoesNotRearmTimeout(t *testing.T) {
	now := time.Now()
	a := newConversationActivity(15*time.Second, now)
	s := &wsSession{activity: a}
	s.handleListen(context.Background(), map[string]any{"state": "start", "mode": "auto"})
	if !a.deadline.Equal(now.Add(15 * time.Second)) {
		t.Fatal("automatic listen:start reset the deadline")
	}
}

func TestConversationWakeAndDisabledTimeout(t *testing.T) {
	now := time.Now()
	a := newConversationActivity(15*time.Second, now)
	a.wake(now.Add(10 * time.Second))
	if a.expired(now.Add(20 * time.Second)) {
		t.Fatal("wake word did not refresh window")
	}
	disabled := newConversationActivity(0, now)
	if disabled.expired(now.Add(24*time.Hour)) || !disabled.audio(now.Add(25*time.Hour), nil) {
		t.Fatal("legacy unlimited mode expired")
	}
	waiting := newConversationActivity(15*time.Second, now)
	waiting.responding(now)
	if !waiting.expired(now.Add(2 * time.Minute)) {
		t.Fatal("stalled provider held the connection indefinitely")
	}
}

func TestConversationRuntimeDefaults(t *testing.T) {
	t.Setenv("STACKCHAN_DATA_DIR", t.TempDir())
	previous := g.Cfg().GetAdapter()
	t.Cleanup(func() { g.Cfg().SetAdapter(previous) })
	for _, test := range []struct {
		config string
		want   int
	}{
		{`{"ai":{"ha_enabled":false}}`, 15},
		{`{"ai":{"ha_enabled":false,"standalone_ha_enabled":true}}`, 15},
		{`{"ai":{"ha_enabled":true}}`, 0},
	} {
		adapter, err := gcfg.NewAdapterContent(test.config)
		if err != nil {
			t.Fatal(err)
		}
		g.Cfg().SetAdapter(adapter)
		if got := conversationIdleSeconds(context.Background()); got != test.want {
			t.Fatalf("timeout=%d, want %d", got, test.want)
		}
	}
}

type idleTestProvider struct{}

func (idleTestProvider) AppendAudio([]int16) error { return nil }
func (idleTestProvider) CommitAudio() error        { return nil }
func (idleTestProvider) CancelResponse() error     { return nil }
func (idleTestProvider) Close()                    {}

func TestConversationIdleClosesDeviceAudioChannel(t *testing.T) {
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		s := &wsSession{conn: conn, rt: idleTestProvider{}, activity: newConversationActivity(200*time.Millisecond, time.Now()), sessionID: "test-session", frameQueue: make(chan []byte, 2)}
		go s.idleLoop(ctx)
		s.run(ctx)
	}))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.WriteJSON(map[string]any{"type": "hello"}); err != nil {
		t.Fatal(err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var hello map[string]any
	if err := conn.ReadJSON(&hello); err != nil || hello["type"] != "hello" {
		t.Fatalf("hello: %v %v", hello, err)
	}
	if err := conn.WriteJSON(map[string]any{"type": "listen", "state": "start", "mode": "auto"}); err != nil {
		t.Fatal(err)
	}
	_, _, err = conn.ReadMessage()
	if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
		t.Fatalf("wanted normal idle close, got %v", err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("device handler did not stop after idle close")
	}
}
