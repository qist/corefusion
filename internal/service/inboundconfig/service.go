package inboundconfig

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/runtime/firewall"
	coreconfigSvc "corefusion/internal/service/coreconfig"
	"corefusion/internal/store/repository"
)

// ErrServerNamesConflict is returned when a reality inbound tries to reuse a
// serverName (SNI) that is already used by another inbound on the same machine
// (node-server). A SNI must be unique per machine, otherwise the SNI routing
// cannot decide which inbound to forward to.
var ErrServerNamesConflict = errors.New("server names conflict")

// ErrProtocolSettingsMismatch 表示 protocol 与 settings 结构对不上。
// 这类配置能让 xray 启动直接失败，**整个核心**（所有入站）一起挂掉，必须在落库前拦住。
var ErrProtocolSettingsMismatch = errors.New("protocol/settings mismatch")

// normalizeXrayInboundConfig 校验（并修复）xray 入站的 protocol 与 settings 结构。
//
// 两个真实事故：
//   - 导入/改协议后残留 shadowsocks 的 method/password，protocol 却是 vless；
//   - vless 缺 "decryption":"none"（xray 强制要求）。
//
// 前者返回 ErrProtocolSettingsMismatch，后者就地补上并返回规范化后的 JSON。
func normalizeXrayInboundConfig(coreType, protocol, configJSON string) (string, error) {
	if !strings.EqualFold(strings.TrimSpace(coreType), "xray") {
		return configJSON, nil
	}
	var root map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &root); err != nil {
		return configJSON, nil
	}
	settings, _ := root["settings"].(map[string]interface{})
	if settings == nil {
		settings = map[string]interface{}{}
		root["settings"] = settings
	}
	clients, _ := settings["clients"].([]interface{})
	p := strings.ToLower(strings.TrimSpace(protocol))
	changed := false

	switch p {
	case "vless":
		// shadowsocks 结构出现在 vless 里 → 100% 是改协议/导入残留，不能落库。
		if _, hasMethod := settings["method"]; hasMethod {
			return "", fmt.Errorf("%w: protocol=vless 但 settings 含 shadowsocks 的 method，请把协议改成 shadowsocks（或清空 settings 后重新添加客户端）", ErrProtocolSettingsMismatch)
		}
		if _, hasPwd := settings["password"]; hasPwd && len(clients) == 0 {
			return "", fmt.Errorf("%w: protocol=vless 但 settings 只有 password（shadowsocks 结构），请检查协议选择", ErrProtocolSettingsMismatch)
		}
		if _, ok := settings["decryption"]; !ok {
			settings["decryption"] = "none"
			changed = true
		}
		if err := requireClientField(clients, "id", "vless 客户端"); err != nil {
			return "", err
		}
	case "shadowsocks":
		if _, ok := settings["method"]; !ok {
			if len(clients) == 0 {
				return "", fmt.Errorf("%w: shadowsocks 入站缺少 settings.method", ErrProtocolSettingsMismatch)
			}
		}
		if _, ok := settings["password"]; !ok && len(clients) == 0 {
			return "", fmt.Errorf("%w: shadowsocks 入站缺少 settings.password（单用户）或 settings.clients（多用户）", ErrProtocolSettingsMismatch)
		}
	case "vmess":
		if err := requireClientField(clients, "id", "vmess 客户端"); err != nil {
			return "", err
		}
	case "trojan":
		if err := requireClientField(clients, "password", "trojan 客户端"); err != nil {
			return "", err
		}
	case "masque":
		if err := validateMasqueInbound(root, settings, clients); err != nil {
			return "", err
		}
	}
	if normalizeRealityXver(root) {
		changed = true
	}
	if !changed {
		return configJSON, nil
	}
	b, err := json.Marshal(root)
	if err != nil {
		return configJSON, nil
	}
	return string(b), nil
}

// normalizeRealityXver 修正 REALITY 入站的 xver。
//
// REALITY 的 xver>0 表示「连 dest/target 时先发 PROXY 协议头（v1/v2）」，用于把真实
// 客户端 IP 传给后端。只有 dest 自己能解析 PROXY 时才成立 —— 即本机 unix socket
// （面板的 /dev/shm/nginx.sock）或 127.0.0.1 / ::1 上的本地反代。
//
// dest 写成公网站点（www.apple.com:443 之类）时，服务器会拿 PROXY 头去握手，
// 目标站的 ServerHello 必然异常 → REALITY 把**所有**连接（包括已认证的）判为
// invalid，节点完全连不通，且日志只说 "target sent incorrect server hello"，
// 极难排查。这里在落库前把这种情况强制改成 0。
//
// 返回是否修改了配置。
func normalizeRealityXver(root map[string]interface{}) bool {
	ss, _ := root["streamSettings"].(map[string]interface{})
	if ss == nil {
		return false
	}
	if sec, _ := ss["security"].(string); !strings.EqualFold(strings.TrimSpace(sec), "reality") {
		return false
	}
	rs, _ := ss["realitySettings"].(map[string]interface{})
	if rs == nil {
		return false
	}
	// settings.fallbacks[].xver 是 fallback 目标自己的协议头，dest 一般是本机端口，不动。
	target, _ := rs["target"].(string)
	if strings.TrimSpace(target) == "" {
		target, _ = rs["dest"].(string)
	}
	if realityTargetAcceptsProxy(target) {
		return false
	}
	cur, ok := rs["xver"]
	if !ok || jsonNumberIsZero(cur) {
		return false
	}
	rs["xver"] = 0
	return true
}

