package logmanage

import (
	"context"
	"log"
	"sync"
	"time"

	"corefusion/internal/store/repository"
)

// Scheduler manages cron jobs for log rotation.
type Scheduler struct {
	policyRepo *repository.LogRotationPolicyRepository
	svc        *Service
	mu         sync.Mutex
	jobs       map[int64]*cronJob
	ctx        context.Context
	cancel     context.CancelFunc
}

type cronJob struct {
	ticker *time.Ticker
	done   chan struct{}
}

// NewScheduler creates a new log rotation scheduler.
func NewScheduler(policyRepo *repository.LogRotationPolicyRepository, svc *Service) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		policyRepo: policyRepo,
		svc:        svc,
		jobs:       make(map[int64]*cronJob),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start loads all enabled local policies (serverID=1) and starts their cron jobs.
// Remote node policies are handled by their respective agents and should not be
// scheduled on the local scheduler.
func (s *Scheduler) Start() {
	policies, err := s.policyRepo.ListEnabledByServerID(context.Background(), 1)
	if err != nil {
		log.Printf("logmanage scheduler: failed to load policies: %v", err)
		return
	}
	for _, p := range policies {
		if p.RotateCron != "" {
			s.addJob(p.ID, p.RotateCron)
		}
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
		job.ticker.Stop()
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
		job.ticker.Stop()
		delete(s.jobs, policyID)
	}
}

// addJob starts a new cron job for a policy.
func (s *Scheduler) addJob(policyID int64, rotateCron string) {
	interval := parseRotationCron(rotateCron)
	if interval <= 0 {
		log.Printf("logmanage scheduler: invalid cron %q for policy %d, skipping", rotateCron, policyID)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Don't duplicate
	if _, ok := s.jobs[policyID]; ok {
		return
	}

	done := make(chan struct{})
	job := &cronJob{done: done}
	s.jobs[policyID] = job

	go func() {
		log.Printf("logmanage scheduler: policy %d rotation every %v", policyID, interval)
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
				log.Printf("logmanage scheduler: rotating policy %d", policyID)
				s.svc.RotateExpiredLogs(s.ctx, policyID)
			}
		}
	}()
}

// parseRotationCron converts a simple cron expression to a time.Duration.
// Supported formats:
//   - "@every 1h", "@every 30m", "@every 6h"
//   - "0 0 * * *" -> 24h (daily midnight)
//   - "0 3 * * *" -> 24h (daily 3am)
//   - "0 */6 * * *" -> 6h
//   - Empty or invalid -> 0
func parseRotationCron(cron string) time.Duration {
	// Handle @every syntax
	if len(cron) > 7 && cron[:7] == "@every " {
		d, err := time.ParseDuration(cron[7:])
		if err == nil {
			return d
		}
	}

	// Common presets
	presets := map[string]time.Duration{
		"0 * * * *":    1 * time.Hour,
		"0 */2 * * *":  2 * time.Hour,
		"0 */6 * * *":  6 * time.Hour,
		"0 */12 * * *": 12 * time.Hour,
		"0 0 * * *":    24 * time.Hour,
		"0 3 * * *":    24 * time.Hour,
		"0 0 * * 0":    7 * 24 * time.Hour,
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
