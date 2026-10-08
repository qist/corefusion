package inboundconfig

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	if fixedJSON, fixed := normalizeShadowsocks2022Keys(configJSON); len(fixed) > 0 {
		log.Printf("[inboundconfig] ss-2022 密钥长度与加密方式不匹配，已重新生成（客户端需重新导入）: %s", strings.Join(fixed, ", "))
		configJSON = fixedJSON
	}
	cfg.Remark = remark
	cfg.Protocol = protocol
	cfg.Tag = tag
	cfg.Listen = listen
	cfg.Port = port
	cfg.ConfigJSON = configJSON
	cfg.Enabled = enabled
	cfg.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, cfg); err != nil {
		return nil, err
	}
	// 用户可能在这次保存里被移除：清掉已不在配置里的流量行
	if s.traffic != nil {
		if err := s.traffic.Prune(ctx, cfg.ServerID, cfg.ID, userEmails(cfg.ConfigJSON)); err != nil {
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

// shadowsocks2022KeyLen 返回该 2022-blake3 加密方式要求的密钥字节长度。
// 这是**强制**的：sing-box / mihomo 客户端会以 `bad key length, required 16, got 32`
// 拒绝加载配置（xray 反而能接受，所以线上很隐蔽）。
func shadowsocks2022KeyLen(method string) int {
	if strings.TrimSpace(method) == "2022-blake3-aes-128-gcm" {
		return 16
	}
	return 32 // aes-256-gcm / chacha20-poly1305 / chacha8-poly1305
}

// decodedKeyLen 返回 base64 密钥解码后的字节数（非 base64 返回 -1）。
func decodedKeyLen(v string) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return -1
	}
	if b, err := base64.StdEncoding.DecodeString(v); err == nil {
		return len(b)
	}
	if b, err := base64.RawStdEncoding.DecodeString(v); err == nil {
		return len(b)
	}
	return -1
}

// randomBase64Key 生成 n 字节随机密钥（base64 标准编码）。
func randomBase64Key(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(buf)
}

// normalizeShadowsocks2022Keys 保证 ss-2022 的密钥长度与加密方式匹配。
//
// 场景：面板切换加密方式时只会重新生成**服务端**密钥，存量客户端密钥长度不变 ——
// 于是出现 aes-128-gcm 配 32 字节客户端密钥这种组合，sing-box / mihomo 客户端直接
// 拒载整份配置（xray 能接受，所以从服务端看不出问题）。长度不匹配的密钥本来就无法
// 使用，这里按当前加密方式重新生成。
//
// 返回（可能被修正的 JSON, 被重新生成密钥的标识列表）。标识为空表示无需改动。
func normalizeShadowsocks2022Keys(configJSON string) (string, []string) {
	if !strings.Contains(configJSON, "2022-blake3") {
		return configJSON, nil
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return configJSON, nil
	}
	settings, _ := cfg["settings"].(map[string]interface{})
	method := ""
	if settings != nil {
		method, _ = settings["method"].(string)
	}
	if strings.TrimSpace(method) == "" {
		method, _ = cfg["method"].(string) // sing-box：method 在顶层
	}
	if !strings.HasPrefix(strings.TrimSpace(method), "2022-blake3") {
		return configJSON, nil
	}
	want := shadowsocks2022KeyLen(method)
	fixed := make([]string, 0, 2)

	fixKey := func(holder map[string]interface{}, label string) {
		if holder == nil {
			return
		}
		raw, exists := holder["password"]
		if !exists {
			// 该位置本来就没有密钥（如 xray 的顶层、sing-box 的 settings）——不能凭空创建
			return
		}
		cur, _ := raw.(string)
		if decodedKeyLen(cur) == want {
			return
		}
		key := randomBase64Key(want)
		if key == "" {
			return
		}
		holder["password"] = key
		fixed = append(fixed, label)
	}

	// 服务端密钥（xray 在 settings.password，sing-box 在顶层 password）
	fixKey(settings, "<server>")
	fixKey(cfg, "<server>")

	// 用户/客户端密钥（xray: settings.clients[]；sing-box: 顶层 users[]）
	fixUsers := func(arr []interface{}) {
		for _, it := range arr {
			m, ok := it.(map[string]interface{})
			if !ok {
				continue
			}
			label, _ := m["email"].(string)
			if label == "" {
				label, _ = m["name"].(string)
			}
			if label == "" {
				label, _ = m["username"].(string)
			}
			fixKey(m, label)
		}
	}
	if arr, ok := cfg["users"].([]interface{}); ok {
		fixUsers(arr)
	}
	if settings != nil {
		if arr, ok := settings["clients"].([]interface{}); ok {
			fixUsers(arr)
		}
	}

	if len(fixed) == 0 {
		return configJSON, nil
	}
	out, err := json.Marshal(cfg)
	if err != nil {
		return configJSON, fixed
	}
	return string(out), fixed
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

	// Update client's last_node_usage_at to reflect real-time activity
	now := time.Now()
	if err := s.db.WithContext(ctx).Model(&entity.Client{}).
		Where("id = ?", client.ID).
		Update("last_node_usage_at", now).Error; err != nil {
		log.Printf("trafficsync: failed to update client %d last_node_usage_at: %v", client.ID, err)
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

// findUserEmail 判断 config_json 里是否存在指定用户，返回其身份字段。
func findUserEmail(configJSON, want string) (string, bool) {
	found := ""
	forEachUser(configJSON, func(m map[string]interface{}) {
		if found == "" {
			if e := userEmailOf(m); e == want {
				found = e
			}
		}
	})
	return found, found != ""
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
func userEmails(configJSON string) []string {
	var out []string
	forEachUser(configJSON, func(m map[string]interface{}) {
		if e := userEmailOf(m); e != "" {
			out = append(out, e)
		}
	})
	return out
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
