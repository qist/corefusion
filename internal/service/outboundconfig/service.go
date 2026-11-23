package outboundconfig

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// Service handles outbound config business logic.
type Service struct {
	repo *repository.OutboundConfigRepository
}

// NewService creates a new outbound config service.
func NewService(repo *repository.OutboundConfigRepository) *Service {
	return &Service{repo: repo}
}

// List returns all outbound configs for a core type on a specific server.
//
// xray 会先补齐保底出站（direct / blocked）。这两个出站在生成配置时由
// EnsureDefaultOutbounds 兜底注入，但如果节点从来没有对应的数据库行，
// 管理端「出站」列表就看不到它们（历史节点删过一次就会这样），用户会以为
// 保底出站不存在、也没法在路由里确认 tag。这里幂等补行，只补缺失的。
func (s *Service) List(ctx context.Context, serverID int64, coreType string) ([]entity.OutboundConfig, error) {
	if coreType == "xray" {
		if err := s.ensureXrayDefaultOutbounds(ctx, serverID); err != nil {
			log.Printf("[outboundconfig] ensure default outbounds for server %d failed: %v", serverID, err)
		}
	}
	return s.repo.List(ctx, serverID, coreType)
}

// xrayDefaultOutbound 是 xray 的保底出站定义。
// tag 必须与 EnsureDefaultOutbounds 里的判断保持一致（direct / blocked）。
type xrayDefaultOutbound struct {
	tag      string
	protocol string
	config   string
}

var xrayDefaultOutbounds = []xrayDefaultOutbound{
	{"direct", "freedom", `{"protocol":"freedom","tag":"direct"}`},
	{"blocked", "blackhole", `{"protocol":"blackhole","tag":"blocked"}`},
}

// ensureXrayDefaultOutbounds 幂等补齐缺失的 xray 保底出站行。
// 已存在同名 tag 的行（不论 is_default）都不动，避免覆盖用户改过的配置。
func (s *Service) ensureXrayDefaultOutbounds(ctx context.Context, serverID int64) error {
	rows, err := s.repo.List(ctx, serverID, "xray")
	if err != nil {
		return err
	}
	exists := make(map[string]bool, len(rows))
	for _, row := range rows {
		if row.Tag != "" {
			exists[row.Tag] = true
		}
	}
	// 追加到末尾，和「生成配置时兜底 append」的顺序保持一致。
	// ⚠ 不能用 sort_order=0 插到最前面：xray 的第一个出站就是路由默认出口，
	// 插前面会把默认出口从用户原本的第一个出站改成 direct（流量行为被静默改掉）。
	maxOrder, _ := s.repo.GetMaxSortOrder(ctx, serverID, "xray")
	for _, d := range xrayDefaultOutbounds {
		if exists[d.tag] {
			continue
		}
		maxOrder++
		cfg := &entity.OutboundConfig{
			ServerID:   serverID,
			CoreType:   "xray",
			Protocol:   d.protocol,
			Tag:        d.tag,
			ConfigJSON: d.config,
			Enabled:    true,
			IsDefault:  true,
			SortOrder:  maxOrder,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		if err := s.repo.Create(ctx, cfg); err != nil {
			return err
		}
		log.Printf("[outboundconfig] seeded missing default xray outbound %q for server %d", d.tag, serverID)
	}
	return nil
}

// GetByID returns an outbound config by ID.
func (s *Service) GetByID(ctx context.Context, serverID int64, id int64) (*entity.OutboundConfig, error) {
	return s.repo.GetByID(ctx, serverID, id)
}

// Create creates a new outbound config.
func (s *Service) Create(ctx context.Context, serverID int64, coreType, remark, protocol, tag string, configJSON string, enabled bool, isDefault bool) (*entity.OutboundConfig, error) {
	maxOrder, _ := s.repo.GetMaxSortOrder(ctx, serverID, coreType)
	cfg := &entity.OutboundConfig{
		ServerID:   serverID,
		CoreType:   coreType,
		Remark:     remark,
		Protocol:   protocol,
		Tag:        tag,
		ConfigJSON: configJSON,
		Enabled:    enabled,
		IsDefault:  isDefault,
		SortOrder:  maxOrder + 1,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := s.repo.Create(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Update updates an outbound config.
func (s *Service) Update(ctx context.Context, serverID int64, id int64, remark, protocol, tag string, configJSON string, enabled bool, isDefault bool) (*entity.OutboundConfig, error) {
	cfg, err := s.repo.GetByID(ctx, serverID, id)
	if err != nil {
		return nil, err
	}
	cfg.Remark = remark
	cfg.Protocol = protocol
	cfg.Tag = tag
	cfg.ConfigJSON = configJSON
	cfg.Enabled = enabled
	cfg.IsDefault = isDefault
	if err := s.repo.Update(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Delete deletes an outbound config.
func (s *Service) Delete(ctx context.Context, serverID int64, id int64) error {
	return s.repo.Delete(ctx, serverID, id)
}

// GetEnabledOutbounds returns enabled outbound configs as JSON objects for config generation.
func (s *Service) GetEnabledOutbounds(ctx context.Context, serverID int64, coreType string) ([]map[string]interface{}, error) {
	configs, err := s.repo.List(ctx, serverID, coreType)
	if err != nil {
		return nil, err
	}
	var outbounds []map[string]interface{}
	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		var ob map[string]interface{}
		if err := json.Unmarshal([]byte(cfg.ConfigJSON), &ob); err != nil {
			continue
		}
		normalizeRestrictedOutbound(cfg.Protocol, cfg.Tag, ob)
		// Set tag from DB
		ob["tag"] = cfg.Tag
		ob["protocol"] = cfg.Protocol
		outbounds = append(outbounds, ob)
	}
	return outbounds, nil
}

func normalizeRestrictedOutbound(protocol, tag string, ob map[string]interface{}) {
	if protocol != "wireguard" || rpFilterWritable() {
		return
	}

	settings, _ := ob["settings"].(map[string]interface{})
	if settings == nil {
		settings = make(map[string]interface{})
		ob["settings"] = settings
	}
	if current, ok := settings["noKernelTun"].(bool); ok && current {
		return
	}

	settings["noKernelTun"] = true
	log.Printf("outboundconfig: forced noKernelTun=true for wireguard outbound %q because rp_filter is not writable in current environment", tag)
}

func rpFilterWritable() bool {
	f, err := os.OpenFile("/proc/sys/net/ipv4/conf/all/rp_filter", os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

// GetDefaultOutbounds returns default outbound configs.
func (s *Service) GetDefaultOutbounds(ctx context.Context, serverID int64, coreType string) ([]entity.OutboundConfig, error) {
	return s.repo.GetDefault(ctx, serverID, coreType)
}

// Reorder updates the sort_order of multiple outbound configs.
// orders is a map of outbound ID → new sort_order value.
func (s *Service) Reorder(ctx context.Context, serverID int64, orders map[int64]int) error {
	return s.repo.Reorder(ctx, serverID, orders)
}
