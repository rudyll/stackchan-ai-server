/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

const (
	backgroundTaskQueued    = "queued"
	backgroundTaskRunning   = "running"
	backgroundTaskCompleted = "completed"
	backgroundTaskFailed    = "failed"
	backgroundTaskCancelled = "cancelled"

	backgroundNotificationNone       = "none"
	backgroundNotificationPending    = "pending"
	backgroundNotificationDelivering = "delivering"
	backgroundNotificationDelivered  = "delivered"
)

type backgroundTask struct {
	ID                    string    `json:"id"`
	OwnerID               string    `json:"owner_id"`
	Objective             string    `json:"objective"`
	Status                string    `json:"status"`
	CreatedAt             time.Time `json:"created_at"`
	StartedAt             time.Time `json:"started_at,omitempty"`
	CompletedAt           time.Time `json:"completed_at,omitempty"`
	Result                string    `json:"result,omitempty"`
	Error                 string    `json:"error,omitempty"`
	NotificationStatus    string    `json:"notification_status"`
	NotificationClaimant  string    `json:"notification_claimant,omitempty"`
	NotificationClaimedAt time.Time `json:"notification_claimed_at,omitempty"`
}

type backgroundTaskRunner func(context.Context, string) (string, error)

type backgroundTaskManager struct {
	mu        sync.Mutex
	tasks     map[string]*backgroundTask
	runners   map[string]backgroundTaskRunner
	cancels   map[string]context.CancelFunc
	dones     map[string]chan struct{}
	active    map[string]bool
	watchers  map[chan struct{}]struct{}
	storePath string
	ttl       time.Duration
	claimTTL  time.Duration
	now       func() time.Time
}

func newBackgroundTaskManager(storePath string, ttl time.Duration) *backgroundTaskManager {
	m := &backgroundTaskManager{
		tasks:     map[string]*backgroundTask{},
		runners:   map[string]backgroundTaskRunner{},
		cancels:   map[string]context.CancelFunc{},
		dones:     map[string]chan struct{}{},
		active:    map[string]bool{},
		watchers:  map[chan struct{}]struct{}{},
		storePath: storePath,
		ttl:       ttl,
		claimTTL:  time.Minute,
		now:       time.Now,
	}
	m.load()
	return m
}

var defaultBackgroundTasks = newBackgroundTaskManager(filepath.Join(stackChanDataDir(), "background-tasks.json"), 7*24*time.Hour)

func cloneBackgroundTask(task *backgroundTask) *backgroundTask {
	if task == nil {
		return nil
	}
	copy := *task
	return &copy
}

func (m *backgroundTaskManager) load() {
	data, err := os.ReadFile(m.storePath)
	if err != nil {
		return
	}
	var saved []*backgroundTask
	if err := json.Unmarshal(data, &saved); err != nil {
		g.Log().Warningf(context.Background(), "[BACKGROUND] could not read task state: %v", err)
		return
	}
	now := m.now()
	for _, task := range saved {
		if task == nil || task.ID == "" || task.OwnerID == "" {
			continue
		}
		if !task.CompletedAt.IsZero() && now.Sub(task.CompletedAt) > m.ttl {
			continue
		}
		if task.Status == backgroundTaskQueued || task.Status == backgroundTaskRunning {
			task.Status = backgroundTaskFailed
			task.Error = "StackChan 服务重启时这项后台任务尚未完成，请重新提交。"
			task.CompletedAt = now
			task.NotificationStatus = backgroundNotificationPending
		}
		if task.NotificationStatus == backgroundNotificationDelivering {
			task.NotificationStatus = backgroundNotificationPending
			task.NotificationClaimant = ""
			task.NotificationClaimedAt = time.Time{}
		}
		m.tasks[task.ID] = task
	}
	m.persistLocked()
}

func (m *backgroundTaskManager) persistLocked() {
	if m.storePath == "" {
		return
	}
	items := make([]*backgroundTask, 0, len(m.tasks))
	for _, task := range m.tasks {
		items = append(items, cloneBackgroundTask(task))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		g.Log().Warningf(context.Background(), "[BACKGROUND] could not encode task state: %v", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(m.storePath), 0700); err != nil {
		g.Log().Warningf(context.Background(), "[BACKGROUND] could not create task state directory: %v", err)
		return
	}
	tmp := m.storePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		g.Log().Warningf(context.Background(), "[BACKGROUND] could not write task state: %v", err)
		return
	}
	if err := os.Rename(tmp, m.storePath); err != nil {
		g.Log().Warningf(context.Background(), "[BACKGROUND] could not replace task state: %v", err)
	}
}

