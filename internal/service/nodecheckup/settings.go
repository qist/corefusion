package nodecheckup

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	keyEnabled      = "node_checkup.enabled"
	keyInterval     = "node_checkup.interval_hours"
	keyHistoryDays  = "node_checkup.history_days"
	keyThresholds   = "node_checkup.thresholds"
	defaultInterval = 24
	defaultHistory  = 180
)

// SettingStore 读取/写入 system_settings（由上层注入）。
type SettingStore interface {
	GetByKey(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

// Thresholds 是所有可配置判定阈值。范围在保存时校验，不做静默纠正。
type Thresholds struct {
	DiskWarnPct         int   `json:"disk_warn_pct"`
	DiskHighPct         int   `json:"disk_high_pct"`
	MemAvailableWarnPct int   `json:"mem_available_warn_pct"`
	CertWarnDays        int   `json:"cert_warn_days"`
	CertHighDays        int   `json:"cert_high_days"`
	ConntrackMin        int64 `json:"conntrack_min"`
	SomaxconnMin        int   `json:"somaxconn_min"`
	SSHMaxAuthTries     int   `json:"ssh_max_auth_tries"`
	SSHGraceSeconds     int   `json:"ssh_grace_seconds"`
	WAFSyncMaxHours     int   `json:"waf_sync_max_hours"`
	CDNHealthyMinPct    int   `json:"cdn_healthy_min_pct"`
}

// DefaultThresholds 与设计文档 §8.3 对齐。
func DefaultThresholds() Thresholds {
	return Thresholds{
		DiskWarnPct:         85,
		DiskHighPct:         95,
		MemAvailableWarnPct: 10,
		CertWarnDays:        14,
		CertHighDays:        3,
		ConntrackMin:        262144,
		SomaxconnMin:        32768,
		SSHMaxAuthTries:     6,
		SSHGraceSeconds:     60,
		WAFSyncMaxHours:     48,
		CDNHealthyMinPct:    50,
	}
}

type Settings struct {
	Enabled       bool       `json:"enabled"`
	IntervalHours int        `json:"interval_hours"`
	HistoryDays   int        `json:"history_days"`
	Thresholds    Thresholds `json:"thresholds"`
}

func DefaultSettings() Settings {
	return Settings{Enabled: false, IntervalHours: defaultInterval, HistoryDays: defaultHistory, Thresholds: DefaultThresholds()}
}

func (s *Service) GetSettings(ctx context.Context) (Settings, error) {
	out := DefaultSettings()
	if s.settingStore == nil {
		return out, nil
	}
	if v, err := s.settingStore.GetByKey(ctx, keyEnabled); err == nil && strings.TrimSpace(v) != "" {
		out.Enabled = strings.EqualFold(strings.TrimSpace(v), "true") || strings.TrimSpace(v) == "1"
	}
	if v, err := s.settingStore.GetByKey(ctx, keyInterval); err == nil {
		if n, perr := parseInt(v); perr == nil && n > 0 {
			out.IntervalHours = n
		}
	}
	if v, err := s.settingStore.GetByKey(ctx, keyHistoryDays); err == nil {
		if n, perr := parseInt(v); perr == nil && n > 0 {
			out.HistoryDays = n
		}
	}
	if v, err := s.settingStore.GetByKey(ctx, keyThresholds); err == nil && strings.TrimSpace(v) != "" {
		var th Thresholds
		if json.Unmarshal([]byte(v), &th) == nil {
			out.Thresholds = th
		}
	}
	return out, nil
}

// SaveSettings 校验范围与互斥后整体写入。
func (s *Service) SaveSettings(ctx context.Context, in Settings) (Settings, error) {
	if in.IntervalHours <= 0 {
		in.IntervalHours = defaultInterval
	}
	if in.IntervalHours > 168 {
		return Settings{}, fmt.Errorf("interval_hours 必须在 1~168 之间")
	}
	if in.HistoryDays <= 0 {
		in.HistoryDays = defaultHistory
	}
	th := in.Thresholds
	if th.DiskWarnPct < 1 || th.DiskWarnPct > 99 || th.DiskHighPct < 2 || th.DiskHighPct > 100 {
		return Settings{}, fmt.Errorf("磁盘阈值必须在 1~100 之间")
	}
	if th.DiskHighPct <= th.DiskWarnPct {
		return Settings{}, fmt.Errorf("disk_high_pct 必须大于 disk_warn_pct")
	}
	if th.MemAvailableWarnPct < 1 || th.MemAvailableWarnPct > 99 {
		return Settings{}, fmt.Errorf("mem_available_warn_pct 必须在 1~99 之间")
	}
	if th.CertHighDays < 0 || th.CertWarnDays <= th.CertHighDays {
		return Settings{}, fmt.Errorf("cert_warn_days 必须大于 cert_high_days")
	}
	if th.ConntrackMin <= 0 || th.SomaxconnMin <= 0 {
		return Settings{}, fmt.Errorf("conntrack_min / somaxconn_min 必须大于 0")
	}
	if th.SSHMaxAuthTries <= 0 || th.SSHGraceSeconds <= 0 {
		return Settings{}, fmt.Errorf("SSH 阈值必须大于 0")
	}
	if s.settingStore == nil {
		return in, nil
	}
	if err := s.settingStore.Set(ctx, keyEnabled, boolString(in.Enabled)); err != nil {
		return Settings{}, err
	}
	if err := s.settingStore.Set(ctx, keyInterval, fmt.Sprintf("%d", in.IntervalHours)); err != nil {
		return Settings{}, err
	}
	if err := s.settingStore.Set(ctx, keyHistoryDays, fmt.Sprintf("%d", in.HistoryDays)); err != nil {
		return Settings{}, err
	}
	data, _ := json.Marshal(th)
	if err := s.settingStore.Set(ctx, keyThresholds, string(data)); err != nil {
		return Settings{}, err
	}
	return in, nil
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func parseInt(v string) (int, error) {
	var n int
	_, err := fmt.Sscanf(strings.TrimSpace(v), "%d", &n)
	return n, err
}
