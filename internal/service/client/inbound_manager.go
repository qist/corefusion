package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"corefusion/internal/coredata"
	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// XrayUserAPI 把用户热增删下发到运行中的 xray。
// 本地节点由 *coredata.ControlProvider 直连 xray gRPC；远端节点经 Agent 转发。
//
// 关键：不再走「面板自身的 HTTP API 回环」。面板开启 TLS 时，回环请求
// （http://127.0.0.1:<port>）会被 HTTPS 监听器以 400 拒绝，
// 导致每次热加都失败 → 回退重载核心 → xray 被频繁重启。
type XrayUserAPI interface {
	AddUser(ctx context.Context, serverID int64, tag, protocol, email, id, password, flow, method string) error
	RemoveUser(ctx context.Context, serverID int64, tag, email string) error
}

// ConfigPersister 把最新配置写盘（不重启核心）。
// 用户热加成功后立即写盘，可让周期性 configsync 不再认为「配置有变化」，
// 从而避免它 5 分钟后重启 xray。
type ConfigPersister func(ctx context.Context, serverID int64, coreType string) error

// InboundManager implements InboundUserManager using the inbound config repository
// and calls xray control API for live user management.
type InboundManager struct {
	inboundRepo *repository.InboundConfigRepository
	serverRepo  *repository.NodeServerRepository
	userAPI     XrayUserAPI

	// configPersister 可选：热加成功后写盘（本地只写盘不重启，远端经 Agent 写文件）。
	configPersister ConfigPersister

	// coreReloader 可选：让「不支持热加用户」的核心（sing-box 的 v2ray API 只提供
	// StatsService，没有 AlterInbound/AddUser）通过「重新生成配置 + 重启核心」生效。
	// 由 router 在 nodeServerSvc 就绪后注入。
	coreReloader func(ctx context.Context, serverID int64, coreType string) error

	// 写盘去抖：批量建号（一个套餐几百个客户）时只生成/下发一次配置。
	persistMu     sync.Mutex
	persistTimers map[string]*time.Timer
}

// SetCoreReloader 注入配置重载能力（远端节点走 SyncServerData，本地走重新生成 + 重启核心）。
func (m *InboundManager) SetCoreReloader(fn func(ctx context.Context, serverID int64, coreType string) error) {
	m.coreReloader = fn
}

// SetUserAPI 注入进程内用户热增删实现（通常为 *coredata.ControlProvider）。
func (m *InboundManager) SetUserAPI(api XrayUserAPI) {
	m.userAPI = api
}

// SetConfigPersister 注入「只写盘不重启」的配置落盘能力。
func (m *InboundManager) SetConfigPersister(fn ConfigPersister) {
	m.configPersister = fn
}

// NewInboundManager creates a new InboundManager.
func NewInboundManager(inboundRepo *repository.InboundConfigRepository) *InboundManager {
	return &InboundManager{inboundRepo: inboundRepo}
}

// SetServerRepo sets the node server repository for mirror node fanout.
// This is used to find mirror nodes of a primary server so that user
// add/remove operations can be directly forwarded to mirror nodes' xray
// via the panel's xray control API (using the primary's tag).
func (m *InboundManager) SetServerRepo(repo *repository.NodeServerRepository) {
	m.serverRepo = repo
}

// AddUserToInbound adds a user entry to ALL inbound configs matching the tag
// (across local and remote nodes) and calls xray API for real-time user management.
// It also fans out to mirror nodes by directly calling xray API on each mirror
// using the primary's tag (mirror's xray config uses the same tag as the primary).
func (m *InboundManager) AddUserToInbound(ctx context.Context, inboundTag, email, uuid, password, flow, method string) error {
	return m.AddUserToInboundWithProtocol(ctx, inboundTag, "", email, uuid, password, flow, method)
}