func (m *backgroundTaskManager) signalLocked() {
	for watcher := range m.watchers {
		select {
		case watcher <- struct{}{}:
		default:
		}
	}
}

func (m *backgroundTaskManager) subscribe() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	m.mu.Lock()
	m.watchers[ch] = struct{}{}
	m.mu.Unlock()
	return ch, func() {
		m.mu.Lock()
		delete(m.watchers, ch)
		m.mu.Unlock()
	}
}

func (m *backgroundTaskManager) create(ownerID, objective string, runner backgroundTaskRunner) (*backgroundTask, error) {
	ownerID = strings.TrimSpace(ownerID)
	objective = strings.TrimSpace(objective)
	if ownerID == "" {
		return nil, errors.New("missing device owner")
	}
	if objective == "" {
		return nil, errors.New("objective is required")
	}
	if runner == nil {
		return nil, errors.New("background agent is not configured")
	}
	now := m.now()
	task := &backgroundTask{
		ID:                 fmt.Sprintf("work_%d", now.UnixNano()),
		OwnerID:            ownerID,
		Objective:          objective,
		Status:             backgroundTaskQueued,
		CreatedAt:          now,
		NotificationStatus: backgroundNotificationNone,
	}
	m.mu.Lock()
	for {
		if _, exists := m.tasks[task.ID]; !exists {
			break
		}
		task.ID += "x"
	}
	m.tasks[task.ID] = task
	m.runners[task.ID] = runner
	m.persistLocked()
	m.signalLocked()
	created := cloneBackgroundTask(task)
	m.mu.Unlock()
	go m.drain(ownerID)
	return created, nil
}

func (m *backgroundTaskManager) drain(ownerID string) {
	m.mu.Lock()
	if m.active[ownerID] {
		m.mu.Unlock()
		return
	}
	var selected *backgroundTask
	for _, task := range m.tasks {
		if task.OwnerID != ownerID || task.Status != backgroundTaskQueued {
			continue
		}
		if selected == nil || task.CreatedAt.Before(selected.CreatedAt) {
			selected = task
		}
	}
	if selected == nil {
		m.mu.Unlock()
		return
	}
	runner := m.runners[selected.ID]
	if runner == nil {
		selected.Status = backgroundTaskFailed
		selected.Error = "后台任务执行器不可用，请重新提交。"
		selected.CompletedAt = m.now()
		selected.NotificationStatus = backgroundNotificationPending
		m.persistLocked()
		m.signalLocked()
		m.mu.Unlock()
		go m.drain(ownerID)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancels[selected.ID] = cancel
	m.dones[selected.ID] = make(chan struct{})
	m.active[ownerID] = true
	selected.Status = backgroundTaskRunning
	selected.StartedAt = m.now()
	m.persistLocked()
	m.signalLocked()
	taskID, objective := selected.ID, selected.Objective
	m.mu.Unlock()

	go func() {
		result, err := runner(ctx, objective)
		m.finish(taskID, result, err)
	}()
}

func (m *backgroundTaskManager) finish(taskID, result string, runErr error) {
	m.mu.Lock()
	task := m.tasks[taskID]
	if task == nil {
		m.mu.Unlock()
		return
	}
	delete(m.cancels, taskID)
	delete(m.runners, taskID)
	delete(m.active, task.OwnerID)
	if task.Status != backgroundTaskCancelled {
		task.CompletedAt = m.now()
		if runErr != nil {
			task.Status = backgroundTaskFailed
			task.Error = runErr.Error()
		} else {
			task.Status = backgroundTaskCompleted
			task.Result = strings.TrimSpace(result)
			if task.Result == "" {
				task.Status = backgroundTaskFailed
				task.Error = "后台 Agent 没有返回结果。"
			}
		}
		task.NotificationStatus = backgroundNotificationPending
	}
	ownerID := task.OwnerID
	m.persistLocked()
	m.signalLocked()
	if done := m.dones[taskID]; done != nil {
		close(done)
		delete(m.dones, taskID)
	}
	m.mu.Unlock()
	go m.drain(ownerID)
}

func (m *backgroundTaskManager) get(ownerID, taskID string) *backgroundTask {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked()
	task := m.tasks[taskID]
	if task == nil || task.OwnerID != ownerID {
		return nil
	}
	return cloneBackgroundTask(task)
}

func (m *backgroundTaskManager) latest(ownerID string) *backgroundTask {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked()
	var latest *backgroundTask
	for _, task := range m.tasks {
		if task.OwnerID != ownerID {
			continue
		}
		if latest == nil || task.CreatedAt.After(latest.CreatedAt) {
			latest = task
		}
	}
	return cloneBackgroundTask(latest)
}

func (m *backgroundTaskManager) cancel(ownerID, taskID string) (*backgroundTask, error) {
	m.mu.Lock()
	task := m.tasks[taskID]
	if task == nil || task.OwnerID != ownerID {
		m.mu.Unlock()
		return nil, errors.New("task not found")
	}
	if task.Status != backgroundTaskQueued && task.Status != backgroundTaskRunning {
		copy := cloneBackgroundTask(task)
		m.mu.Unlock()
		return copy, errors.New("task is no longer active")
	}
	cancel := m.cancels[taskID]
	done := m.dones[taskID]
	wasRunning := task.Status == backgroundTaskRunning
	if !wasRunning {
		delete(m.runners, taskID)
	}
	task.Status = backgroundTaskCancelled
	task.CompletedAt = m.now()
	task.Error = ""
	task.NotificationStatus = backgroundNotificationNone
	owner := task.OwnerID
	copy := cloneBackgroundTask(task)
	m.persistLocked()
	m.signalLocked()
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if wasRunning && done != nil {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			return copy, errors.New("task cancellation was not confirmed")
		}
	} else {
		go m.drain(owner)
	}
	return copy, nil
}