// realityTargetAcceptsProxy 判断 REALITY 的 dest/target 是否是「能吃 PROXY 协议头」
// 的本地后端：unix socket 路径，或回环地址上的端口。
func realityTargetAcceptsProxy(target string) bool {
	t := strings.ToLower(strings.TrimSpace(target))
	if t == "" {
		return false
	}
	if strings.HasPrefix(t, "unix:") || strings.HasPrefix(t, "/") || strings.HasPrefix(t, "@") {
		return true
	}
	for _, loop := range []string{"127.0.0.1:", "localhost:", "[::1]:", "::1:"} {
		if strings.HasPrefix(t, loop) {
			return true
		}
	}
	return false
}

// jsonNumberIsZero 判断 JSON 解出来的数字是否为 0（xver 可能是 float64/int/json.Number）。
func jsonNumberIsZero(v interface{}) bool {
	switch n := v.(type) {
	case float64:
		return n == 0
	case float32:
		return n == 0
	case int:
		return n == 0
	case int64:
		return n == 0
	case json.Number:
		f, err := n.Float64()
		return err == nil && f == 0
	}
	return false
}

// NormalizeRealityXverAll 对存量入站做一次 xver 规范化（幂等）。
//
// 历史数据里 REALITY 的 dest 若写成公网站点却又留着 xver=1，该节点会完全连不通，
// 而日志只有 "target sent incorrect server hello"，很难定位。落库口已经拦了，
// 这里负责把改规则之前就存在的行修正掉。
//
// 返回被修正的入站数量。
func (s *Service) NormalizeRealityXverAll(ctx context.Context) (int, error) {
	if s.repo == nil {
		return 0, nil
	}
	configs, err := s.repo.ListAll(ctx, "xray")
	if err != nil {
		return 0, fmt.Errorf("list xray inbounds: %w", err)
	}
	fixed := 0
	for i := range configs {
		cfg := &configs[i]
		var root map[string]interface{}
		if err := json.Unmarshal([]byte(cfg.ConfigJSON), &root); err != nil {
			continue
		}
		if !normalizeRealityXver(root) {
			continue
		}
		b, err := json.Marshal(root)
		if err != nil {
			continue
		}
		cfg.ConfigJSON = string(b)
		if err := s.repo.Update(ctx, cfg); err != nil {
			log.Printf("[inbound-config] normalize xver for inbound %d (%s) failed: %v", cfg.ID, cfg.Tag, err)
			continue
		}
		log.Printf("[inbound-config] inbound %d (%s): REALITY dest is external, xver forced to 0", cfg.ID, cfg.Tag)
		fixed++
	}
	return fixed, nil
}

// requireClientField 校验每个 client 都带指定凭据字段（vless/vmess 的 id、trojan 的 password）。
// 缺字段时 xray 会以 "invalid UUID:" / "empty password" 直接启动失败 → 整个核心挂掉。
func requireClientField(clients []interface{}, field, what string) error {
	for i, raw := range clients {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		v, _ := m[field].(string)
		if strings.TrimSpace(v) != "" {
			continue
		}
		if field == "id" {
			if pwd, _ := m["password"].(string); strings.TrimSpace(pwd) != "" {
				return fmt.Errorf("%w: %s[%d] 只有 password（trojan 结构）没有 id，请检查协议选择", ErrProtocolSettingsMismatch, what, i)
			}
		}
		return fmt.Errorf("%w: %s[%d] 缺少 %s", ErrProtocolSettingsMismatch, what, i, field)
	}
	return nil
}

// inboundEmailSuffix 与 generateInboundEmail 的口径保持一致：
// inbound-vless-20296 -> vless20296；短 tag 原样返回。
func inboundEmailSuffix(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return ""
	}
	parts := strings.Split(tag, "-")
	if len(parts) >= 3 {
		return parts[1] + parts[len(parts)-1]
	}
	return tag
}

// ensureClientEmails 给没有 email 的客户端补一个稳定且唯一的 email。
//
// 为什么必须补：xray 的流量统计以 client.email 为键（stats user 名就是 email）。
// xray-ui 迁移过来的客户端通常不带 email，结果这些用户的流量既不会出现在面板，
// 也不会落到 inbound_user_traffic（只按 email 归户）。
//
// base 由凭据的 SHA256 派生（u<8hex>@cf.local），不是每次真随机：
// 重复保存/重跑迁移得到同一套 email，便于对账；再拼上 `_{tag后缀}`，与订阅的
// generateInboundEmail 同口径。已有 email 的客户端保持原样（不擅自改写存量邮箱）。
func ensureClientEmails(coreType, tag, configJSON string) string {
	if !strings.EqualFold(strings.TrimSpace(coreType), "xray") {
		return configJSON
	}
	var root map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &root); err != nil {
		return configJSON
	}
	settings, _ := root["settings"].(map[string]interface{})
	if settings == nil {
		return configJSON
	}
	suffix := inboundEmailSuffix(tag)
	used := make(map[string]bool)
	changed := false
	for _, key := range []string{"clients", "users"} {
		items, _ := settings[key].([]interface{})
		for idx, raw := range items {
			m, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if e, _ := m["email"].(string); strings.TrimSpace(e) != "" {
				used[strings.TrimSpace(e)] = true
				continue
			}
			seed := ""
			for _, f := range []string{"id", "password", "auth"} {
				if v, _ := m[f].(string); strings.TrimSpace(v) != "" {
					seed = f + ":" + strings.TrimSpace(v)
					break
				}
			}
			if seed == "" {
				// 没有凭据字段（极少见）：用入站 tag + 序号兜底，仍保证唯一。
				seed = fmt.Sprintf("%s#%d", tag, idx)
			}
			sum := sha256.Sum256([]byte(seed))
			base := fmt.Sprintf("u%x@cf.local", sum[:4])
			email := base
			if suffix != "" {
				email = base + "_" + suffix
			}
			if used[email] {
				// 同一入站里凭据重复（同一份配置复制了多个 client）：加序号区分，
				// 否则 xray stats 会把两行合成一个，流量对不上。
				email = base + "-" + strconv.Itoa(idx)
				if suffix != "" {
					email += "_" + suffix
				}
			}
			used[email] = true
			m["email"] = email
			changed = true
		}
	}
	if !changed {
		return configJSON
	}
	b, err := json.Marshal(root)
	if err != nil {
		return configJSON
	}
	return string(b)
}

