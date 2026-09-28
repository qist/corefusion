package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	agentpb "corefusion/internal/runtime/agent/proto"
)

// ============================================================
// TaskQueue — 异步任务队列 + Worker Pool
// ============================================================
//
// 核心设计：
//   所有"系统控制操作"（start/stop/restart/reload/sync config）
//   都是异步任务。TaskQueue 管理任务的生命周期：
//     1. Submit: 接收任务 → 检查幂等 → 入队 → 返回 TaskAccept
//     2. Worker: 从队列取出 → 标记 RUNNING → 执行 → 标记 SUCCESS/FAILED
//     3. Result: 通过 callback 回传 TaskResult（stream 或本地 channel）
//     4. GetTask: 查询任务状态
//
//   类 Kubernetes runtime 模型：
//     Panel → Agent: 下发命令（立刻返回 accepted + task_id）
//     Agent 内部:   TaskQueue → Worker Pool → 执行
//     Agent → Panel: 通过 stream 回传 TaskResult
//     Panel 查询:    GetTask(task_id) 获取最新状态

// TaskFunc 是任务的实际执行函数。
// 返回的 detail 会被序列化为 JSON 存入 TaskResult.DetailJson。
type TaskFunc func(ctx context.Context) (detail interface{}, err error)

// TaskResultCallback 在任务完成后被调用，用于回传结果。
// 对于远程 Agent，通过 stream 发送 TaskResult。
// 对于本地面板，通过 channel 传递。
type TaskResultCallback func(result *agentpb.TaskResult)

// taskEntry 是内部任务记录。
type taskEntry struct {
	ID             string
	IdempotencyKey string
	Type           string
	Status         agentpb.TaskStatus
	CreatedAt      int64
	FinishedAt     int64
	Error          string
	DetailJSON     string
	ResultCallback TaskResultCallback
	Execute        TaskFunc
}

// TaskQueue 管理异步任务的提交、执行和状态查询。
type TaskQueue struct {
	mu      sync.RWMutex
	tasks   map[string]*taskEntry // taskID → entry
	byKey   map[string]*taskEntry // idempotencyKey → entry（幂等去重）
	queue   chan *taskEntry       // 工作队列
	workers int                   // Worker 数量
	wg      sync.WaitGroup
	stopCh  chan struct{}
}

// NewTaskQueue 创建新的任务队列。
// workers 控制 Worker 池大小（推荐 4，避免过多并发操作系统进程）。
func NewTaskQueue(workers int) *TaskQueue {
	if workers <= 0 {
		workers = 4
	}
	tq := &TaskQueue{
		tasks:   make(map[string]*taskEntry),
		byKey:   make(map[string]*taskEntry),
		queue:   make(chan *taskEntry, 256),
		workers: workers,
		stopCh:  make(chan struct{}),
	}
	tq.startWorkers()
	tq.startCleanupLoop()
	return tq
}

// startWorkers 启动 Worker goroutine。
func (tq *TaskQueue) startWorkers() {
	for i := 0; i < tq.workers; i++ {
		tq.wg.Add(1)
		go func(workerID int) {
			defer tq.wg.Done()
			log.Printf("[TaskQueue] worker %d started", workerID)
			for {
				select {
				case <-tq.stopCh:
					log.Printf("[TaskQueue] worker %d stopped", workerID)
					return
				case entry := <-tq.queue:
					tq.executeTask(entry)
				}
			}
		}(i)
	}
}

// Submit 提交一个异步任务。
// 如果 idempotencyKey 与仍在运行（QUEUED/RUNNING）的任务重复，返回已有任务的 TaskAccept（不重复执行）。
// 如果已有任务已完成（SUCCESS/FAILED），则移除旧记录并允许新任务提交。
// 这确保了 start→stop→start 这样的操作序列不会被幂等机制错误阻断。
func (tq *TaskQueue) Submit(taskID, taskType, idempotencyKey string, fn TaskFunc, callback TaskResultCallback) *agentpb.TaskAccept {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	// 幂等检查：同一 key 的任务如果仍在运行（QUEUED/RUNNING），直接返回不重复执行。
	// 如果已有任务已完成（SUCCESS/FAILED），移除旧记录，允许新任务提交。
	if idempotencyKey != "" {
		if existing, ok := tq.byKey[idempotencyKey]; ok {
			if existing.Status == agentpb.TaskStatus_TASK_QUEUED || existing.Status == agentpb.TaskStatus_TASK_RUNNING {
				log.Printf("[TaskQueue] idempotent hit (active): key=%s → task_id=%s status=%s",
					idempotencyKey, existing.ID, existing.Status)
				return &agentpb.TaskAccept{
					Accepted:       true,
					TaskId:         existing.ID,
					IdempotencyKey: idempotencyKey,
					RejectReason:   "duplicate: task already in progress",
				}
			}
			// 任务已完成，移除旧记录以允许重新提交
			log.Printf("[TaskQueue] idempotent key recycled: key=%s old_task=%s old_status=%s",
				idempotencyKey, existing.ID, existing.Status)
			delete(tq.tasks, existing.ID)
			delete(tq.byKey, idempotencyKey)
		}
	}

	now := time.Now().Unix()
	entry := &taskEntry{
		ID:             taskID,
		IdempotencyKey: idempotencyKey,
		Type:           taskType,
		Status:         agentpb.TaskStatus_TASK_QUEUED,
		CreatedAt:      now,
		ResultCallback: callback,
		Execute:        fn,
	}

	tq.tasks[taskID] = entry
	if idempotencyKey != "" {
		tq.byKey[idempotencyKey] = entry
	}

	// 入队（非阻塞，队列满则拒绝）
	select {
	case tq.queue <- entry:
		log.Printf("[TaskQueue] task submitted: id=%s type=%s key=%s", taskID, taskType, idempotencyKey)
	default:
		// 队列满，拒绝
		entry.Status = agentpb.TaskStatus_TASK_FAILED
		entry.Error = "task queue full"
		entry.FinishedAt = time.Now().Unix()
		log.Printf("[TaskQueue] task rejected (queue full): id=%s", taskID)
		return &agentpb.TaskAccept{
			Accepted:     false,
			TaskId:       taskID,
			RejectReason: "task queue full",
		}
	}

	return &agentpb.TaskAccept{
		Accepted:       true,
		TaskId:         taskID,
		IdempotencyKey: idempotencyKey,
	}
}