func (m *backgroundTaskManager) claimPending(ownerID, claimant string) *backgroundTask {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reclaimExpiredClaimsLocked()
	m.pruneLocked()
	var selected *backgroundTask
	for _, task := range m.tasks {
		if task.OwnerID != ownerID || task.NotificationStatus != backgroundNotificationPending {
			continue
		}
		if selected == nil || task.CreatedAt.Before(selected.CreatedAt) {
			selected = task
		}
	}
	if selected == nil {
		return nil
	}
	selected.NotificationStatus = backgroundNotificationDelivering
	selected.NotificationClaimant = claimant
	selected.NotificationClaimedAt = m.now()
	m.persistLocked()
	return cloneBackgroundTask(selected)
}

func (m *backgroundTaskManager) markDelivered(taskID, claimant string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	task := m.tasks[taskID]
	if task == nil || task.NotificationStatus != backgroundNotificationDelivering || task.NotificationClaimant != claimant {
		return false
	}
	task.NotificationStatus = backgroundNotificationDelivered
	task.NotificationClaimant = ""
	task.NotificationClaimedAt = time.Time{}
	m.persistLocked()
	m.signalLocked()
	return true
}

func (m *backgroundTaskManager) releaseClaim(taskID, claimant string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	task := m.tasks[taskID]
	if task == nil || task.NotificationStatus != backgroundNotificationDelivering || task.NotificationClaimant != claimant {
		return false
	}
	task.NotificationStatus = backgroundNotificationPending
	task.NotificationClaimant = ""
	task.NotificationClaimedAt = time.Time{}
	m.persistLocked()
	m.signalLocked()
	return true
}

func (m *backgroundTaskManager) reclaimExpiredClaimsLocked() {
	now := m.now()
	for _, task := range m.tasks {
		if task.NotificationStatus == backgroundNotificationDelivering &&
			!task.NotificationClaimedAt.IsZero() && now.Sub(task.NotificationClaimedAt) >= m.claimTTL {
			task.NotificationStatus = backgroundNotificationPending
			task.NotificationClaimant = ""
			task.NotificationClaimedAt = time.Time{}
		}
	}
}

func (m *backgroundTaskManager) pruneLocked() {
	now := m.now()
	changed := false
	for id, task := range m.tasks {
		if task.CompletedAt.IsZero() || now.Sub(task.CompletedAt) <= m.ttl {
			continue
		}
		delete(m.tasks, id)
		delete(m.runners, id)
		delete(m.cancels, id)
		changed = true
	}
	if changed {
		m.persistLocked()
	}
}