// syncConfigJSONMeta 让 config_json 里的 protocol/tag 与表列保持一致。
//
// 历史原因：前端保存时会把 protocol/tag 也塞进 config_json，之后改协议只更新了表列，
// config_json 里的旧值就变成"幽灵协议"——前端回显优先用它，结果所有入站都显示 VLESS，
// 编辑保存时又把错误协议写回去（进而让 xray 起不来）。
func syncConfigJSONMeta(configJSON, protocol, tag string) string {
	var root map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &root); err != nil {
		return configJSON
	}
	changed := false
	if protocol != "" {
		if cur, _ := root["protocol"].(string); cur != protocol {
			root["protocol"] = protocol
			changed = true
		}
	}
	if tag != "" {
		if cur, _ := root["tag"].(string); cur != tag {
			root["tag"] = tag
			changed = true
		}
	}
	if !changed {
		return configJSON
	}
	b, err := json.Marshal(root)
	if err != nil {
		return configJSON
	}
	return string(b)
}

// Service handles inbound config business logic.
type Service struct {
	repo       *repository.InboundConfigRepository
	db         *gorm.DB
	serverRepo *repository.NodeServerRepository
	// traffic 存放用户级流量计数（inbound_user_traffic 表）。
	// 计数不再寄存在 config_json 里——那会让每次流量归户都整包重写配置。
	traffic *repository.InboundUserTrafficRepository
}

// SetServerRepo sets the node server repository for mirror node detection.
// When set, List queries for mirror nodes will resolve to the primary server's data.
func (s *Service) SetServerRepo(repo *repository.NodeServerRepository) {
	s.serverRepo = repo
}

// resolveMirrorSource checks if the given serverID is a mirror node.
// If it is, returns the primary (partner) server ID.
// This allows the frontend to query a mirror node's inbound configs
// and see the same data as the primary.
func (s *Service) resolveMirrorSource(ctx context.Context, serverID int64) int64 {
	if s.serverRepo == nil {
		return serverID
	}
	server, err := s.serverRepo.GetByID(ctx, serverID)
	if err != nil || server == nil {
		return serverID
	}
	if server.NodeType == entity.NodeTypeMirror && server.PartnerServerID > 0 {
		return server.PartnerServerID
	}
	return serverID
}

// NewService creates a new inbound config service.
//
// traffic 为 nil 时（单元测试等场景）流量读写自动降级为空操作，不影响配置读写。
func NewService(repo *repository.InboundConfigRepository, db *gorm.DB, traffic *repository.InboundUserTrafficRepository) *Service {
	return &Service{repo: repo, db: db, traffic: traffic}
}

// List returns all inbound configs for a core type on a specific server.
// For mirror nodes, returns the primary server's inbound configs.
func (s *Service) List(ctx context.Context, serverID int64, coreType string) ([]entity.InboundConfig, error) {
	sourceID := s.resolveMirrorSource(ctx, serverID)
	return s.repo.List(ctx, sourceID, coreType)
}

// ListAll returns all inbound configs across all servers (for traffic sync).
//
// 直查库（绕过仓储），所以必须显式解密 config_json，否则密文会被当成 JSON 解析。
func (s *Service) ListAll(ctx context.Context) ([]entity.InboundConfig, error) {
	var configs []entity.InboundConfig
	err := s.db.WithContext(ctx).Order("id").Find(&configs).Error
	if err == nil {
		repository.DecryptInboundConfigs(configs)
	}
	return configs, err
}

// GetByID returns an inbound config by ID.
func (s *Service) GetByID(ctx context.Context, id int64) (*entity.InboundConfig, error) {
	return s.repo.GetByID(ctx, id)
}

