package agent

import (
	"testing"
	"time"
)

// 只有"推进中"的任务才应阻塞新推送；failed/done 必须放行重试。
func TestUpdateTaskInFlight(t *testing.T) {
	cases := map[string]bool{
		UpdateStatusPending:     true,
		UpdateStatusDownloading: true,
		UpdateStatusVerifying:   true,
		UpdateStatusReplacing:   true,
		UpdateStatusRestarting:  true,
		UpdateStatusDone:        false,
		UpdateStatusFailed:      false,
	}
	for status, want := range cases {
		task := &UpdateTask{Status: status}
		if got := task.inFlight(); got != want {
			t.Errorf("status %q: inFlight() = %v, want %v", status, got, want)
		}
	}
}

// 失败后按退避重排，达到 maxAttempts 后不再重试。
func TestScheduleRetryBackoff(t *testing.T) {
	u := NewAgentUpdater(nil, nil)

	task := &UpdateTask{ServerID: 12, Status: UpdateStatusFailed, Attempts: 1}
	u.scheduleRetryLocked(task)
	if task.NextRetryAt == nil {
		t.Fatalf("attempt 1 should schedule a retry")
	}
	if d := time.Until(*task.NextRetryAt); d < 25*time.Second || d > 35*time.Second {
		t.Errorf("first retry delay = %v, want ~30s", d)
	}

	task.Attempts = 2
	u.scheduleRetryLocked(task)
	if task.NextRetryAt == nil {
		t.Fatalf("attempt 2 should schedule a retry")
	}
	if d := time.Until(*task.NextRetryAt); d < 110*time.Second || d > 130*time.Second {
		t.Errorf("second retry delay = %v, want ~2m", d)
	}

	task.Attempts = u.maxAttempts
	u.scheduleRetryLocked(task)
	if task.NextRetryAt != nil {
		t.Errorf("attempt %d must not schedule further retries", u.maxAttempts)
	}
}

func TestRetryDelay(t *testing.T) {
	u := NewAgentUpdater(nil, nil)
	for attempt, want := range map[int]time.Duration{
		0: 30 * time.Second, // 防御：非法 attempt 按首次处理
		1: 30 * time.Second,
		2: 2 * time.Minute,
		9: 2 * time.Minute, // 超出退避表时取最后一档
	} {
		if got := u.retryDelay(attempt); got != want {
			t.Errorf("retryDelay(%d) = %v, want %v", attempt, got, want)
		}
	}
}

// 成功确认必须清掉先前失败留下的错误信息（否则 UI 上是"done 但带 error"）。
func TestOnAgentReconnectClearsError(t *testing.T) {
	u := NewAgentUpdater(nil, nil)
	task := &UpdateTask{
		ServerID:    18,
		FromVersion: "v0.12.298",
		ToVersion:   "v0.13.0-alpha.3",
		Status:      UpdateStatusFailed,
		Error:       "version mismatch: expected v0.13.0-alpha.3, got v0.12.298",
	}
	u.pendingUpdates[18] = task

	u.OnAgentReconnect(18, "v0.13.0-alpha.3")

	if task.Status != UpdateStatusDone {
		t.Fatalf("status = %s, want %s", task.Status, UpdateStatusDone)
	}
	if task.Error != "" {
		t.Errorf("error = %q, want empty after success", task.Error)
	}
	if task.NextRetryAt != nil {
		t.Errorf("NextRetryAt should be cleared after success")
	}
}