// executeTask 由 Worker 调用，执行任务并更新状态。
func (tq *TaskQueue) executeTask(entry *taskEntry) {
	// 标记 RUNNING
	tq.mu.Lock()
	entry.Status = agentpb.TaskStatus_TASK_RUNNING
	tq.mu.Unlock()

	log.Printf("[TaskQueue] task running: id=%s type=%s", entry.ID, entry.Type)

	// 执行
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	detail, err := entry.Execute(ctx)

	now := time.Now().Unix()

	tq.mu.Lock()
	if err != nil {
		entry.Status = agentpb.TaskStatus_TASK_FAILED
		entry.Error = err.Error()
	} else {
		entry.Status = agentpb.TaskStatus_TASK_SUCCESS
	}
	entry.FinishedAt = now
	if detail != nil {
		if b, jsonErr := json.Marshal(detail); jsonErr == nil {
			entry.DetailJSON = string(b)
		}
	}

	// 构造 TaskResult
	result := &agentpb.TaskResult{
		TaskId:         entry.ID,
		IdempotencyKey: entry.IdempotencyKey,
		Status:         entry.Status,
		Error:          entry.Error,
		CreatedAt:      entry.CreatedAt,
		FinishedAt:     entry.FinishedAt,
		TaskType:       entry.Type,
		DetailJson:     entry.DetailJSON,
	}
	callback := entry.ResultCallback
	tq.mu.Unlock()

	log.Printf("[TaskQueue] task finished: id=%s status=%s err=%v", entry.ID, entry.Status, err)

	// 回传结果
	if callback != nil {
		callback(result)
	}
}

// GetTask 查询任务状态。
func (tq *TaskQueue) GetTask(taskID string) *agentpb.GetTaskResponse {
	tq.mu.RLock()
	defer tq.mu.RUnlock()

	entry, ok := tq.tasks[taskID]
	if !ok {
		return &agentpb.GetTaskResponse{
			TaskId: taskID,
			Status: agentpb.TaskStatus_TASK_UNKNOWN,
			Error:  "task not found",
		}
	}

	return &agentpb.GetTaskResponse{
		TaskId:     entry.ID,
		Status:     entry.Status,
		Error:      entry.Error,
		CreatedAt:  entry.CreatedAt,
		FinishedAt: entry.FinishedAt,
		TaskType:   entry.Type,
		DetailJson: entry.DetailJSON,
	}
}

// startCleanupLoop 启动定期清理 goroutine，防止已完成的任务记录无限增长。
func (tq *TaskQueue) startCleanupLoop() {
	tq.wg.Add(1)
	go func() {
		defer tq.wg.Done()
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-tq.stopCh:
				return
			case <-ticker.C:
				tq.Cleanup(10 * time.Minute)
			}
		}
	}()
}

// Stop 优雅停止 TaskQueue，等待所有 Worker 退出。
func (tq *TaskQueue) Stop() {
	close(tq.stopCh)
	tq.wg.Wait()
}

// Cleanup 清理超过 maxAge 的已完成任务。
func (tq *TaskQueue) Cleanup(maxAge time.Duration) {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	now := time.Now().Unix()
	cutoff := now - int64(maxAge.Seconds())

	for id, entry := range tq.tasks {
		if entry.FinishedAt > 0 && entry.FinishedAt < cutoff {
			delete(tq.tasks, id)
			if entry.IdempotencyKey != "" {
				delete(tq.byKey, entry.IdempotencyKey)
			}
		}
	}
}

// GenerateTaskID 生成唯一的任务 ID（导出版本，供 agent 包外部使用）。
func GenerateTaskID() string {
	return fmt.Sprintf("task-%d-%04d", time.Now().UnixNano(), time.Now().Nanosecond()%10000)
}
