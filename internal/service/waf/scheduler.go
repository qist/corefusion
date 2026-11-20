package waf

import (
	"context"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

// wafLastSyncSettingKey 记录上一次同步**尝试**的时间（成功或失败都记）。
// 为什么要持久化：原来调度器是纯内存的「启动时刻 + 间隔」，面板每次重启
// 计时器都会重置 —— 演示环境一天重启几十次，配了 @every 24h 就**永远
// 不会触发**（线上实测：11-19 起多次重启，日志里只有 scheduled，没有 start）。
const wafLastSyncSettingKey = "waf.github_source.last_sync"

// Scheduler 驱动 WAF GitHub 规则源的 sync_cron 定时同步。
//
// 支持两种语义：
//   - @every 1h / @every 24h：距**上次同步**的间隔（已持久化，重启不重置）
//   - 0 3 * * *、0 */6 * * *：按**本地时区**的墙上时间对齐（3:00 / 每 6 小时整点）
//
// 若发现上次同步已超过一个周期（面板停机、频繁重启错过窗口），启动后
// 短暂延迟即补一次，之后才按墙上时间排下一次。
type Scheduler struct {
	svc *Service

	mu      sync.Mutex
	timer   *time.Timer
	current string
	spec    scheduleSpec

	syncMu      sync.Mutex // 防同步重入（下载+推节点可能耗时）
	lastRunAt   time.Time
	lastRunOK   bool
	lastRunInfo string

	// now / catchUpDelay 可在测试中替换，避免真的等 15 秒或依赖真实时钟。
	now          func() time.Time
	catchUpDelay time.Duration

	ctx    context.Context
	cancel context.CancelFunc
}

// scheduleSpec 是简化 cron 解析后的调度规则。
type scheduleSpec struct {
	every      time.Duration // @every / 每周
	daily      bool          // 每日固定时刻
	hour       int
	minute     int
	hourlyStep int // 每 N 小时整点
}

// interval 返回该规则的标称周期，用于「是否错过窗口」的补跑判断。
func (sp scheduleSpec) interval() time.Duration {
	switch {
	case sp.daily:
		return 24 * time.Hour
	case sp.hourlyStep > 0:
		return time.Duration(sp.hourlyStep) * time.Hour
	default:
		return sp.every
	}
}

// NewScheduler creates the WAF GitHub source sync scheduler.
func NewScheduler(svc *Service) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		svc:          svc,
		now:          time.Now,
		catchUpDelay: 15 * time.Second,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start 读取已保存的配置与上次同步时间并注册定时任务（面板启动时调用一次）。
func (s *Scheduler) Start() {
	s.loadLastRun()
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
	cron := strings.TrimSpace(cfg.SyncCron)
	spec, ok := parseSchedule(cron)
	if !ok || spec.interval() <= 0 {
		log.Printf("[WAFScheduler] invalid cron %q, scheduler idle (supported: @every 1h / 0 3 * * * / 0 */6 * * *)", cfg.SyncCron)
		s.Stop()
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == cron && s.timer != nil {
		return // 配置未变化，不重排
	}
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	s.current = cron
	s.spec = spec
	s.scheduleLocked(cron, spec)
}

// scheduleLocked 排下一次执行；每轮 fire 结束后再排下一轮（调用方必须持有 s.mu）。
func (s *Scheduler) scheduleLocked(cron string, spec scheduleSpec) {
	now := s.now()
	last := s.lastRunAt
	due := nextRun(spec, now, last, s.catchUpDelay)
	delay := due.Sub(now)
	if delay < 0 {
		delay = 0
	}

	s.timer = time.AfterFunc(delay, func() {
		s.fire(cron, spec)
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.current == cron {
			s.scheduleLocked(cron, spec)
		}
	})

	reason := ""
	if last.IsZero() || now.Sub(last) >= spec.interval() {
		reason = "（错过同步窗口，先补一次）"
	}
	// 时区直接打出来：运维需要确认「0 3 * * *」到底按哪个时区算。
	// Go 的 time.Local.String() 在 systemd 下常常只返回 "Local"，所以用
	// 实际格式化出的缩写 + 偏移（如 CST+0800）更可靠。
	log.Printf("[WAFScheduler] next sync at %s (in %s, cron=%q, tz=%s)%s",
		due.Format("2006-01-02 15:04:05 MST-0700"), delay.Round(time.Second), cron,
		due.Format("MST-0700"), reason)
}

// nextRun 计算下一次执行时间。
//
// 规则：
//  1. 从未同步过 / 距上次已超过一个周期 → now + catchUpDelay（补跑）
//  2. 每日固定时刻 → 下一个本地 HH:MM（今天没过就今天，否则明天）
//  3. 每 N 小时 → 下一个整点（小时数能被 N 整除）
//  4. @every D → 上次同步 + D（没有上次则走第 1 条）
func nextRun(sp scheduleSpec, now, last time.Time, catchUpDelay time.Duration) time.Time {
	interval := sp.interval()
	if last.IsZero() || now.Sub(last) >= interval {
		return now.Add(catchUpDelay)
	}
	switch {
	case sp.daily:
		due := time.Date(now.Year(), now.Month(), now.Day(), sp.hour, sp.minute, 0, 0, now.Location())
		if !due.After(now) {
			due = due.AddDate(0, 0, 1)
		}
		return due
	case sp.hourlyStep > 0:
		due := now.Truncate(time.Hour).Add(time.Hour)
		for i := 0; i < 24 && due.Hour()%sp.hourlyStep != 0; i++ {
			due = due.Add(time.Hour)
		}
		return due
	default:
		return last.Add(sp.every)
	}
}

// parseSchedule 解析简化 cron：
//   - @every 1h / 30m
//   - 0 * * * *（每小时）、0 */6 * * *（每 6 小时）
//   - 0 3 * * *（每天 03:00，本地时区）
//   - 0 0 * * 0（每周，按 7 天间隔）
func parseSchedule(cron string) (scheduleSpec, bool) {
	cron = strings.TrimSpace(cron)
	if strings.HasPrefix(cron, "@every ") {
		d, err := time.ParseDuration(strings.TrimSpace(cron[len("@every "):]))
		if err != nil || d <= 0 {
			return scheduleSpec{}, false
		}
		return scheduleSpec{every: d}, true
	}

	fields := strings.Fields(cron)
	if len(fields) != 5 || fields[0] != "0" {
		return scheduleSpec{}, false
	}
	hourField, dom, mon, dow := fields[1], fields[2], fields[3], fields[4]
	if dom != "*" || mon != "*" {
		return scheduleSpec{}, false
	}
	// 每周：0 0 * * 0
	if dow != "*" {
		if dow == "0" && (hourField == "0" || hourField == "*") {
			return scheduleSpec{every: 7 * 24 * time.Hour}, true
		}
		return scheduleSpec{}, false
	}
	// 每 N 小时：0 */N * * *
	if strings.HasPrefix(hourField, "*/") {
		n, err := strconv.Atoi(strings.TrimPrefix(hourField, "*/"))
		if err != nil || n <= 0 || n > 23 {
			return scheduleSpec{}, false
		}
		return scheduleSpec{hourlyStep: n}, true
	}
	// 每小时：0 * * * *
	if hourField == "*" {
		return scheduleSpec{hourlyStep: 1}, true
	}
	// 每日固定时刻：0 H * * *
	h, err := strconv.Atoi(hourField)
	if err != nil || h < 0 || h > 23 {
		return scheduleSpec{}, false
	}
	return scheduleSpec{daily: true, hour: h, minute: 0}, true
}

// fire 执行一次同步（拉规则 → 推送全节点），带重入保护。
func (s *Scheduler) fire(cron string, spec scheduleSpec) {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	// 追赶保护：若停了（Apply(空)/Stop）就不再执行
	select {
	case <-s.ctx.Done():
		return
	default:
	}

	started := s.now()
	log.Printf("[WAFScheduler] scheduled sync start (cron=%q)", cron)
	cfg, err := s.svc.GetGitHubSource(s.ctx)
	if err != nil || cfg == nil {
		s.recordRun(started, false, "no source config")
		log.Printf("[WAFScheduler] no source config, skip this round")
		return
	}
	if !cfg.Enabled || (cfg.URL == "" && len(cfg.FileMappings) == 0) {
		s.recordRun(started, false, "source disabled or empty")
		log.Printf("[WAFScheduler] source disabled or empty (url=%q, file_mappings=%d), skip this round", cfg.URL, len(cfg.FileMappings))
		return
	}
	if err := s.svc.SyncFromGitHubWithSource(s.ctx, cfg.URL, cfg.Files, cfg.FileMappings); err != nil {
		s.recordRun(started, false, err.Error())
		log.Printf("[WAFScheduler] sync from github failed: %v", err)
		return
	}
	if err := s.svc.SyncToAllNodesFromDB(s.ctx); err != nil {
		s.recordRun(started, false, err.Error())
		log.Printf("[WAFScheduler] push rules to nodes failed: %v", err)
		return
	}
	s.recordRun(started, true, "synced + pushed")
	log.Printf("[WAFScheduler] scheduled sync done: rules synced and pushed to nodes")
}

// recordRun 记录本次尝试（内存 + 持久化），失败也记：避免 GitHub 不可达时
// 每次重启都立刻重试、把启动日志刷满。
func (s *Scheduler) recordRun(at time.Time, ok bool, info string) {
	s.mu.Lock()
	s.lastRunAt = at
	s.lastRunOK = ok
	s.lastRunInfo = info
	s.mu.Unlock()

	if s.svc == nil || s.svc.settingSvc == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.svc.settingSvc.Set(ctx, wafLastSyncSettingKey, at.Format(time.RFC3339)); err != nil {
		log.Printf("[WAFScheduler] persist last sync time failed: %v", err)
	}
}

// loadLastRun 从设置里恢复上次同步时间（重启后计时不再从头开始）。
func (s *Scheduler) loadLastRun() {
	if s.svc == nil || s.svc.settingSvc == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	raw, err := s.svc.settingSvc.GetByKey(ctx, wafLastSyncSettingKey)
	if err != nil || strings.TrimSpace(raw) == "" {
		return
	}
	t, perr := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if perr != nil {
		return
	}
	s.mu.Lock()
	s.lastRunAt = t
	s.mu.Unlock()
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