// AddUserToInboundWithProtocol 与 AddUserToInbound 相同，但协议由调用方显式指定
// （protocol 为空时按数据库 inbound_configs.protocol 解析，最后才按 tag 猜）。
//
// 上下行分离需要把上行用户的凭据原样复制到下行入站：凭据字段名随协议不同
// （vless/vmess 是 id/uuid，trojan/ss 是 password，hysteria 是 auth），
// 只靠 tag 推断在 tag 不含协议名时会写错字段 → 账号建了但连不上。
func (m *InboundManager) AddUserToInboundWithProtocol(ctx context.Context, inboundTag, protocol, email, uuid, password, flow, method string) error {
	// Find ALL inbounds with this tag across all servers (local + remote)
	inbounds, err := m.inboundRepo.ListByTag(ctx, inboundTag)
	if err != nil || len(inbounds) == 0 {
		return fmt.Errorf("inbound not found: %s", inboundTag)
	}

	for i := range inbounds {
		inbound := &inbounds[i]

		// 解析该入站的真实协议：显式传入 > 数据库 protocol > tag（tag 常常只有端口）。
		p := inboundProtocol(inbound, protocol)
		if !isSingboxInbound(inbound) && !coredata.ProtocolHasUsers(p) {
			log.Printf("inbound_manager: skip user %s on inbound %s: protocol %q has no user accounts", email, inbound.Tag, p)
			continue
		}

		// Check if user already exists in this inbound's config
		if userExistsInConfig(inbound.ConfigJSON, email) {
			continue // Skip, don't create duplicate
		}

		// Add user to config_json
		if err := m.addUserToInboundConfig(ctx, inbound, p, email, uuid, password, flow, method); err != nil {
			log.Printf("inbound_manager: failed to add user %s to inbound %s (serverID=%d): %v", email, inbound.Tag, inbound.ServerID, err)
			continue
		}

		// 运行时下发：xray 走 gRPC 热加；sing-box 无热加接口，改为重载核心。
		if isSingboxInbound(inbound) {
			go m.reloadCoreForInbound(inbound.ServerID, inbound.CoreType, "add user "+email)
			m.reloadMirrorsForInbound(ctx, inbound.ServerID, inbound.CoreType, "add user "+email)
			continue
		}

		// 主节点热加用户（进程内直连/经 Agent，协议显式传递，不靠 tag 猜）
		go m.hotAddUser(inbound.ServerID, inbound.Tag, p, email, uuid, password, flow, method)

		// Fan out to mirror nodes: directly call xray API on each mirror
		// using the primary's tag (mirror's tag is the same as primary's in new design).
		m.fanOutAddUserToMirrors(ctx, inbound.ServerID, inbound.Tag, p, email, uuid, password, flow, method)
	}
	return nil
}

// UpsertUserInbound 保证入站里存在指定账号，且凭据与传入值**完全一致**：
//   - 不存在 → 创建
//   - 已存在但凭据不同 → 按传入凭据修正（用于上下行分离时让下行账号与上行对齐）
//   - 已存在且一致 → 跳过
//
// 与 AddUserToInbound 不同，它不会因为「email 已存在」就跳过，避免下行残留旧凭据
// （旧 uuid/password 会让客户端认证失败，或把流量记到别人头上）。
func (m *InboundManager) UpsertUserInbound(ctx context.Context, inboundTag, protocol, email, uuid, password, flow, method string) error {
	inbounds, err := m.inboundRepo.ListByTag(ctx, inboundTag)
	if err != nil || len(inbounds) == 0 {
		return fmt.Errorf("inbound not found: %s", inboundTag)
	}

	for i := range inbounds {
		inbound := &inbounds[i]
		p := inboundProtocol(inbound, protocol)
		if !isSingboxInbound(inbound) && !coredata.ProtocolHasUsers(p) {
			continue
		}

		var config map[string]interface{}
		if err := json.Unmarshal([]byte(inbound.ConfigJSON), &config); err != nil {
			log.Printf("inbound_manager: upsert user %s parse inbound %s: %v", email, inbound.Tag, err)
			continue
		}

		existing := findInboundUserEntry(config, email)
		if existing == nil {
			if err := m.addUserToInboundConfig(ctx, inbound, p, email, uuid, password, flow, method); err != nil {
				log.Printf("inbound_manager: failed to add user %s to inbound %s: %v", email, inbound.Tag, err)
				continue
			}
			// addUserToInboundConfig 只写库：运行中的核心不会自动感知，
			// 必须调 xray API 热加（或 sing-box 重载核心），否则要等下次重启才生效。
			if isSingboxInbound(inbound) {
				go m.reloadCoreForInbound(inbound.ServerID, inbound.CoreType, "upsert user "+email)
				m.reloadMirrorsForInbound(ctx, inbound.ServerID, inbound.CoreType, "upsert user "+email)
				continue
			}
			go m.hotAddUser(inbound.ServerID, inbound.Tag, p, email, uuid, password, flow, method)
			m.fanOutAddUserToMirrors(ctx, inbound.ServerID, inbound.Tag, p, email, uuid, password, flow, method)
			continue
		}

		curUUID, curPassword, curFlow, curMethod := entryCredentials(existing, p)
		if curUUID == uuid && curPassword == password && curFlow == flow && curMethod == method {
			continue
		}
		applyUserCredentials(existing, p, uuid, password, flow, method)

		data, err := json.Marshal(config)
		if err != nil {
			log.Printf("inbound_manager: upsert user %s marshal inbound %s: %v", email, inbound.Tag, err)
			continue
		}
		inbound.ConfigJSON = string(data)
		if err := m.inboundRepo.Update(ctx, inbound); err != nil {
			log.Printf("inbound_manager: upsert user %s update inbound %s: %v", email, inbound.Tag, err)
			continue
		}
		if isSingboxInbound(inbound) {
			go m.reloadCoreForInbound(inbound.ServerID, inbound.CoreType, "upsert user "+email)
			m.reloadMirrorsForInbound(ctx, inbound.ServerID, inbound.CoreType, "upsert user "+email)
			continue
		}
		// xray：先移除旧凭据再添加，避免运行时保留旧 uuid（只写库不会生效）。
		m.hotRemoveUser(inbound.ServerID, inbound.Tag, email)
		go m.hotAddUser(inbound.ServerID, inbound.Tag, p, email, uuid, password, flow, method)
		m.fanOutReplaceUserToMirrors(ctx, inbound.ServerID, inbound.Tag, p, email, uuid, password, flow, method)
	}
	return nil
}

