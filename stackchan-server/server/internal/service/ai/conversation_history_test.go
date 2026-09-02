package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func enableHistoryForTest(t *testing.T) context.Context {
	t.Helper()
	t.Setenv("STACKCHAN_DATA_DIR", t.TempDir())
	if err := writeSettings(map[string]string{"conversation_history_enabled": "true", "conversation_history_days": "90", "conversation_context_messages": "20"}); err != nil {
		t.Fatal(err)
	}
	return context.Background()
}

func TestConversationHistorySurvivesReconnectAndIsolatesDevices(t *testing.T) {
	ctx := enableHistoryForTest(t)
	if err := appendConversation(ctx, "device-one", "first-session", "openai", "user", "我的猫叫团团"); err != nil {
		t.Fatal(err)
	}
	if err := appendConversation(ctx, "device-one", "second-session", "gemini", "assistant", "团团今天怎么样？"); err != nil {
		t.Fatal(err)
	}
	memory, err := conversationContext(ctx, "device-one")
	if err != nil || !strings.Contains(memory, "团团") || !strings.Contains(memory, "first-session") {
		t.Fatalf("context not restored: %v %q", err, memory)
	}
	other, err := conversationContext(ctx, "device-two")
	if err != nil || other != "" {
		t.Fatalf("another device inherited history: %v %q", err, other)
	}
	path, _ := conversationHistoryPath("device-one")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("history permissions = %o", info.Mode().Perm())
	}
	if strings.Contains(filepath.Base(path), "device-one") {
		t.Fatal("raw Device-Id in filename")
	}
	if _, err := conversationHistoryPath(""); err == nil {
		t.Fatal("anonymous devices must not share history")
	}
}

func TestConversationHistoryDisabledAndArchiveOnly(t *testing.T) {
	ctx := enableHistoryForTest(t)
	if err := writeSettings(map[string]string{"conversation_history_enabled": "false"}); err != nil {
		t.Fatal(err)
	}
	if err := appendConversation(ctx, "device-one", "session", "openai", "user", "not saved"); err != nil {
		t.Fatal(err)
	}
	path, _ := conversationHistoryPath("device-one")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("disabled history created a file")
	}
	if err := writeSettings(map[string]string{"conversation_history_enabled": "true", "conversation_context_messages": "0"}); err != nil {
		t.Fatal(err)
	}
	if err := appendConversation(ctx, "device-one", "session", "openai", "user", "archive only"); err != nil {
		t.Fatal(err)
	}
	if memory, err := conversationContext(ctx, "device-one"); err != nil || memory != "" {
		t.Fatal("archive-only history was injected")
	}
	messages, err := loadConversation(ctx, "device-one")
	if err != nil || len(messages) != 1 {
		t.Fatal("archive-only record was not saved")
	}
}

func TestConversationHistoryPrunesExpiredAndLimitsContext(t *testing.T) {
	ctx := enableHistoryForTest(t)
	path, _ := conversationHistoryPath("device-one")
	now := time.Now()
	messages := []conversationMessage{
		{Time: now.Add(-91 * 24 * time.Hour), Role: "user", Text: "expired"},
		{Time: now, Role: "system", Text: "not conversation"},
	}
	for i := 0; i < 30; i++ {
		messages = append(messages, conversationMessage{Time: now, Role: "user", Text: strings.Repeat("猫", 5000)})
	}
	if err := writeConversationFile(path, messages); err != nil {
		t.Fatal(err)
	}
	got, err := loadConversation(ctx, "device-one")
	if err != nil || len(got) != 30 {
		t.Fatalf("retention: %d %v", len(got), err)
	}
	stored, err := readConversationFile(path)
	if err != nil || len(stored) != 30 {
		t.Fatal("expired records not pruned from disk")
	}
	memory, err := conversationContext(ctx, "device-one")
	if err != nil || strings.Count(memory, "猫") > historyContextRunes || strings.Contains(memory, "expired") {
		t.Fatalf("unbounded or expired context: %v", err)
	}
	if len([]rune(got[0].Text)) != historyMaxTextRunes {
		t.Fatal("message size not limited")
	}
}