// Create creates a new inbound config.
func (s *Service) Create(ctx context.Context, serverID int64, coreType, remark, protocol, tag, listen string, port int, configJSON string, enabled bool) (*entity.InboundConfig, error) {
	if serverID == 0 {
		serverID = 1 // default to local node
	}
	protocol, tag = strings.TrimSpace(protocol), strings.TrimSpace(tag)
	configJSON, err := normalizeXrayInboundConfig(coreType, protocol, configJSON)
	if err != nil {
		return nil, err
	}
	configJSON = syncConfigJSONMeta(configJSON, protocol, tag)
	// xray 流量按 client.email 统计：xray-ui 迁移来的客户端通常没有 email，
	// 不补就永远统计不到流量（见 ensureClientEmails）。
	configJSON = ensureClientEmails(coreType, tag, configJSON)
	if snis := extractServerNames(configJSON); len(snis) > 0 {
		if err := s.checkServerNamesConflict(ctx, serverID, 0, snis); err != nil {
			return nil, err
		}
	}
	cfg := &entity.InboundConfig{
		ServerID:   serverID,
		CoreType:   coreType,
		Remark:     remark,
		Protocol:   protocol,
		Tag:        tag,
		Listen:     listen,
		Port:       port,
		ConfigJSON: configJSON,
		Enabled:    enabled,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := s.repo.Create(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Update updates an existing inbound config.
func (s *Service) Update(ctx context.Context, id int64, remark, protocol, tag, listen string, port int, configJSON string, enabled bool) (*entity.InboundConfig, error) {
	cfg, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// 只校验「本次新增」的域名：存量数据里可能存在同节点重复（历史数据、镜像/批量导入
	// 等绕过校验的路径都有），若对全量报错，这些入站连改个备注都存不下去。
	if added := addedServerNames(cfg.ConfigJSON, configJSON); len(added) > 0 {
		if err := s.checkServerNamesConflict(ctx, cfg.ServerID, id, added); err != nil {
			return nil, err
		}
	}
	protocol, tag = strings.TrimSpace(protocol), strings.TrimSpace(tag)
	configJSON, err = normalizeXrayInboundConfig(cfg.CoreType, protocol, configJSON)
	if err != nil {
		return nil, err
	}
	configJSON = syncConfigJSONMeta(configJSON, protocol, tag)
	// 同 Create：补齐缺失的 client email（已有 email 不动，幂等）。
	configJSON = ensureClientEmails(cfg.CoreType, tag, configJSON)
	cfg.Remark = remark
	cfg.Protocol = protocol
	cfg.Tag = tag
	cfg.Listen = listen
	cfg.Port = port
	cfg.ConfigJSON = configJSON
	cfg.Enabled = enabled
	cfg.UpdatedAt = time.Now()
	// ⚠ 保留名单必须在写库前算好：仓储 Update 会把 cfg.ConfigJSON **就地替换成密文**
	// （enc: 前缀）。写库后再解析必然失败 → 拿到空名单 → Prune 删光该入站的全部流量行，
	// 表现为「改一下入口配置，用户已用流量被清零」（用户反馈的大 bug）。
	keepEmails, keepOK := userEmails(cfg.ConfigJSON)
	if err := s.repo.Update(ctx, cfg); err != nil {
		return nil, err
	}
	// 用户可能在这次保存里被移除：清掉已不在配置里的流量行。
	// 解析失败时宁可不清理，也不能把存量流量当成「用户已全部删除」。
	if s.traffic != nil && keepOK {
		if err := s.traffic.Prune(ctx, cfg.ServerID, cfg.ID, keepEmails); err != nil {
			log.Printf("inboundconfig: prune traffic rows failed for %s: %v", cfg.Tag, err)
		}
	}
	return cfg, nil
}

// Delete deletes an inbound config（同时清掉该入站的流量计数行）。
func (s *Service) Delete(ctx context.Context, id int64) error {
	if cfg, err := s.repo.GetByID(ctx, id); err == nil && cfg != nil && s.traffic != nil {
		if err := s.traffic.DeleteByInbound(ctx, cfg.ID); err != nil {
			log.Printf("inboundconfig: delete traffic rows failed for %s: %v", cfg.Tag, err)
		}
	}
	return s.repo.Delete(ctx, id)
}

// extractServerNames parses the reality serverNames (SNI) from an inbound
// config JSON. It reads both the array form ("serverNames") and the
// newline/comma separated string form ("serverNamesStr"), returning a
// lower-cased, de-duplicated slice. Returns nil when the inbound is not a
// reality inbound or has no serverNames.
func extractServerNames(cfgJSON string) []string {
	var inbound struct {
		// xray：streamSettings.security=reality 时用 realitySettings.serverNames / serverNamesStr
		StreamSettings struct {
			Security string `json:"security"`
			Reality  struct {
				ServerNames    []string `json:"serverNames"`
				ServerNamesStr string   `json:"serverNamesStr"`
			} `json:"realitySettings"`
		} `json:"streamSettings"`
		// sing-box：reality 入站的 SNI 是 tls.server_name
		// （tls.reality.handshake.server 是握手转发目标，等价于 xray 的 target，不参与互斥）
		TLS struct {
			ServerName string `json:"server_name"`
			Reality    struct {
				Enabled bool `json:"enabled"`
			} `json:"reality"`
		} `json:"tls"`
	}
	if err := json.Unmarshal([]byte(cfgJSON), &inbound); err != nil {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	if strings.TrimSpace(inbound.StreamSettings.Security) == "reality" {
		for _, s := range inbound.StreamSettings.Reality.ServerNames {
			add(s)
		}
		for _, s := range strings.FieldsFunc(inbound.StreamSettings.Reality.ServerNamesStr, func(r rune) bool {
			return r == '\n' || r == '\r' || r == ',' || r == ' ' || r == '\t'
		}) {
			add(s)
		}
	}
	if inbound.TLS.Reality.Enabled {
		add(inbound.TLS.ServerName)
	}
	return out
}

// PortHopRule 一条已启用的端口跳跃规则（由入站配置推导，等价于节点上实际存在的规则）。
type PortHopRule struct {
	InboundID  int64  `json:"inbound_id"`
	PortRange  string `json:"port_range"`
	TargetPort int    `json:"target_port"`
	Interface  string `json:"interface"`
	CoreType   string `json:"core_type"`
	Tag        string `json:"tag"`
	Remark     string `json:"remark"`
}

// PortHopRules 返回某节点上所有入站已启用的端口跳跃规则，以及按 10000 宽切块算出的
// 第一个空闲段（没有则空串）。
//
// 端口段是**节点级全局资源**：同一段只能指向一个入站端口（xray / sing-box 可能同时开）。
// 前端编辑器用它提前标出「已被占用/会冲突」的预设段，避免保存时才被
// firewall.CheckConflict 拦下（用户反馈：选 20000-29999 撞上旧规则 10000-20000 才报错）。
// 口径与保存时完全一致（区间重叠 + 目标端口不同 = 冲突）。
func (s *Service) PortHopRules(ctx context.Context, serverID int64) ([]PortHopRule, string) {
	configs, err := s.repo.List(ctx, serverID, "")
	if err != nil {
		return nil, ""
	}
	var out []PortHopRule
	var specs []firewall.RuleSpec
	for _, cfg := range configs {
		if cfg.ServerID != serverID {
			continue
		}
		ph := firewall.ParsePortHopFromConfig(cfg.ConfigJSON, cfg.Port)
		if ph == nil {
			continue
		}
		out = append(out, PortHopRule{
			InboundID:  cfg.ID,
			PortRange:  ph.PortRange,
			TargetPort: ph.TargetPort,
			Interface:  ph.Interface,
			CoreType:   cfg.CoreType,
			Tag:        cfg.Tag,
			Remark:     cfg.Remark,
		})
		specs = append(specs, firewall.RuleSpec{PortRange: ph.PortRange, TargetPort: ph.TargetPort, Interface: ph.Interface})
	}
	return out, firewall.SuggestFreeRange(specs, 10000)
}

// UsedServerName 描述一个已被占用的 Server Name（SNI）及其占用方。
type UsedServerName struct {
	InboundID  int64  `json:"inbound_id"`
	ServerName string `json:"server_name"`
	CoreType   string `json:"core_type"`
	Tag        string `json:"tag"`
	Remark     string `json:"remark"`
}

// UsedServerNames 返回某节点上所有入站（xray + sing-box）已占用的 Server Names。
//
// 用途：编辑器标记「已使用」的预设/域名。编辑器自己只能看到本核心的入站，看不到同一
// 节点另一核心占用的域名（用户反馈：sing-box 用了 www.apple.com，xray 那边没标已使用）。
// 提取口径与 checkServerNamesConflict 完全一致（同一函数 extractServerNames）。
func (s *Service) UsedServerNames(ctx context.Context, serverID int64) ([]UsedServerName, error) {
	configs, err := s.repo.List(ctx, serverID, "")
	if err != nil {
		return nil, err
	}
	out := make([]UsedServerName, 0, len(configs))
	for _, cfg := range configs {
		if cfg.ServerID != serverID {
			continue // 只看本节点自身的行（镜像/主备的另一侧属于另一台机器）
		}
		for _, sn := range extractServerNames(cfg.ConfigJSON) {
			out = append(out, UsedServerName{InboundID: cfg.ID, ServerName: sn, CoreType: cfg.CoreType, Tag: cfg.Tag, Remark: cfg.Remark})
		}
	}
	return out, nil
}

// addedServerNames 返回「新配置里相对旧配置新增的 Server Names」。
func addedServerNames(oldCfg, newCfg string) []string {
	oldSet := make(map[string]struct{})
	for _, sn := range extractServerNames(oldCfg) {
		oldSet[sn] = struct{}{}
	}
	var added []string
	for _, sn := range extractServerNames(newCfg) {
		if _, ok := oldSet[sn]; !ok {
			added = append(added, sn)
		}
	}
	return added
}

// checkServerNamesConflict 校验这些 Server Names 是否已被**同一节点**的其它入站占用。
// excludeID 是正在编辑的入站 ID（新建时为 0，自身不参与比较）。
//
// 口径：同节点内互斥，跨节点允许重复（SNI 路由只在单机内需要区分；不同机器的同一
// 诱饵域名互不影响）。覆盖 xray 的 realitySettings.serverNames 与 sing-box 的
// tls.server_name（见 extractServerNames）。
func (s *Service) checkServerNamesConflict(ctx context.Context, serverID, excludeID int64, newSNIs []string) error {
	existing, err := s.repo.List(ctx, serverID, "")
	if err != nil {
		return err
	}
	newSet := make(map[string]struct{}, len(newSNIs))
	for _, sn := range newSNIs {
		newSet[sn] = struct{}{}
	}
	for _, cfg := range existing {
		if cfg.ID == excludeID {
			continue
		}
		for _, sn := range extractServerNames(cfg.ConfigJSON) {
			if _, ok := newSet[sn]; ok {
				return fmt.Errorf("%w: 域名 %q 已被本节点的入站 %q 使用", ErrServerNamesConflict, sn, cfg.Remark)
			}
		}
	}
	return nil
}

// UpdateTraffic updates traffic stats for a user by email.
// UpdateTraffic 把一份流量增量归户到入站用户，并累加订阅流量缓存。
//
// 归户范围控制（重要，此前缺这两层过滤会造成成倍虚高）：
//   - serverID > 0：只匹配该节点上的入站。远端流量事件带节点信息，必须按节点归户，
//     否则同一用户在多节点都有入站时，每个节点的流量都会加到所有节点上。
//   - tag != ""：进一步限定到具体入站。
//   - 无论命中多少个入站，订阅流量缓存对同一份增量**只累加一次**
//     （此前按“匹配到的入站数量”累加，用户存在于 N 个入站时订阅流量虚高 N 倍）。
func (s *Service) UpdateTraffic(ctx context.Context, serverID int64, tag string, email string, upDelta int64, downDelta int64) error {
	// Find inbound configs (optionally scoped to a single server)
	configs, err := s.ListAll(ctx)
	if err != nil {
		return err
	}

	attributed := false
	for _, cfg := range configs {
		if attributed {
			break // 一份增量只归户到一个入站
		}
		if serverID > 0 && cfg.ServerID != serverID {
			continue
		}
		if tag != "" && cfg.Tag != tag {
			continue
		}
		var inbound map[string]interface{}
		if err := json.Unmarshal([]byte(cfg.ConfigJSON), &inbound); err != nil {
			continue
		}

		// Find the user in this inbound's user containers.
		// xray-style configs put users in settings.users/clients/accounts;
		// sing-box style configs put users directly in the top-level users array.
		updated := false
		matchedEmail := ""
		for _, container := range userContainers(inbound) {
			for _, key := range []string{"users", "clients", "accounts"} {
				items, ok := container[key].([]interface{})
				if !ok {
					continue
				}
				for _, item := range items {
					m, ok := item.(map[string]interface{})
					if !ok {
						continue
					}
					// Match by the identity field used by each protocol/core:
					//   xray:   clients[].email / user
					//   sing-box: users[].name（vless/vmess/trojan/ss/hysteria2/tuic/anytls…）
					//             users[].username（socks/http/mixed/naive）
					// 缺 username 会导致 socks/http/mixed/naive 用户的流量
					// 「采集到了但永远不归户」（不报错、不落库）。
					itemEmail := ""
					for _, key := range []string{"email", "user", "name", "username"} {
						if v, _ := m[key].(string); v != "" {
							itemEmail = v
							break
						}
					}
					if itemEmail == email {
						// 命中：记下 config_json 里该用户的真实 email，用于落到流量表
						matchedEmail = itemEmail
						updated = true
					}
				}
			}
		}

		if updated {
			// 计数落到 inbound_user_traffic（原子 UPSERT），不再整包回写 config_json
			if s.traffic != nil {
				if err := s.traffic.AddUsage(ctx, cfg.ServerID, cfg.ID, matchedEmail, upDelta, downDelta); err != nil {
					log.Printf("trafficsync: save traffic for %s in %s failed: %v", email, cfg.Tag, err)
					continue
				}
			}
			// 订阅流量缓存只累加一次（见函数注释：按入站数量累加会成倍虚高）
			s.updateSubscriptionTraffic(ctx, email, upDelta, downDelta)
			attributed = true
		}
	}
	return nil
}

// updateSubscriptionTraffic finds the subscription associated with the given
// user email and increments its traffic cache columns.
// The email from xray stats is in the format "user@email.com_inboundTag",
// so we extract the real email by finding the last "_" and taking everything before it
// (only if the part after "_" looks like a tag, i.e. the email contains "@").
func (s *Service) updateSubscriptionTraffic(ctx context.Context, statsEmail string, upDelta, downDelta int64) {
	if s.db == nil || (upDelta == 0 && downDelta == 0) {
		return
	}

	// Extract real email from "user@email.com_tag" format
	realEmail := statsEmail
	if idx := strings.LastIndex(statsEmail, "_"); idx > 0 {
		candidate := statsEmail[:idx]
		if strings.Contains(candidate, "@") {
			realEmail = candidate
		}
	}

	total := upDelta + downDelta

	// Find client by email, then use their subscription_id
	var client entity.Client
	if err := s.db.WithContext(ctx).Where("email = ?", realEmail).First(&client).Error; err != nil {
		return
	}
	if client.SubscriptionID == nil || *client.SubscriptionID == 0 {
		return
	}

	err := s.db.WithContext(ctx).Model(&entity.Subscription{}).
		Where("id = ?", *client.SubscriptionID).
		Updates(map[string]interface{}{
			"used_bytes_cache":     gorm.Expr("used_bytes_cache + ?", total),
			"upload_bytes_cache":   gorm.Expr("upload_bytes_cache + ?", upDelta),
			"download_bytes_cache": gorm.Expr("download_bytes_cache + ?", downDelta),
		}).Error
	if err != nil {
		log.Printf("trafficsync: failed to update subscription %d traffic: %v", *client.SubscriptionID, err)
	}

	// 同步客户行的 used_bytes 与活跃时间。
	// 管理端「客户管理」列表读的是 clients.used_bytes，而流量一直只累加到
	// subscriptions.used_bytes_cache → 列表里的已用流量永远是 0（用户反馈
	// 「使用流量没有友好展示」）。这里与订阅缓存一起原子累加，两者口径一致；
	// 续费/升级不清零时两边都保留，重置时 ResetUsage 也会同时清零。
	now := time.Now()
	if err := s.db.WithContext(ctx).Model(&entity.Client{}).
		Where("id = ?", client.ID).
		Updates(map[string]interface{}{
			"used_bytes":         gorm.Expr("used_bytes + ?", total),
			"last_node_usage_at": now,
		}).Error; err != nil {
		log.Printf("trafficsync: failed to update client %d usage: %v", client.ID, err)
	}
}

// userContainers returns the containers that may hold user entries in an
// inbound config JSON. xray-style configs put users in settings.users/clients/
// accounts; sing-box style configs put users directly in the top-level users
// array. This mirrors the container logic in coreconfig.CleanInboundFields.
func userContainers(inbound map[string]interface{}) []map[string]interface{} {
	containers := []map[string]interface{}{inbound}
	if settings, ok := inbound["settings"].(map[string]interface{}); ok {
		containers = append(containers, settings)
	}
	return containers
}

// userEmailOf 返回用户条目里用于归户的身份字段（xray: email/user；
// sing-box: name/username）。口径与 UpdateTraffic 的匹配保持一致。
func userEmailOf(m map[string]interface{}) string {
	for _, key := range []string{"email", "user", "name", "username"} {
		if v, _ := m[key].(string); v != "" {
			return v
		}
	}
	return ""
}

// forEachUser 依次访问 config_json 里所有用户条目（xray 的 settings.users/clients/
// accounts 与 sing-box 的顶层 users 都覆盖）。
func forEachUser(configJSON string, fn func(m map[string]interface{})) {
	var inbound map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &inbound); err != nil {
		return
	}
	walkUsers(inbound, fn)
}

// walkUsers 是 forEachUser 的 map 版本：解析结果会被就地修改。
func walkUsers(inbound map[string]interface{}, fn func(m map[string]interface{})) {
	for _, container := range userContainers(inbound) {
		for _, key := range []string{"users", "clients", "accounts"} {
			items, ok := container[key].([]interface{})
			if !ok {
				continue
			}
			for _, item := range items {
				if m, ok := item.(map[string]interface{}); ok {
					fn(m)
				}
			}
		}
	}
}

// matchedEmails 返回该入站里匹配 email 的全部身份字段（精确匹配，或历史
// "email_tag" 前缀格式）。
func matchedEmails(configJSON, email string) []string {
	var out []string
	seen := make(map[string]bool)
	forEachUser(configJSON, func(m map[string]interface{}) {
		e := userEmailOf(m)
		if e == "" || seen[e] {
			return
		}
		if e == email || strings.HasPrefix(e, email+"_") {
			seen[e] = true
			out = append(out, e)
		}
	})
	return out
}

// userEmails 返回 config_json 里全部用户身份字段（用于删除已移除用户的流量行）。
// 第二个返回值表示解析是否成功：解析失败时调用方必须放弃清理，避免把「读不出来」
// 误当成「没有用户」而删光流量行。
func userEmails(configJSON string) ([]string, bool) {
	var inbound map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &inbound); err != nil {
		return nil, false
	}
	var out []string
	walkUsers(inbound, func(m map[string]interface{}) {
		if e := userEmailOf(m); e != "" {
			out = append(out, e)
		}
	})
	return out, true
}

// EnrichWithTraffic 把 inbound_user_traffic 的计数注回 config_json 的用户对象，
// 供 API 响应展示（前端因此零改动）。
//
// 只改内存里的响应副本、不落库——真正写库时 repository 会再次剥离这些键。
func (s *Service) EnrichWithTraffic(ctx context.Context, configs []entity.InboundConfig) error {
	if s.traffic == nil || len(configs) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(configs))
	for i := range configs {
		ids = append(ids, configs[i].ID)
	}
	rows, err := s.traffic.MapForInbounds(ctx, ids)
	if err != nil {
		return err
	}
	for i := range configs {
		list := rows[configs[i].ID]
		if len(list) == 0 {
			continue
		}
		byEmail := make(map[string]entity.InboundUserTraffic, len(list))
		for _, row := range list {
			byEmail[row.Email] = row
		}
		data := injectTraffic(configs[i].ConfigJSON, byEmail)
		configs[i].ConfigJSON = data
	}
	return nil
}

// injectTraffic 在 config_json 副本里写回 _usedUp/_usedDown（仅响应用）。
func injectTraffic(configJSON string, byEmail map[string]entity.InboundUserTraffic) string {
	var inbound map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &inbound); err != nil {
		return configJSON
	}
	changed := false
	walkUsers(inbound, func(m map[string]interface{}) {
		e := userEmailOf(m)
		if e == "" {
			return
		}
		if row, ok := byEmail[e]; ok {
			m["_usedUp"] = row.UsedUp
			m["_usedDown"] = row.UsedDown
			changed = true
		}
	})
	if !changed {
		return configJSON
	}
	data, err := json.Marshal(inbound)
	if err != nil {
		return configJSON
	}
	return string(data)
}