// findInboundUserEntry 在入站配置里按身份字段精确匹配用户条目。
func findInboundUserEntry(config map[string]interface{}, email string) map[string]interface{} {
	var found map[string]interface{}
	containers := []map[string]interface{}{config}
	if settings, ok := config["settings"].(map[string]interface{}); ok {
		containers = append(containers, settings)
	}
	for _, c := range containers {
		for _, key := range []string{"users", "clients", "accounts"} {
			arr, ok := c[key].([]interface{})
			if !ok {
				continue
			}
			for _, item := range arr {
				if m, ok := item.(map[string]interface{}); ok && inboundUserIdentityOf(m) == email {
					found = m
				}
			}
		}
	}
	return found
}

func inboundUserIdentityOf(u map[string]interface{}) string {
	for _, key := range []string{"email", "user", "name", "username"} {
		if v, _ := u[key].(string); v != "" {
			return v
		}
	}
	return ""
}

// entryCredentials 按协议读取用户条目里的凭据字段。
func entryCredentials(u map[string]interface{}, protocol string) (uuid, password, flow, method string) {
	uuid, _ = u["id"].(string)
	if uuid == "" {
		uuid, _ = u["uuid"].(string)
	}
	password, _ = u["password"].(string)
	flow, _ = u["flow"].(string)
	method, _ = u["method"].(string)
	switch strings.ToLower(protocol) {
	case "hysteria":
		if auth, _ := u["auth"].(string); auth != "" {
			password = auth
		}
	case "vless", "vmess":
		password = ""
	case "trojan", "shadowsocks":
		uuid = ""
	}
	return uuid, password, flow, method
}

// applyUserCredentials 按协议把凭据写进用户条目（保留 level 等其它字段）。
func applyUserCredentials(u map[string]interface{}, protocol, uuid, password, flow, method string) {
	switch strings.ToLower(protocol) {
	case "vless":
		u["id"] = uuid
		u["flow"] = flow
		if _, ok := u["level"]; !ok {
			u["level"] = 0
		}
	case "vmess":
		u["id"] = uuid
		if _, ok := u["alterId"]; !ok {
			u["alterId"] = 0
		}
		if _, ok := u["level"]; !ok {
			u["level"] = 0
		}
	case "trojan":
		u["password"] = password
		delete(u, "id")
	case "hysteria":
		u["auth"] = password
		delete(u, "password")
	case "shadowsocks":
		u["password"] = password
		if method != "" {
			u["method"] = method
		}
	default:
		u["id"] = uuid
		u["password"] = password
	}
}