type backgroundAgentConfig struct {
	Enabled bool
	BaseURL string
	APIKey  string
	Model   string
	Prompt  string
	Timeout time.Duration
	HAURL   string
	HAToken string
}

func loadBackgroundAgentConfig(ctx context.Context) backgroundAgentConfig {
	baseURL := aiString(ctx, "background_agent_base_url", "")
	if baseURL == "" {
		baseURL = override(aiString(ctx, "llm_base_url", ""), aiString(ctx, "compatible_base_url", "https://api.openai.com"))
	}
	apiKey := aiString(ctx, "background_agent_api_key", "")
	if apiKey == "" {
		apiKey = override(aiString(ctx, "llm_api_key", ""), override(aiString(ctx, "compatible_api_key", ""), aiString(ctx, "openai_api_key", "")))
	}
	model := aiString(ctx, "background_agent_model", "")
	if model == "" {
		model = override(aiString(ctx, "llm_model", ""), aiString(ctx, "compatible_model", ""))
	}
	timeoutSeconds := aiInt(ctx, "background_agent_timeout_seconds", 300)
	haEnabled := aiBool(ctx, "ha_enabled", true)
	return backgroundAgentConfig{
		Enabled: haEnabled && aiBool(ctx, "background_tasks_enabled", false) && apiKey != "" && model != "",
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
		Prompt:  aiString(ctx, "background_agent_prompt", "你是 StackChan 的后台任务 Agent。使用可用的 Home Assistant 工具完成目标，最终只返回准确、简洁、适合中文语音播报的结果。不要声称执行未完成的操作。"),
		Timeout: time.Duration(max(10, timeoutSeconds)) * time.Second,
		HAURL:   g.Cfg().MustGet(ctx, "ai.ha_ws_url", "ws://homeassistant:8123/api/websocket").String(),
		HAToken: g.Cfg().MustGet(ctx, "ai.ha_mcp_token", "").String(),
	}
}

func (cfg backgroundAgentConfig) runner() backgroundTaskRunner {
	if !cfg.Enabled {
		return nil
	}
	return func(ctx context.Context, objective string) (string, error) {
		runCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
		ha, err := dialHAWebSocket(cfg.HAURL, cfg.HAToken)
		if err != nil {
			return "", fmt.Errorf("后台 Agent 连接 Home Assistant 失败: %w", err)
		}
		defer ha.Close()
		client := newOpenAIClient(cfg.BaseURL, cfg.APIKey, cfg.Model, "", "", "", cfg.Prompt)
		return client.Chat(runCtx, []chatMessage{{Role: "user", Content: objective}}, ha)
	}
}

func backgroundRealtimeTools() []map[string]any {
	return []map[string]any{
		{
			"type": "function", "name": "start_background_task",
			"description": "Start a task that may take time. Return immediately and keep talking to the user while it runs. Use for analysis or multi-step work, not simple Home Assistant commands.",
			"parameters": map[string]any{"type": "object", "properties": map[string]any{
				"objective": map[string]any{"type": "string", "description": "Conservative statement of the user's requested outcome."},
			}, "required": []string{"objective"}},
		},
		{
			"type": "function", "name": "get_background_task_status",
			"description": "Get the latest background task status, or a specific task by work_id.",
			"parameters": map[string]any{"type": "object", "properties": map[string]any{
				"work_id": map[string]any{"type": "string"},
			}},
		},
		{
			"type": "function", "name": "cancel_background_task",
			"description": "Cancel an active background task.",
			"parameters": map[string]any{"type": "object", "properties": map[string]any{
				"work_id": map[string]any{"type": "string"},
			}, "required": []string{"work_id"}},
		},
	}
}

func backgroundTaskJSON(task *backgroundTask) string {
	if task == nil {
		return `{"error":"没有找到后台任务"}`
	}
	result := map[string]any{
		"work_id":   task.ID,
		"status":    task.Status,
		"objective": task.Objective,
	}
	if task.Status == backgroundTaskCompleted {
		result["result"] = task.Result
	}
	if task.Status == backgroundTaskFailed {
		result["error"] = task.Error
	}
	data, _ := json.Marshal(result)
	return string(data)
}
