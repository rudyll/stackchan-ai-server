/*
SPDX-FileCopyrightText: 2026 M5Stack Technology CO LTD
SPDX-License-Identifier: MIT
*/

package ai

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func waitForTaskStatus(t *testing.T, manager *backgroundTaskManager, owner, id, status string) *backgroundTask {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if task := manager.get(owner, id); task != nil && task.Status == status {
			return task
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("task %s did not reach %s", id, status)
	return nil
}

func TestBackgroundTasksSerializePerOwner(t *testing.T) {
	manager := newBackgroundTaskManager(filepath.Join(t.TempDir(), "tasks.json"), time.Hour)
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondStarted := make(chan struct{})

	first, err := manager.create("device-a", "first", func(ctx context.Context, _ string) (string, error) {
		close(firstStarted)
		select {
		case <-releaseFirst:
			return "first done", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.create("device-a", "second", func(context.Context, string) (string, error) {
		close(secondStarted)
		return "second done", nil
	})
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first task did not start")
	}
	select {
	case <-secondStarted:
		t.Fatal("second task started before the first completed")
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseFirst)
	waitForTaskStatus(t, manager, "device-a", first.ID, backgroundTaskCompleted)
	waitForTaskStatus(t, manager, "device-a", second.ID, backgroundTaskCompleted)
}

func TestBackgroundTasksExcludeOtherOwner(t *testing.T) {
	manager := newBackgroundTaskManager(filepath.Join(t.TempDir(), "tasks.json"), time.Hour)
	task, err := manager.create("device-a", "private", func(context.Context, string) (string, error) {
		return "secret result", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	waitForTaskStatus(t, manager, "device-a", task.ID, backgroundTaskCompleted)
	if got := manager.get("device-b", task.ID); got != nil {
		t.Fatal("task leaked to a different device owner")
	}
	if got := manager.latest("device-b"); got != nil {
		t.Fatal("latest task leaked to a different device owner")
	}
	if got := manager.claimPending("device-b", "other-session"); got != nil {
		t.Fatal("notification leaked to a different device owner")
	}
}

func TestBackgroundTaskCancelRunningTask(t *testing.T) {
	manager := newBackgroundTaskManager(filepath.Join(t.TempDir(), "tasks.json"), time.Hour)
	started := make(chan struct{})
	task, err := manager.create("device-a", "cancel me", func(ctx context.Context, _ string) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	<-started
	cancelled, err := manager.cancel("device-a", task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != backgroundTaskCancelled {
		t.Fatalf("status = %s", cancelled.Status)
	}
	if pending := manager.claimPending("device-a", "voice"); pending != nil {
		t.Fatal("cancelled task must not produce a completion announcement")
	}
}

func TestBackgroundTaskClaimReleasedForReconnect(t *testing.T) {
	manager := newBackgroundTaskManager(filepath.Join(t.TempDir(), "tasks.json"), time.Hour)
	task, err := manager.create("device-a", "announce", func(context.Context, string) (string, error) {
		return "done", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	waitForTaskStatus(t, manager, "device-a", task.ID, backgroundTaskCompleted)
	claimed := manager.claimPending("device-a", "voice-1")
	if claimed == nil || claimed.ID != task.ID {
		t.Fatal("first voice session did not claim result")
	}
	if duplicate := manager.claimPending("device-a", "voice-2"); duplicate != nil {
		t.Fatal("result was claimed twice")
	}
	if !manager.releaseClaim(task.ID, "voice-1") {
		t.Fatal("claim release failed")
	}
	if reclaimed := manager.claimPending("device-a", "voice-2"); reclaimed == nil || reclaimed.ID != task.ID {
		t.Fatal("reconnected session could not reclaim result")
	}
	if !manager.markDelivered(task.ID, "voice-2") {
		t.Fatal("mark delivered failed")
	}
	if duplicate := manager.claimPending("device-a", "voice-3"); duplicate != nil {
		t.Fatal("delivered result was announced again")
	}
}

func TestBackgroundTaskRestartAndExpiry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	manager := newBackgroundTaskManager(path, time.Hour)
	started := make(chan struct{})
	task, err := manager.create("device-a", "interrupted", func(ctx context.Context, _ string) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	<-started

	restarted := newBackgroundTaskManager(path, time.Hour)
	recovered := restarted.get("device-a", task.ID)
	if recovered == nil || recovered.Status != backgroundTaskFailed || recovered.NotificationStatus != backgroundNotificationPending {
		t.Fatalf("unexpected restart recovery: %#v", recovered)
	}
	_, _ = manager.cancel("device-a", task.ID)

	restarted.mu.Lock()
	restarted.tasks[task.ID].CompletedAt = time.Now().Add(-2 * time.Hour)
	restarted.persistLocked()
	restarted.mu.Unlock()
	if got := restarted.get("device-a", task.ID); got != nil {
		t.Fatal("expired terminal task was not pruned")
	}
}

func TestRealtimeDeliveryCompletionAndInterruption(t *testing.T) {
	manager := newBackgroundTaskManager(filepath.Join(t.TempDir(), "tasks.json"), time.Hour)
	makeCompleted := func(objective string) *backgroundTask {
		task, err := manager.create("device-a", objective, func(context.Context, string) (string, error) {
			return "done", nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return waitForTaskStatus(t, manager, "device-a", task.ID, backgroundTaskCompleted)
	}

	completed := makeCompleted("deliver once")
	if manager.claimPending("device-a", "voice") == nil {
		t.Fatal("could not claim completion")
	}
	session := &openaiRealtimeSession{
		tasks:              manager,
		claimantID:         "voice",
		deliveryWake:       make(chan struct{}, 1),
		deliveryTaskID:     completed.ID,
		deliveryResponseID: "response-1",
		activeResponseID:   "response-1",
		playbackBusy:       true,
	}
	session.finishResponse("response-1", "completed")
	if task := manager.get("device-a", completed.ID); task.NotificationStatus != backgroundNotificationDelivering {
		t.Fatalf("delivery completed before device playback: %s", task.NotificationStatus)
	}
	session.SetPlaybackBusy(false)
	if task := manager.get("device-a", completed.ID); task.NotificationStatus != backgroundNotificationDelivered {
		t.Fatalf("completed delivery status = %s", task.NotificationStatus)
	}

	interrupted := makeCompleted("retry after interruption")
	if manager.claimPending("device-a", "voice") == nil {
		t.Fatal("could not claim interrupted completion")
	}
	session.deliveryTaskID = interrupted.ID
	session.deliveryResponseID = "response-2"
	session.activeResponseID = "response-2"
	session.finishResponse("response-2", "cancelled")
	if task := manager.get("device-a", interrupted.ID); task.NotificationStatus != backgroundNotificationPending {
		t.Fatalf("interrupted delivery status = %s", task.NotificationStatus)
	}

	retry := makeCompleted("retry after playback abort")
	if manager.claimPending("device-a", "voice") == nil {
		t.Fatal("could not claim playback-abort completion")
	}
	session.deliveryTaskID = retry.ID
	session.deliveryResponseID = "response-3"
	session.activeResponseID = "response-3"
	session.playbackBusy = true
	session.finishResponse("response-3", "completed")
	session.InterruptPlayback()
	if task := manager.get("device-a", retry.ID); task.NotificationStatus != backgroundNotificationPending {
		t.Fatalf("playback-aborted delivery status = %s", task.NotificationStatus)
	}
}

func TestRealtimeDeliveryWaitsForHandshakeAndPlayback(t *testing.T) {
	manager := newBackgroundTaskManager(filepath.Join(t.TempDir(), "tasks.json"), time.Hour)
	task, err := manager.create("device-a", "wait for idle", func(context.Context, string) (string, error) {
		return "done", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	waitForTaskStatus(t, manager, "device-a", task.ID, backgroundTaskCompleted)
	session := &openaiRealtimeSession{
		deviceID:     "device-a",
		tasks:        manager,
		claimantID:   "voice",
		deliveryWake: make(chan struct{}, 1),
	}
	session.tryDeliverBackgroundResult()
	if got := manager.get("device-a", task.ID); got.NotificationStatus != backgroundNotificationPending {
		t.Fatalf("result claimed before hello: %s", got.NotificationStatus)
	}
	session.deliveryReady = true
	session.playbackBusy = true
	session.tryDeliverBackgroundResult()
	if got := manager.get("device-a", task.ID); got.NotificationStatus != backgroundNotificationPending {
		t.Fatalf("result claimed during playback: %s", got.NotificationStatus)
	}
}