// RemoveUserFromInbound removes a user entry from ALL inbound configs matching the tag
func (m *InboundManager) RemoveUserFromInbound(ctx context.Context, inboundTag, email string) error {
	// Find ALL inbounds with this tag across all servers (local + remote)
	inbounds, err := m.inboundRepo.ListByTag(ctx, inboundTag)
	if err != nil || len(inbounds) == 0 {
		return nil
	}

	for i := range inbounds {
		inbound := &inbounds[i]

		// Remove user from config_json
		if err := m.removeUserFromInboundConfig(ctx, inbound, email); err != nil {
			log.Printf("inbound_manager: failed to remove user %s from inbound %s (serverID=%d): %v", email, inbound.Tag, inbound.ServerID, err)
			continue
		}

		// 运行时下发：sing-box 需要重载核心才能移除用户
		if isSingboxInbound(inbound) {
			go m.reloadCoreForInbound(inbound.ServerID, inbound.CoreType, "remove user "+email)
			m.reloadMirrorsForInbound(ctx, inbound.ServerID, inbound.CoreType, "remove user "+email)
			continue
		}

		// 主节点热移除用户（进程内直连/经 Agent）
		go m.hotRemoveUser(inbound.ServerID, inbound.Tag, email)

		// Fan out to mirror nodes
		m.fanOutRemoveUserToMirrors(ctx, inbound.ServerID, inbound.Tag, email)
	}
	return nil
}

// PauseUser sets _paused=true and removes user from xray runtime.
// Works on both local and remote nodes, and fans out to mirror nodes.
func (m *InboundManager) PauseUser(ctx context.Context, inboundID int64, email string) error {
	inbound, err := m.inboundRepo.GetByID(ctx, inboundID)
	if err != nil {
		return nil
	}

	// Update database (_paused flag) on primary
	if err := m.setUserPaused(ctx, inboundID, email, true); err != nil {
		return err
	}

	// 运行时下发：sing-box 需要重载核心才会应用 _paused 过滤
	if isSingboxInbound(inbound) {
		go m.reloadCoreForInbound(inbound.ServerID, inbound.CoreType, "pause user "+email)
		m.reloadMirrorsForInbound(ctx, inbound.ServerID, inbound.CoreType, "pause user "+email)
		return nil
	}

	// Call xray API to remove user from running instance on primary
	go m.hotRemoveUser(inbound.ServerID, inbound.Tag, email)

	// Fan out to mirror nodes: remove user from their xray runtime
	m.fanOutPauseToMirrors(ctx, inbound.ServerID, inbound.Tag, email, true)
	return nil
}

// ResumeUser sets _paused=false and adds user back to xray runtime.
// Works on both local and remote nodes, and fans out to mirror nodes.
func (m *InboundManager) ResumeUser(ctx context.Context, inboundID int64, email string) error {
	inbound, err := m.inboundRepo.GetByID(ctx, inboundID)
	if err != nil {
		return nil
	}

	// Find user credentials from config
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(inbound.ConfigJSON), &config); err != nil {
		return err
	}
	userEntry := findUserEntry(config, email)

	// Update database (_paused flag) on primary
	if err := m.setUserPaused(ctx, inboundID, email, false); err != nil {
		return err
	}

	// 运行时下发：sing-box 没有热加接口，必须重载核心才会让用户恢复生效
	if isSingboxInbound(inbound) {
		go m.reloadCoreForInbound(inbound.ServerID, inbound.CoreType, "resume user "+email)
		m.reloadMirrorsForInbound(ctx, inbound.ServerID, inbound.CoreType, "resume user "+email)
		return nil
	}

	// Extract credentials for xray API calls
	p := inboundProtocol(inbound, "")
	if !coredata.ProtocolHasUsers(p) {
		return nil
	}
	var uuid, password, flow, method string
	if userEntry != nil {
		uuid, _ = userEntry["id"].(string)
		if uuid == "" {
			// sing-box 用的是 uuid 字段
			uuid, _ = userEntry["uuid"].(string)
		}
		password, _ = userEntry["password"].(string)
		auth, _ := userEntry["auth"].(string)
		flow, _ = userEntry["flow"].(string)
		method, _ = userEntry["method"].(string)
		if p == "hysteria" && auth != "" {
			password = auth
		}

		// Call xray API to add user back on primary
		go m.hotAddUser(inbound.ServerID, inbound.Tag, p, email, uuid, password, flow, method)
	}

	// Fan out to mirror nodes: add user back to their xray runtime
	m.fanOutResumeToMirrors(ctx, inbound.ServerID, inbound.Tag, p, email, uuid, password, flow, method)
	return nil
}

