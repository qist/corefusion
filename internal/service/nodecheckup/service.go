package nodecheckup

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/model/nodecheckup"
	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/store/repository"
)

// FactsFetcher 由上层注入（NodeServerManager 同时支持本机与远程 Agent）。
type FactsFetcher interface {
	GetSystemFacts(ctx context.Context, serverID int64, req *agentpb.GetSystemFactsRequest) (*agentpb.GetSystemFactsResponse, error)
}

const cacheTTL = 10 * time.Minute

type cacheEntry struct {
	report *nodecheckup.Report
	at     time.Time
}

type Service struct {
	fetcher       FactsFetcher
	settingStore  SettingStore
	repo          *repository.NodeCheckupRepository
	listServerIDs func(ctx context.Context) ([]int64, error)

	mu      sync.Mutex
	cache   map[int64]cacheEntry
	running int32
}

// SetServerIDLister 注入节点列表（定时体检用）。
func (s *Service) SetServerIDLister(fn func(ctx context.Context) ([]int64, error)) {
	s.listServerIDs = fn
}

func NewService(fetcher FactsFetcher, store SettingStore, repo *repository.NodeCheckupRepository) *Service {
	return &Service{
		fetcher:      fetcher,
		settingStore: store,
		repo:         repo,
		cache:        make(map[int64]cacheEntry),
	}
}

// GetReport 返回节点最新体检结果；refresh=true 强制重新采集（只读）。
func (s *Service) GetReport(ctx context.Context, serverID int64, refresh bool) (*nodecheckup.Report, error) {
	if serverID <= 0 {
		return nil, fmt.Errorf("invalid server id")
	}
	if !refresh {
		s.mu.Lock()
		if e, ok := s.cache[serverID]; ok && time.Since(e.at) < cacheTTL {
			cp := *e.report
			cp.Cached = true
			s.mu.Unlock()
			return &cp, nil
		}
		s.mu.Unlock()
		if stored := s.loadStored(ctx, serverID); stored != nil {
			return stored, nil
		}
	}
	if s.fetcher == nil {
		return nil, fmt.Errorf("facts fetcher not configured")
	}
	resp, err := s.fetcher.GetSystemFacts(ctx, serverID, &agentpb.GetSystemFactsRequest{
		IncludeFirewallRules: true,
		IncludeSsh:           true,
	})
	if err != nil {
		return nil, fmt.Errorf("采集节点事实失败：%w", err)
	}
	if resp == nil || !resp.GetSuccess() {
		msg := "采集节点事实失败"
		if resp != nil && resp.GetError() != "" {
			msg = resp.GetError()
		}
		return nil, fmt.Errorf("%s", msg)
	}
	var facts nodecheckup.Facts
	if err := json.Unmarshal([]byte(resp.GetFacts()), &facts); err != nil {
		return nil, fmt.Errorf("解析节点事实失败：%w", err)
	}
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	env, summary, items := Judge(&facts, settings.Thresholds)
	report := &nodecheckup.Report{
		ServerID:    serverID,
		GeneratedAt: time.Now().Unix(),
		Env:         env,
		Summary:     summary,
		Items:       items,
	}
	s.persist(ctx, report)
	s.mu.Lock()
	s.cache[serverID] = cacheEntry{report: report, at: time.Now()}
	s.mu.Unlock()
	return report, nil
}

// loadStored 读取上一次落库的结果（面板重启/缓存过期后打开详情页不再重新采集）。
func (s *Service) loadStored(ctx context.Context, serverID int64) *nodecheckup.Report {
	if s.repo == nil {
		return nil
	}
	row, err := s.repo.Get(ctx, serverID)
	if err != nil || row == nil {
		return nil
	}
	report := &nodecheckup.Report{ServerID: row.ServerID, GeneratedAt: row.GeneratedAt, Cached: true}
	_ = json.Unmarshal([]byte(row.Env), &report.Env)
	_ = json.Unmarshal([]byte(row.Summary), &report.Summary)
	_ = json.Unmarshal([]byte(row.Items), &report.Items)
	if report.Items == nil {
		report.Items = []nodecheckup.Item{}
	}
	s.mu.Lock()
	s.cache[serverID] = cacheEntry{report: report, at: time.Now()}
	s.mu.Unlock()
	return report
}

func (s *Service) persist(ctx context.Context, report *nodecheckup.Report) {
	if s.repo == nil {
		return
	}
	env, _ := json.Marshal(report.Env)
	summary, _ := json.Marshal(report.Summary)
	items, _ := json.Marshal(report.Items)
	_ = s.repo.Upsert(ctx, &entity.NodeCheckupResult{
		ServerID:    report.ServerID,
		GeneratedAt: report.GeneratedAt,
		Env:         string(env),
		Summary:     string(summary),
		Items:       string(items),
	})
}

// SummaryRow 是列表页徽标所需的精简结果。
type SummaryRow struct {
	ServerID    int64               `json:"server_id"`
	GeneratedAt int64               `json:"generated_at"`
	Env         nodecheckup.Env     `json:"env"`
	Summary     nodecheckup.Summary `json:"summary"`
}

func (s *Service) ListSummary(ctx context.Context) ([]SummaryRow, error) {
	if s.repo == nil {
		return nil, nil
	}
	rows, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SummaryRow, 0, len(rows))
	for _, r := range rows {
		var env nodecheckup.Env
		var sum nodecheckup.Summary
		_ = json.Unmarshal([]byte(r.Env), &env)
		_ = json.Unmarshal([]byte(r.Summary), &sum)
		out = append(out, SummaryRow{ServerID: r.ServerID, GeneratedAt: r.GeneratedAt, Env: env, Summary: sum})
	}
	return out, nil
}

// StartScheduler 启动定时体检（默认关闭，开启后按 interval_hours 执行）。
// 只做只读采集；同一时刻不会并发跑两轮。
func (s *Service) StartScheduler(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		var lastRun time.Time
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				settings, err := s.GetSettings(ctx)
				if err != nil || !settings.Enabled {
					continue
				}
				interval := time.Duration(settings.IntervalHours) * time.Hour
				if interval <= 0 {
					interval = 24 * time.Hour
				}
				if !lastRun.IsZero() && time.Since(lastRun) < interval {
					continue
				}
				if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
					continue
				}
				lastRun = time.Now()
				go s.runOnce(ctx)
			}
		}
	}()
}

func (s *Service) runOnce(ctx context.Context) {
	defer atomic.StoreInt32(&s.running, 0)
	if s.listServerIDs == nil {
		return
	}
	ids, err := s.listServerIDs(ctx)
	if err != nil {
		log.Printf("[nodecheckup] list servers: %v", err)
		return
	}
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, err := s.GetReport(ctx, id, true); err != nil {
			log.Printf("[nodecheckup] server %d: %v", id, err)
		}
	}
}
