package georule

import (
	"context"
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
	for _, src := range sources {
		if src.Enabled && src.SyncCron != "" {
			s.addJob(src.ID, src.SyncCron)
		}
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
				s.svc.SyncSource(s.ctx, sourceID)
			}
		}
	}()
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