// addUserToInboundConfig adds a user entry to the specified inbound's config_json.
func (m *InboundManager) addUserToInboundConfig(ctx context.Context, inbound *entity.InboundConfig, protocol, email, uuid, password, flow, method string) error {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(inbound.ConfigJSON), &config); err != nil {
		return err
	}
	if protocol == "" {
		protocol = inbound.Protocol
	}
	if protocol == "" {
		protocol = detectProtocol(inbound.Tag)
	}

	// sing-box 入站没有 settings，用户挂在顶层 users[]，字段名也与 xray 不同
	if isSingboxInbound(inbound) {
		sbProtocol := singboxProtocolOf(config, inbound.Tag)
		user := singboxUserEntry(sbProtocol, email, uuid, password, flow, method)
		users, _ := config["users"].([]interface{})
		config["users"] = append(users, user)
		data, err := json.Marshal(config)
		if err != nil {
			return err
		}
		inbound.ConfigJSON = string(data)
		return m.inboundRepo.Update(ctx, inbound)
	}

	settings, ok := config["settings"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("no settings in config")
	}

	user := map[string]interface{}{
		"email":   email,
		"_paused": false,
	}

	switch protocol {
	case "vless":
		user["id"] = uuid
		user["flow"] = flow
		user["level"] = 0
	case "vmess":
		user["id"] = uuid
		user["alterId"] = 0
		user["level"] = 0
	case "trojan":
		user["password"] = password
		user["level"] = 0
	case "hysteria":
		user["auth"] = password
		user["level"] = 0
	case "shadowsocks":
		user["password"] = password
		user["method"] = method
		user["level"] = 0
	default:
		user["id"] = uuid
		user["password"] = password
		user["level"] = 0
	}

	for _, key := range []string{"clients", "users"} {
		if arr, ok := settings[key].([]interface{}); ok {
			settings[key] = append(arr, user)
			break
		}
	}

	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	inbound.ConfigJSON = string(data)
	return m.inboundRepo.Update(ctx, inbound)
}

// removeUserFromInboundConfig removes a user entry from the specified inbound's config_json.
func (m *InboundManager) removeUserFromInboundConfig(ctx context.Context, inbound *entity.InboundConfig, email string) error {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(inbound.ConfigJSON), &config); err != nil {
		return err
	}

	// 兼容 xray（settings.clients/users）与 sing-box（顶层 users[]）
	if !deleteUserFromConfig(config, email) {
		return nil
	}

	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	inbound.ConfigJSON = string(data)
	return m.inboundRepo.Update(ctx, inbound)
}

func (m *InboundManager) setUserPaused(ctx context.Context, inboundID int64, email string, paused bool) error {
	inbound, err := m.inboundRepo.GetByID(ctx, inboundID)
	if err != nil {
		return nil
	}

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(inbound.ConfigJSON), &config); err != nil {
		return err
	}

	// 兼容 xray（settings.clients/users）与 sing-box（顶层 users[]）
	if !setUserPausedInConfig(config, email, paused) {
		return nil
	}

	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	inbound.ConfigJSON = string(data)
	return m.inboundRepo.Update(ctx, inbound)
}

// fanOutAddUserToMirrors calls xray API on all mirror nodes of the given primary
// to add the user. Uses the primary's tag (mirror's xray config uses the same tag).
func (m *InboundManager) fanOutAddUserToMirrors(ctx context.Context, primaryServerID int64, tag, protocol, email, uuid, password, flow, method string) {
	if m.serverRepo == nil {
		return
	}
	mirrors, err := m.serverRepo.ListByPartner(ctx, primaryServerID)
	if err != nil {
		return
	}
	for _, mirror := range mirrors {
		if mirror.NodeType != entity.NodeTypeMirror {
			continue
		}
		// Use the primary's tag (mirror's tag is the same in new design)
		go m.hotAddUser(mirror.ID, tag, protocol, email, uuid, password, flow, method)
	}
}

// fanOutRemoveUserToMirrors calls xray API on all mirror nodes of the given primary
// to remove the user.
func (m *InboundManager) fanOutRemoveUserToMirrors(ctx context.Context, primaryServerID int64, tag, email string) {
	if m.serverRepo == nil {
		return
	}
	mirrors, err := m.serverRepo.ListByPartner(ctx, primaryServerID)
	if err != nil {
		return
	}
	for _, mirror := range mirrors {
		if mirror.NodeType != entity.NodeTypeMirror {
			continue
		}
		go m.hotRemoveUser(mirror.ID, tag, email)
	}
}

