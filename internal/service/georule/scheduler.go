package georule

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"corefusion/internal/store/repository"
)

// Scheduler manages cron jobs for rule source syncing.
type Scheduler struct {
	sourceRepo *repository.RuleSourceRepository
	svc        *Service
	mu         sync.Mutex
	jobs       map[int64]*cronJob
	pushMu     sync.Mutex // 串行化节点推送，避免多个源同时推全节点
	ctx        context.Context
	cancel     context.CancelFunc
}

type cronJob struct {
	ticker *time.Ticker
	done   chan struct{}
}

// NewScheduler creates a new rule sync scheduler.
func NewScheduler(sourceRepo *repository.RuleSourceRepository, svc *Service) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		sourceRepo: sourceRepo,
		svc:        svc,
		jobs:       make(map[int64]*cronJob),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start loads all enabled sources and starts their cron jobs.
func (s *Scheduler) Start() {
	sources, err := s.sourceRepo.List(context.Background())
	if err != nil {
		log.Printf("scheduler: failed to load sources: %v", err)
		return
	}
	var overdue []int64
	for _, src := range sources {
		if !src.Enabled || src.SyncCron == "" {
			continue
		}
		s.addJob(src.ID, src.SyncCron)
		// 启动补跑：ticker 在面板重启后会从头计时，若上次同步已超过周期，
		// 说明这一轮被重启吞掉了，立即补一次，避免频繁重启导致任务饿死。
		if interval := parseCronInterval(src.SyncCron); interval > 0 {
			if src.LastSyncAt == nil || time.Since(*src.LastSyncAt) >= interval {
				log.Printf("scheduler: source %d overdue (last sync %v), catch-up now", src.ID, src.LastSyncAt)
				overdue = append(overdue, src.ID)
			}
		}
	}
	if len(overdue) > 0 {
		go s.catchUp(overdue)
	}
	log.Printf("scheduler: started %d cron jobs", len(s.jobs))
}

// Stop stops all cron jobs.
func (s *Scheduler) Stop() {
	s.cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, job := range s.jobs {
		close(job.done)
		job.ticker.Stop()
		delete(s.jobs, id)
	}
}

// RefreshJob updates the cron job for a source.
func (s *Scheduler) RefreshJob(sourceID int64, enabled bool, syncCron string) {
	s.removeJob(sourceID)
	if enabled && syncCron != "" {
		s.addJob(sourceID, syncCron)
	}
}

// removeJob stops and removes a cron job.
func (s *Scheduler) removeJob(sourceID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.jobs[sourceID]; ok {
		close(job.done)
		job.ticker.Stop()
		delete(s.jobs, sourceID)
	}
}

// addJob starts a new cron job for a source.
func (s *Scheduler) addJob(sourceID int64, syncCron string) {
	interval := parseCronInterval(syncCron)
	if interval <= 0 {
		log.Printf("scheduler: invalid cron %q for source %d, skipping", syncCron, sourceID)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Don't duplicate
	if _, ok := s.jobs[sourceID]; ok {
		return
	}

	done := make(chan struct{})
	job := &cronJob{done: done}
	s.jobs[sourceID] = job

	go func() {
		log.Printf("scheduler: source %d sync every %v", sourceID, interval)
		ticker := time.NewTicker(interval)
		job.ticker = ticker
		defer ticker.Stop()

		for {
			select {
			case <-s.ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				log.Printf("scheduler: syncing source %d", sourceID)
				s.runSync(sourceID)
			}
		}
	}()
}

// runSync 同步一个源；若是全局源（server_id=0），同步完成后把文件推送到所有节点。
// ⚠ 旧实现只调 Service.SyncSource()，它只下载到面板 data/rules/，
// 不会推送节点 —— 定时任务等于只更新了面板，节点要等手动推送或面板重启才更新。
func (s *Scheduler) runSync(sourceID int64) {
	if global, ok := s.syncSource(sourceID); ok && global {
		s.pushGlobalFiles(fmt.Sprintf("source %d", sourceID))
	}
}

// catchUp 启动补跑：逐个同步过期源，最后只推送一次节点。
func (s *Scheduler) catchUp(ids []int64) {
	needPush := false
	for _, id := range ids {
		if global, ok := s.syncSource(id); ok && global {
			needPush = true
		}
	}
	if needPush {
		s.pushGlobalFiles("startup catch-up")
	}
}

// syncSource 把源下载到面板本地，返回该源是否为全局源以及是否同步成功。
func (s *Scheduler) syncSource(sourceID int64) (global, ok bool) {
	src, err := s.sourceRepo.GetByID(s.ctx, sourceID)
	if err != nil {
		log.Printf("scheduler: load source %d failed: %v", sourceID, err)
		return false, false
	}
	if _, err := s.svc.SyncSource(s.ctx, sourceID); err != nil {
		log.Printf("scheduler: sync source %d failed: %v", sourceID, err)
		return false, false
	}
	log.Printf("scheduler: source %d synced", sourceID)
	return src.ServerID == 0, true
}

// pushGlobalFiles 把面板上的全局 geo 文件推送到所有节点（Agent 按 hash 去重）。
func (s *Scheduler) pushGlobalFiles(reason string) {
	s.pushMu.Lock()
	defer s.pushMu.Unlock()
	if err := s.svc.SyncToAllNodes(s.ctx); err != nil {
		log.Printf("scheduler: push global files to nodes failed (%s): %v", reason, err)
		return
	}
	log.Printf("scheduler: pushed global files to all nodes (%s)", reason)
}

// parseCronInterval converts a simple cron expression to a time.Duration.
// Supported formats:
//   - "@every 1h", "@every 30m", "@every 6h"
//   - "0 3 * * *" -> 24h (daily at 3am)
//   - "0 */6 * * *" -> 6h
//   - Empty or invalid -> 0
func parseCronInterval(cron string) time.Duration {
	// Handle @every syntax
	if len(cron) > 7 && cron[:7] == "@every " {
		d, err := time.ParseDuration(cron[7:])
		if err == nil {
			return d
		}
	}

	// Common presets
	presets := map[string]time.Duration{
		"0 * * * *":    1 * time.Hour,      // every hour
		"0 */2 * * *":  2 * time.Hour,      // every 2 hours
		"0 */6 * * *":  6 * time.Hour,      // every 6 hours
		"0 */12 * * *": 12 * time.Hour,     // every 12 hours
		"0 0 * * *":    24 * time.Hour,     // daily midnight
		"0 3 * * *":    24 * time.Hour,     // daily 3am
		"0 0 * * 0":    7 * 24 * time.Hour, // weekly
	}
	if d, ok := presets[cron]; ok {
		return d
	}

	// Try to extract hour interval from "0 */N * * *"
	if len(cron) > 6 && cron[:2] == "0 " && cron[2] == '*' && cron[3] == '/' {
		var n int
		for i := 4; i < len(cron) && cron[i] >= '0' && cron[i] <= '9'; i++ {
			n = n*10 + int(cron[i]-'0')
		}
		if n > 0 {
			return time.Duration(n) * time.Hour
		}
	}

	return 0
}