// ResetAllTraffic 把该节点所有入站用户的流量计数清零（inbound_user_traffic 表），
// 并重置受影响客户端的订阅流量缓存。
//
// 流量已不在 config_json 里，因此这里**不再改写任何配置**，也不会触发核心重载。
func (s *Service) ResetAllTraffic(ctx context.Context, serverID int64) (int, error) {
	resetCount := int64(0)
	if s.traffic != nil {
		n, err := s.traffic.ResetForServer(ctx, serverID)
		if err != nil {
			return 0, err
		}
		resetCount = n
	}

	// Reset subscription traffic caches for all clients on this server
	s.resetSubscriptionTrafficForServer(ctx, serverID)

	return int(resetCount), nil
}

// resetSubscriptionTrafficForServer resets the traffic cache columns
// (used_bytes_cache, upload_bytes_cache, download_bytes_cache) for all
// subscriptions that have clients associated with the given server's inbounds.
func (s *Service) resetSubscriptionTrafficForServer(ctx context.Context, serverID int64) {
	if s.db == nil {
		return
	}

	// Find all client emails from this server's inbound configs
	configs, err := s.repo.List(ctx, serverID, "")
	if err != nil {
		return
	}

	emailSet := make(map[string]bool)
	for _, cfg := range configs {
		var inbound map[string]interface{}
		if err := json.Unmarshal([]byte(cfg.ConfigJSON), &inbound); err != nil {
			continue
		}
		for _, container := range userContainers(inbound) {
			for _, key := range []string{"users", "clients", "accounts"} {
				items, ok := container[key].([]interface{})
				if !ok {
					continue
				}
				for _, item := range items {
					m, ok := item.(map[string]interface{})
					if !ok {
						continue
					}
					email, _ := m["email"].(string)
					if email == "" {
						email, _ = m["user"].(string)
					}
					if email == "" {
						email, _ = m["name"].(string)
					}
					if email != "" {
						// Extract real email from "user@email.com_tag" format
						realEmail := email
						if idx := strings.LastIndex(email, "_"); idx > 0 {
							candidate := email[:idx]
							if strings.Contains(candidate, "@") {
								realEmail = candidate
							}
						}
						emailSet[realEmail] = true
					}
				}
			}
		}
	}

	if len(emailSet) == 0 {
		return
	}

	// Collect unique subscription IDs for these clients
	emails := make([]string, 0, len(emailSet))
	for email := range emailSet {
		emails = append(emails, email)
	}

	var subIDs []int64
	s.db.WithContext(ctx).Model(&entity.Client{}).
		Where("email IN ?", emails).
		Where("subscription_id IS NOT NULL AND subscription_id > 0").
		Distinct("subscription_id").
		Pluck("subscription_id", &subIDs)

	if len(subIDs) == 0 {
		return
	}

	// Reset traffic cache for these subscriptions
	s.db.WithContext(ctx).Model(&entity.Subscription{}).
		Where("id IN ?", subIDs).
		Updates(map[string]interface{}{
			"used_bytes_cache":     0,
			"upload_bytes_cache":   0,
			"download_bytes_cache": 0,
		})

	// 客户列表的已用流量（clients.used_bytes）同口径清零，否则节点重置后
	// 「客户管理」仍显示旧值。
	s.db.WithContext(ctx).Model(&entity.Client{}).
		Where("subscription_id IN ?", subIDs).
		Update("used_bytes", 0)

	log.Printf("resetTraffic: reset traffic cache for %d subscriptions on server %d", len(subIDs), serverID)
}