// fanOutPauseToMirrors calls xray API on all mirror nodes to pause (remove) the user.
func (m *InboundManager) fanOutPauseToMirrors(ctx context.Context, primaryServerID int64, tag, email string, paused bool) {
	if m.serverRepo == nil {
		return
	}
	mirrors, err := m.serverRepo.ListByPartner(ctx, primaryServerID)
	if err != nil {
		return
	}
	for _, mirror := range mirrors {
		if mirror.NodeType != entity.NodeTypeMirror {
			continue
		}
		if paused {
			go m.hotRemoveUser(mirror.ID, tag, email)
		}
	}
}

// fanOutReplaceUserToMirrors 在镜像节点上按「先移除、再添加」更新同一账号的凭据。
// 放在同一个 goroutine 里顺序执行，避免 remove/add 乱序导致账号缺失。
func (m *InboundManager) fanOutReplaceUserToMirrors(ctx context.Context, primaryServerID int64, tag, protocol, email, uuid, password, flow, method string) {
	if m.serverRepo == nil {
		return
	}
	mirrors, err := m.serverRepo.ListByPartner(ctx, primaryServerID)
	if err != nil {
		return
	}
	for _, mirror := range mirrors {
		if mirror.NodeType != entity.NodeTypeMirror {
			continue
		}
		serverID := mirror.ID
		go func() {
			m.hotRemoveUser(serverID, tag, email)
			m.hotAddUser(serverID, tag, protocol, email, uuid, password, flow, method)
		}()
	}
}

// fanOutResumeToMirrors calls xray API on all mirror nodes to resume (add back) the user.
func (m *InboundManager) fanOutResumeToMirrors(ctx context.Context, primaryServerID int64, tag, protocol, email, uuid, password, flow, method string) {
	if m.serverRepo == nil {
		return
	}
	mirrors, err := m.serverRepo.ListByPartner(ctx, primaryServerID)
	if err != nil {
		return
	}
	for _, mirror := range mirrors {
		if mirror.NodeType != entity.NodeTypeMirror {
			continue
		}
		go m.hotAddUser(mirror.ID, tag, protocol, email, uuid, password, flow, method)
	}
}

// hotAddUser 把用户热加到运行中的 xray（本地进程内直连 gRPC，远端经 Agent）。
//
// 以前这里走的是「面板自身 HTTP API 回环」：面板开了 TLS 时回环请求被 HTTPS
// 监听器 400 拒绝 → 3 次重试全失败 → 回退重载核心 → xray 被频繁重启。
// 现在直接走进程内控制层，并且协议由调用方显式传入（tag 只有端口时也能正确建号）。
func (m *InboundManager) hotAddUser(serverID int64, tag, protocol, email, uuid, password, flow, method string) {
	if m.userAPI == nil {
		log.Printf("[inbound_manager] user API not configured, cannot hot add user %s to %s (server=%d)", email, tag, serverID)
		return
	}

	// 与「核心正在重启」竞态：第一次可能连不上，重试几次。
	const attempts = 3
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err := m.userAPI.AddUser(ctx, serverID, tag, protocol, email, uuid, password, flow, method)
		cancel()
		if err == nil {
			log.Printf("xray add user %s to %s (serverID=%d, %s): ok", email, tag, serverID, protocol)
			// 立即写盘（不重启核心）：否则周期性 configsync 发现配置变化会重启 xray。
			m.scheduleConfigPersist(serverID, "xray")
			return
		}
		lastErr = err
		log.Printf("xray add user %s to %s (serverID=%d, %s) attempt %d/%d: %v",
			email, tag, serverID, protocol, attempt, attempts, err)
		if attempt < attempts {
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
		}
	}

	// 失败处理：
	//   - 本地节点：绝不自动重启核心（重启会断开本机 xray 全部在线连接，正是这次的病根）。
	//     账号会在下次核心重启/配置生效时补上，日志里给出排查方向。
	//   - 远程节点：保留原有的「重新生成配置 + 重启该节点核心」兜底 —— 节点与面板版本
	//     可能不一致（旧 Agent 不认 protocol、只按 tag 猜协议），不能让账号永远落不下去。
	if serverID != coredata.LocalServerID && m.coreReloader != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		log.Printf("[inbound_manager] hot add failed on remote server=%d tag=%s, reloading that node's core: %v", serverID, tag, lastErr)
		if err := m.coreReloader(ctx, serverID, "xray"); err != nil {
			log.Printf("[inbound_manager] core reload fallback failed: server=%d tag=%s err=%v", serverID, tag, err)
		}
		return
	}
	log.Printf("[inbound_manager] hot add user %s to %s (serverID=%d, %s) failed after %d attempts, core NOT restarted: %v (check xray api.listen / api config)",
		email, tag, serverID, protocol, attempts, lastErr)
}