func TestConversationHistoryConcurrentAppends(t *testing.T) {
	ctx := enableHistoryForTest(t)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := appendConversation(ctx, "device-one", "session", "openai", "user", "message"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	messages, err := loadConversation(ctx, "device-one")
	if err != nil || len(messages) != 12 {
		t.Fatalf("lost concurrent writes: %d %v", len(messages), err)
	}
}

func TestConversationHistoryCleanupIncludesInactiveDevices(t *testing.T) {
	ctx := enableHistoryForTest(t)
	for _, device := range []string{"inactive-one", "inactive-two"} {
		path, _ := conversationHistoryPath(device)
		if err := writeConversationFile(path, []conversationMessage{{Time: time.Now().Add(-100 * 24 * time.Hour), Role: "user", Text: "expired"}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := pruneConversationHistory(ctx); err != nil {
		t.Fatal(err)
	}
	for _, device := range []string{"inactive-one", "inactive-two"} {
		path, _ := conversationHistoryPath(device)
		messages, err := readConversationFile(path)
		if err != nil || len(messages) != 0 {
			t.Fatal("inactive device retained expired history")
		}
	}
}

func TestConversationHistoryAPIAuthExportAndClear(t *testing.T) {
	ctx := enableHistoryForTest(t)
	if err := appendConversation(ctx, "device-one", "session", "openai", "user", "hello <script>"); err != nil {
		t.Fatal(err)
	}
	handler := configUIAuth(configUIHandler(), "test-token", true)
	url := "/api/conversation-history?device_id=device-one&format=md"
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(method, url, nil))
		if response.Code != http.StatusUnauthorized {
			t.Fatal("history API did not require authentication")
		}
	}
	request := httptest.NewRequest(http.MethodGet, url, nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "    hello <script>") || !strings.Contains(response.Header().Get("Content-Disposition"), "attachment") {
		t.Fatal("Markdown export missing or rendered as HTML")
	}
	request = httptest.NewRequest(http.MethodDelete, url, nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatal("history clear failed")
	}
	if memory, err := conversationContext(ctx, "device-one"); err != nil || memory != "" {
		t.Fatal("deleted history still included in context")
	}
}

func TestConversationSettingsRejectInvalidRanges(t *testing.T) {
	for key, value := range map[string]string{"conversation_idle_seconds": "-1", "conversation_history_days": "0", "conversation_context_messages": "101", "conversation_history_enabled": "yes"} {
		if settingsUpdateError(map[string]string{key: value}) == "" {
			t.Errorf("accepted invalid %s", key)
		}
	}
	if settingsUpdateError(map[string]string{"conversation_idle_seconds": "0", "conversation_context_messages": "0"}) != "" {
		t.Fatal("zero should disable timeout or context")
	}
}

func TestGeminiHistoryBuffersTranscriptChunksAndDropsInterruptedReply(t *testing.T) {
	var user, assistant []string
	s := &geminiSession{cb: RealtimeCallbacks{OnSTT: func(text string) { user = append(user, text) }, OnText: func(text string) { assistant = append(assistant, text) }}}
	for _, raw := range []string{
		`{"inputTranscription":{"text":"我的猫"},"outputTranscription":{"text":"团团"}}`,
		`{"input_transcription":{"text":"叫团团"},"output_transcription":{"text":"真可爱"}}`,
		`{"turnComplete":true}`,
		`{"inputTranscription":{"text":"下一句"},"outputTranscription":{"text":"未播完"},"interrupted":true,"turnComplete":true}`,
	} {
		var sc map[string]any
		if err := json.Unmarshal([]byte(raw), &sc); err != nil {
			t.Fatal(err)
		}
		s.handleServerContent(sc)
	}
	if len(user) != 2 || user[0] != "我的猫叫团团" || len(assistant) != 1 || assistant[0] != "团团真可爱" {
		t.Fatalf("chunk aggregation failed: %v %v", user, assistant)
	}
}

func TestConversationCorruptArchiveIsNotOverwritten(t *testing.T) {
	ctx := enableHistoryForTest(t)
	path, _ := conversationHistoryPath("device-one")
	if err := writeConversationFile(path, []conversationMessage{}); err != nil {
		t.Fatal(err)
	}
	const corrupt = "[] trailing-invalid-json"
	if err := os.WriteFile(path, []byte(corrupt), 0600); err != nil {
		t.Fatal(err)
	}
	if err := appendConversation(ctx, "device-one", "session", "openai", "user", "hello"); err == nil {
		t.Fatal("accepted corrupt archive")
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != corrupt {
		t.Fatal("corrupt archive was overwritten")
	}
}
