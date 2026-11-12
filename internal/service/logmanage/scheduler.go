package logmanage

import (
	"context"
	"log"
	"sync"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/service/cronutil"
	"corefusion/internal/store/repository"
)

// Scheduler manages cron jobs for log rotation.
type Scheduler struct {
	policyRepo *repository.LogRotationPolicyRepository
	jobRepo    *repository.LogRotationJobRepository
	svc        *Service
	mu         sync.Mutex
	jobs       map[int64]*cronJob
	ctx        context.Context
	cancel     context.CancelFunc
}

type cronJob struct {
	done chan struct{}
}

// NewScheduler creates a new log rotation scheduler.
func NewScheduler(policyRepo *repository.LogRotationPolicyRepository, jobRepo *repository.LogRotationJobRepository, svc *Service) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		policyRepo: policyRepo,
		jobRepo:    jobRepo,
		svc:        svc,
		jobs:       make(map[int64]*cronJob),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start loads all enabled local policies (serverID=1) and starts their cron jobs.
// Remote node policies are handled by their respective agents and should not be
// scheduled on the local scheduler.
//
// ⚠ 调度必须按**墙上时钟**算下一次执行时间（见 nextRotation），不能用
// `time.NewTicker(24h)`：面板/Agent 经常一天重启多次，ticker 每次都从头计时，
// 永远走不到 24 小时 → 生产的日志轮转 11-10 之后就再没执行过（真实现象）。
// 启动时还会补跑一次「上次执行之后已经错过的时间点」，避免重启把当天的轮转吞掉。
func (s *Scheduler) Start() {
	policies, err := s.policyRepo.ListEnabledByServerID(context.Background(), 1)
	if err != nil {
		log.Printf("logmanage scheduler: failed to load policies: %v", err)
		return
	}
	for _, p := range policies {
		if p.RotateCron == "" {
			continue
		}
		if _, ok := nextRotation(p.RotateCron, time.Now()); !ok {
			log.Printf("logmanage scheduler: invalid cron %q for policy %d, skipping", p.RotateCron, p.ID)
			continue
		}
		s.addJob(p.ID, p.RotateCron)
		s.catchUp(&p)
	}
	log.Printf("logmanage scheduler: started %d rotation jobs", len(s.jobs))
}

// Stop stops all cron jobs.
func (s *Scheduler) Stop() {
	s.cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, job := range s.jobs {
		close(job.done)
		delete(s.jobs, id)
	}
}

// RefreshJob updates the cron job for a policy.
func (s *Scheduler) RefreshJob(policyID int64, enabled bool, rotateCron string) {
	s.removeJob(policyID)
	if enabled && rotateCron != "" {
		s.addJob(policyID, rotateCron)
	}
}

// RemoveJob stops and removes a cron job.
func (s *Scheduler) RemoveJob(policyID int64) {
	s.removeJob(policyID)
}

// ActiveJobs returns the number of active cron jobs.
func (s *Scheduler) ActiveJobs() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.jobs)
}

// removeJob stops and removes a cron job.
func (s *Scheduler) removeJob(policyID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.jobs[policyID]; ok {
		close(job.done)
		delete(s.jobs, policyID)
	}
}

// catchUp 在启动时补跑「按计划已经过期但没执行」的轮转。
// 基准取该策略最近一次任务记录（没有则策略创建时间）；若基准之后的下一个
// 计划时间点已经过去，说明期间被重启吞掉了，立即补跑一次。
func (s *Scheduler) catchUp(p *entity.LogRotationPolicy) {
	base := p.CreatedAt
	if s.jobRepo != nil {
		if latest, err := s.jobRepo.GetLatestByPolicy(context.Background(), p.ID); err == nil && latest != nil {
			if !latest.FinishedAt.IsZero() {
				base = latest.FinishedAt
			} else if !latest.StartedAt.IsZero() {
				base = latest.StartedAt
			}
		}
	}
	if base.IsZero() {
		return
	}
	next, ok := nextRotation(p.RotateCron, base)
	if !ok || next.After(time.Now()) {
		return
	}
	log.Printf("logmanage scheduler: policy %d missed scheduled run at %s (last run %s), rotating now",
		p.ID, next.Format("2006-01-02 15:04"), base.Format("2006-01-02 15:04"))
	go s.svc.RotateExpiredLogs(context.Background(), p.ID)
}

// addJob starts a new cron job for a policy.
func (s *Scheduler) addJob(policyID int64, rotateCron string) {
	s.mu.Lock()
	if _, ok := s.jobs[policyID]; ok {
		s.mu.Unlock()
		return
	}
	done := make(chan struct{})
	s.jobs[policyID] = &cronJob{done: done}
	s.mu.Unlock()

	go func() {
		for {
			next, ok := nextRotation(rotateCron, time.Now())
			if !ok {
				log.Printf("logmanage scheduler: invalid cron %q for policy %d, job stopped", rotateCron, policyID)
				return
			}
			wait := time.Until(next)
			if wait < 0 {
				wait = 0
			}
			log.Printf("logmanage scheduler: policy %d next rotation at %s (in %s)",
				policyID, next.Format("2006-01-02 15:04"), wait.Round(time.Second))
			timer := time.NewTimer(wait)
			select {
			case <-s.ctx.Done():
				timer.Stop()
				return
			case <-done:
				timer.Stop()
				return
			case <-timer.C:
				log.Printf("logmanage scheduler: rotating policy %d (scheduled %s)",
					policyID, next.Format("2006-01-02 15:04"))
				s.svc.RotateExpiredLogs(s.ctx, policyID)
			}
		}
	}()
}

// nextRotation 返回 after 之后（严格晚于）的下一个计划执行时间。
//
// 解析规则与 DB 日志清理共用 cronutil（见 internal/service/cronutil）：
//   - "@every 1h" / "@every 30m"
//   - 5 段 cron 的 分、时 字段（`*`、`*/N`、单个数字）
//   - 日/月 必须为 `*`（或 `?`），周可为 `*` 或单个 0-7（0/7 = 周日）
//
// 返回 false 表示表达式不受支持/非法。
func nextRotation(cron string, after time.Time) (time.Time, bool) {
	return cronutil.NextRotation(cron, after)
}