// hotRemoveUser 从运行中的 xray 热移除用户（本地直连 / 远端经 Agent）。
func (m *InboundManager) hotRemoveUser(serverID int64, tag, email string) {
	if m.userAPI == nil {
		log.Printf("[inbound_manager] user API not configured, cannot hot remove user %s from %s (server=%d)", email, tag, serverID)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := m.userAPI.RemoveUser(ctx, serverID, tag, email); err != nil {
		log.Printf("xray remove user %s from %s (serverID=%d) failed, core NOT restarted: %v", email, tag, serverID, err)
		return
	}
	log.Printf("xray remove user %s from %s (serverID=%d): ok", email, tag, serverID)
	m.scheduleConfigPersist(serverID, "xray")
}

// persistDelay 是热加/热删后写盘的去抖窗口：批量建号只落一次配置。
const persistDelay = 2 * time.Second

// scheduleConfigPersist 合并短时间内的多次用户变更，只写一次盘且不重启核心。
func (m *InboundManager) scheduleConfigPersist(serverID int64, coreType string) {
	if m.configPersister == nil {
		return
	}
	key := fmt.Sprintf("%d/%s", serverID, coreType)

	m.persistMu.Lock()
	if m.persistTimers == nil {
		m.persistTimers = make(map[string]*time.Timer)
	}
	if t, ok := m.persistTimers[key]; ok {
		t.Stop()
	}
	m.persistTimers[key] = time.AfterFunc(persistDelay, func() {
		m.persistMu.Lock()
		delete(m.persistTimers, key)
		m.persistMu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		if err := m.configPersister(ctx, serverID, coreType); err != nil {
			log.Printf("inbound_manager: persist %s config for server %d failed: %v", coreType, serverID, err)
		}
	})
	m.persistMu.Unlock()
}

// reloadCoreForInbound 重新生成并下发配置、重启核心，使配置变更生效。
// sing-box 的 v2ray API 只实现 StatsService（无 AlterInbound/AddUser），
// 无法像 xray 那样热加用户，只能重启核心。
func (m *InboundManager) reloadCoreForInbound(serverID int64, coreType, reason string) {
	if m.coreReloader == nil {
		log.Printf("inbound_manager: core reloader not configured, %s on server %d (core=%s) will take effect after next config sync", reason, serverID, coreType)
		return
	}
	if coreType == "" {
		coreType = "singbox"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := m.coreReloader(ctx, serverID, coreType); err != nil {
		log.Printf("inbound_manager: reload core failed (%s, server=%d, core=%s): %v", reason, serverID, coreType, err)
		return
	}
	log.Printf("inbound_manager: core reloaded for %s (server=%d, core=%s)", reason, serverID, coreType)
}

// reloadMirrorsForInbound 让该主节点的镜像节点也重载核心（镜像共用主节点的 tag/配置）。
func (m *InboundManager) reloadMirrorsForInbound(ctx context.Context, primaryServerID int64, coreType, reason string) {
	if m.serverRepo == nil {
		return
	}
	mirrors, err := m.serverRepo.ListByPartner(ctx, primaryServerID)
	if err != nil {
		return
	}
	for _, mirror := range mirrors {
		if mirror.NodeType != entity.NodeTypeMirror {
			continue
		}
		go m.reloadCoreForInbound(mirror.ID, coreType, reason+" (mirror)")
	}
}

// inboundProtocol 解析入站的真实协议：显式传入 > 数据库 protocol > tag 推断。
// 线上新入站 tag 常为 inbound-<port>（不含协议名），只靠 tag 会解析失败。
func inboundProtocol(inbound *entity.InboundConfig, explicit string) string {
	if p := coredata.NormalizeProtocol(explicit); p != "" {
		return p
	}
	if inbound == nil {
		return ""
	}
	if p := coredata.NormalizeProtocol(inbound.Protocol); p != "" {
		return p
	}
	return coredata.InferProtocolFromTag(inbound.Tag)
}