// ResetUserTrafficByEmail 把某 email 的用户在**所有入站**上的流量计数清零
// （inbound_user_traffic 表），返回受影响的节点 ID（供调用方触发同步）。
//
// 匹配语义与历史一致：精确 email，或 "email_tag" 前缀格式。
func (s *Service) ResetUserTrafficByEmail(ctx context.Context, email string) ([]int64, error) {
	configs, err := s.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	targets := make([]repository.ResetTarget, 0, len(configs))
	var affectedServers []int64
	for _, cfg := range configs {
		matched := matchedEmails(cfg.ConfigJSON, email)
		if len(matched) == 0 {
			continue
		}
		targets = append(targets, repository.ResetTarget{
			ServerID:  cfg.ServerID,
			InboundID: cfg.ID,
			Emails:    matched,
		})
		affectedServers = append(affectedServers, cfg.ServerID)
	}

	if s.traffic != nil && len(targets) > 0 {
		if _, err := s.traffic.ResetTargets(ctx, targets); err != nil {
			log.Printf("resetUserTraffic: failed to reset traffic for %s: %v", email, err)
			return affectedServers, err
		}
		log.Printf("resetUserTraffic: reset traffic for %s in %d inbound(s)", email, len(targets))
	}

	return affectedServers, nil
}

