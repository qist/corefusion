package waf

import (
	"context"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Scheduler 驱动 WAF GitHub 规则源的 sync_cron 定时同步。
//
// ⚠ 此前 sync_cron 只有前端 UI 和落库，后端**没有任何调度器消费它** ✗ ——
// 配置保存了但永远不会执行（对比 georule 包有自己的 scheduler.go ✓）。
// 这里补齐：Start() 读 waf.github_source 注册任务；保存配置后由 handler
// 调 Refresh() 立即重排；到点执行"拉规则 + 推送全节点"。
type Scheduler struct {
	svc *Service

	mu      sync.Mutex
	timer   *time.Timer
	current string // 当前生效的 cron 表达式（空 = 无任务）

	syncMu      sync.Mutex // 防同步重入（下载+推节点可能耗时）
	lastRunAt   time.Time
	lastRunOK   bool
	lastRunInfo string

	ctx    context.Context
	cancel context.CancelFunc
}

// NewScheduler creates the WAF GitHub source sync scheduler.
func NewScheduler(svc *Service) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{svc: svc, ctx: ctx, cancel: cancel}
}

// Start 读取已保存的配置并注册定时任务（面板启动时调用一次）。
func (s *Scheduler) Start() {
	cfg, err := s.svc.GetGitHubSource(context.Background())
	if err != nil || cfg == nil {
		log.Printf("[WAFScheduler] no github source config, scheduler idle")
		return
	}
	s.Apply(cfg)
}

// Apply 按最新配置重排任务（enabled=false 或 cron 为空 → 停任务）。
func (s *Scheduler) Apply(cfg *GitHubSourceConfig) {
	if cfg == nil || !cfg.Enabled || strings.TrimSpace(cfg.SyncCron) == "" {
		s.Stop()
		return
	}
	interval := parseCronInterval(strings.TrimSpace(cfg.SyncCron))
	if interval <= 0 {
		log.Printf("[WAFScheduler] invalid cron %q, scheduler idle (supported: @every / 0 */N * * * / daily presets)", cfg.SyncCron)
		s.Stop()
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == cfg.SyncCron && s.timer != nil {
		return // 配置未变化，不重排
	}
	if s.timer != nil {
		s.timer.Stop()
	}
	s.current = cfg.SyncCron
	cron := cfg.SyncCron
	s.scheduleLocked(cron, interval)
	log.Printf("[WAFScheduler] github source sync scheduled every %v (cron=%q)", interval, cron)
}

// scheduleLocked 排下一次执行；每轮 fire 结束后再排下一轮（调用方必须持有 s.mu）。
// ⚠ 旧实现把重排塞在第一次回调里，只滚两轮就停了（第三轮永远不来）。
func (s *Scheduler) scheduleLocked(cron string, interval time.Duration) {
	s.timer = time.AfterFunc(interval, func() {
		s.fire(cron, interval)
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.current == cron {
			s.scheduleLocked(cron, interval)
		}
	})
}

// fire 执行一次同步（拉规则 → 推送全节点），带重入保护。
func (s *Scheduler) fire(cron string, interval time.Duration) {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	// 追赶保护：若停了（Apply(空)/Stop）就不再执行
	select {
	case <-s.ctx.Done():
		return
	default:
	}

	log.Printf("[WAFScheduler] scheduled sync start (cron=%q)", cron)
	cfg, err := s.svc.GetGitHubSource(s.ctx)
	if err != nil || cfg == nil {
		log.Printf("[WAFScheduler] no source config, skip this round")
		return
	}
	if !cfg.Enabled || (cfg.URL == "" && len(cfg.FileMappings) == 0) {
		log.Printf("[WAFScheduler] source disabled or empty (url=%q, file_mappings=%d), skip this round", cfg.URL, len(cfg.FileMappings))
		return
	}
	if err := s.svc.SyncFromGitHubWithSource(s.ctx, cfg.URL, cfg.Files, cfg.FileMappings); err != nil {
		log.Printf("[WAFScheduler] sync from github failed: %v", err)
		s.lastRunAt, s.lastRunOK, s.lastRunInfo = time.Now(), false, err.Error()
		return
	}
	if err := s.svc.SyncToAllNodesFromDB(s.ctx); err != nil {
		log.Printf("[WAFScheduler] push rules to nodes failed: %v", err)
		s.lastRunAt, s.lastRunOK, s.lastRunInfo = time.Now(), false, err.Error()
		return
	}
	s.lastRunAt, s.lastRunOK, s.lastRunInfo = time.Now(), true, "synced + pushed"
	log.Printf("[WAFScheduler] scheduled sync done: rules synced and pushed to nodes")
}

// Status 返回调度器当前状态（前端展示用）。
func (s *Scheduler) Status() map[string]interface{} {
	s.mu.Lock()
	current := s.current
	hasTimer := s.timer != nil
	s.mu.Unlock()
	s.syncMu.Lock()
	lastRunAt, lastRunOK, lastRunInfo := s.lastRunAt, s.lastRunOK, s.lastRunInfo
	s.syncMu.Unlock()
	return map[string]interface{}{
		"active":        hasTimer && current != "",
		"cron":          current,
		"last_run_at":   lastRunAt,
		"last_run_ok":   lastRunOK,
		"last_run_info": lastRunInfo,
	}
}

// Stop 停止调度（面板关闭或配置禁用时）。
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	s.current = ""
}

// Close 释放调度器（面板关闭）。
func (s *Scheduler) Close() {
	s.cancel()
	s.Stop()
}

// parseCronInterval 把简化 cron 表达式转为轮询间隔（与 georule scheduler 同口径）。
// 支持：@every 1h、预设预设（每小时/每 N 小时/每天 0 点/3 点/每周）。
// ⚠ 这是**间隔近似**而非精确 cron —— "0 3 * * *" 按 24h 周期轮询，锚点是面板启动时刻。
func parseCronInterval(cron string) time.Duration {
	if len(cron) > 7 && cron[:7] == "@every " {
		if d, err := time.ParseDuration(cron[7:]); err == nil {
			return d
		}
	}
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
	// "0 */N * * *" → N 小时
	if len(cron) > 6 && strings.HasPrefix(cron, "0 */") {
		if n, err := strconv.Atoi(strings.SplitN(cron[4:], " ", 2)[0]); err == nil && n > 0 {
			return time.Duration(n) * time.Hour
		}
	}
	return 0
}