// GetEnabledInbounds returns all enabled inbound configs as parsed JSON for a specific server.
// For mirror nodes, returns the primary server's inbound configs.
func (s *Service) GetEnabledInbounds(ctx context.Context, serverID int64, coreType string) ([]json.RawMessage, error) {
	sourceID := s.resolveMirrorSource(ctx, serverID)
	configs, err := s.repo.List(ctx, sourceID, coreType)
	if err != nil {
		return nil, err
	}
	var result []json.RawMessage
	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		// Parse the config JSON (contains full inbound object with settings, streamSettings, etc.)
		var inbound map[string]interface{}
		if err := json.Unmarshal([]byte(cfg.ConfigJSON), &inbound); err != nil {
			return nil, fmt.Errorf("parse inbound %s: %w", cfg.Tag, err)
		}
		// Clean internal fields and filter paused users (shared logic)
		coreconfigSvc.CleanInboundFields(inbound, cfg.Tag, cfg.Listen, cfg.Protocol, cfg.Port, coreType)
		data, err := json.Marshal(inbound)
		if err != nil {
			return nil, fmt.Errorf("marshal inbound %s: %w", cfg.Tag, err)
		}
		result = append(result, data)
	}
	return result, nil
}

// validateMasqueInbound 校验 MASQUE 入站的关键字段。
// MASQUE 是 IP 隧道（RFC 9484），只能配 masque 传输 + TLS：
//   - settings.address 必填（最多一个 IPv4 + 一个 IPv6 前缀）
//   - 每个用户必须有 email 与 pass（HTTP Basic 认证）
//
// 缺这些字段 xray 会直接拒绝启动该入站（甚至整个核心起不来）。
func validateMasqueInbound(root, settings map[string]interface{}, clients []interface{}) error {
	users := clients
	if len(users) == 0 {
		if arr, ok := settings["users"].([]interface{}); ok {
			users = arr
		}
	}
	for i, raw := range users {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		email, _ := m["email"].(string)
		pass, _ := m["pass"].(string)
		if strings.TrimSpace(email) == "" || strings.TrimSpace(pass) == "" {
			return fmt.Errorf("%w: masque 用户[%d] 需要 email 与 pass（HTTP Basic 认证）", ErrProtocolSettingsMismatch, i)
		}
		if strings.Contains(email, ":") {
			return fmt.Errorf("%w: masque 用户[%d] 的 email 不能包含 ':'", ErrProtocolSettingsMismatch, i)
		}
	}
	if !hasNonEmptyStringArray(settings["address"]) {
		return fmt.Errorf("%w: masque 入站缺少 settings.address（至少填一个 CIDR，如 10.0.0.1/24）", ErrProtocolSettingsMismatch)
	}
	ss, _ := root["streamSettings"].(map[string]interface{})
	network, _ := ss["network"].(string)
	if !strings.EqualFold(network, "masque") {
		if method, _ := ss["method"].(string); !strings.EqualFold(method, "masque") {
			return fmt.Errorf("%w: masque 入站必须使用 masque 传输（streamSettings.network/method = masque）", ErrProtocolSettingsMismatch)
		}
	}
	security, _ := ss["security"].(string)
	if !strings.EqualFold(security, "tls") {
		return fmt.Errorf("%w: masque 入站必须使用 tls（不支持 reality 或 plaintext）", ErrProtocolSettingsMismatch)
	}
	return nil
}

// hasNonEmptyStringArray 判断值是否为「至少包含一个非空字符串」的数组。
func hasNonEmptyStringArray(v interface{}) bool {
	arr, ok := v.([]interface{})
	if !ok {
		if ss, ok := v.([]string); ok {
			for _, s := range ss {
				if strings.TrimSpace(s) != "" {
					return true
				}
			}
		}
		return false
	}
	for _, item := range arr {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			return true
		}
	}
	return false
}
