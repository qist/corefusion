package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"corefusion/internal/auth/jwt"
	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// InboundManager implements InboundUserManager using the inbound config repository
// and calls xray control API for live user management.
type InboundManager struct {
	inboundRepo *repository.InboundConfigRepository
	serverRepo  *repository.NodeServerRepository
	apiBaseURL  string
	jwtSecret   string

	// coreReloader 可选：让「不支持热加用户」的核心（sing-box 的 v2ray API 只提供
	// StatsService，没有 AlterInbound/AddUser）通过「重新生成配置 + 重启核心」生效。
	// 由 router 在 nodeServerSvc 就绪后注入。
	coreReloader func(ctx context.Context, serverID int64, coreType string) error
}

// SetCoreReloader 注入配置重载能力（远端节点走 SyncServerData，本地走重新生成 + 重启核心）。
func (m *InboundManager) SetCoreReloader(fn func(ctx context.Context, serverID int64, coreType string) error) {
	m.coreReloader = fn
}

// NewInboundManager creates a new InboundManager.
func NewInboundManager(inboundRepo *repository.InboundConfigRepository, apiPort, jwtSecret string) *InboundManager {
	return &InboundManager{
		inboundRepo: inboundRepo,
		apiBaseURL:  "http://127.0.0.1:" + apiPort,
		jwtSecret:   jwtSecret,
	}
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
	// Find ALL inbounds with this tag across all servers (local + remote)
	inbounds, err := m.inboundRepo.ListByTag(ctx, inboundTag)
	if err != nil || len(inbounds) == 0 {
		return fmt.Errorf("inbound not found: %s", inboundTag)
	}

	for i := range inbounds {
		inbound := &inbounds[i]

		// Check if user already exists in this inbound's config
		if userExistsInConfig(inbound.ConfigJSON, email) {
			continue // Skip, don't create duplicate
		}

		// Add user to config_json
		if err := m.addUserToInboundConfig(ctx, inbound, email, uuid, password, flow, method); err != nil {
			log.Printf("inbound_manager: failed to add user %s to inbound %s (serverID=%d): %v", email, inbound.Tag, inbound.ServerID, err)
			continue
		}

		// 运行时下发：xray 走 gRPC 热加；sing-box 无热加接口，改为重载核心。
		if isSingboxInbound(inbound) {
			go m.reloadCoreForInbound(inbound.ServerID, inbound.CoreType, "add user "+email)
			m.reloadMirrorsForInbound(ctx, inbound.ServerID, inbound.CoreType, "add user "+email)
			continue
		}

		// Call xray API for real-time user addition on the primary
		go m.callXrayAddUser(inbound.ServerID, inbound.Tag, email, uuid, password, flow, method)

		// Fan out to mirror nodes: directly call xray API on each mirror
		// using the primary's tag (mirror's tag is the same as primary's in new design).
		m.fanOutAddUserToMirrors(ctx, inbound.ServerID, inbound.Tag, email, uuid, password, flow, method)
	}
	return nil
}

// RemoveUserFromInbound removes a user entry from ALL inbound configs matching the tag
// (across local and remote nodes) and calls xray API for real-time user removal.
// It also fans out to mirror nodes by directly calling xray API on each mirror.
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

		// Call xray API for real-time user removal on the primary
		go m.callXrayRemoveUser(inbound.ServerID, inbound.Tag, email)

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
	go m.callXrayRemoveUser(inbound.ServerID, inbound.Tag, email)

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
	var uuid, password, flow, method string
	if userEntry != nil {
		protocol := detectProtocol(inbound.Tag)
		uuid, _ = userEntry["id"].(string)
		if uuid == "" {
			// sing-box 用的是 uuid 字段
			uuid, _ = userEntry["uuid"].(string)
		}
		password, _ = userEntry["password"].(string)
		auth, _ := userEntry["auth"].(string)
		flow, _ = userEntry["flow"].(string)
		method, _ = userEntry["method"].(string)
		if protocol == "hysteria" && auth != "" {
			password = auth
		}

		// Call xray API to add user back on primary
		go m.callXrayAddUser(inbound.ServerID, inbound.Tag, email, uuid, password, flow, method)
	}

	// Fan out to mirror nodes: add user back to their xray runtime
	m.fanOutResumeToMirrors(ctx, inbound.ServerID, inbound.Tag, email, uuid, password, flow, method)
	return nil
}

// addUserToInboundConfig adds a user entry to the specified inbound's config_json.
func (m *InboundManager) addUserToInboundConfig(ctx context.Context, inbound *entity.InboundConfig, email, uuid, password, flow, method string) error {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(inbound.ConfigJSON), &config); err != nil {
		return err
	}

	// sing-box 入站没有 settings，用户挂在顶层 users[]，字段名也与 xray 不同
	if isSingboxInbound(inbound) {
		protocol := singboxProtocolOf(config, inbound.Tag)
		user := singboxUserEntry(protocol, email, uuid, password, flow, method)
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

	protocol := detectProtocol(inbound.Tag)
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
func (m *InboundManager) fanOutAddUserToMirrors(ctx context.Context, primaryServerID int64, tag, email, uuid, password, flow, method string) {
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
		go m.callXrayAddUser(mirror.ID, tag, email, uuid, password, flow, method)
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
		go m.callXrayRemoveUser(mirror.ID, tag, email)
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
			go m.callXrayRemoveUser(mirror.ID, tag, email)
		}
	}
}

// fanOutResumeToMirrors calls xray API on all mirror nodes to resume (add back) the user.
func (m *InboundManager) fanOutResumeToMirrors(ctx context.Context, primaryServerID int64, tag, email, uuid, password, flow, method string) {
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
		go m.callXrayAddUser(mirror.ID, tag, email, uuid, password, flow, method)
	}
}

// callXrayAddUser calls xray control API to add a user on the specified server.
// For local nodes (serverID=1), it connects to the local xray gRPC.
// For remote nodes (serverID>1), it proxies through the Agent gRPC channel.
func (m *InboundManager) callXrayAddUser(serverID int64, tag, email, uuid, password, flow, method string) {
	protocol := detectProtocol(tag)
	body := fmt.Sprintf(`{"email":"%s","id":"%s","password":"%s","flow":"%s","method":"%s"}`,
		email, uuid, password, flow, method)

	// URL includes /node-servers/:id/ prefix for server routing
	url := fmt.Sprintf("%s/api/v1/node-servers/%d/xray/control/inbound/%s/user", m.apiBaseURL, serverID, tag)

	// 热加用户会与「核心正在重启」竞态：SyncData 刚触发 restart 时 xray 还没起来，
	// 第一次请求必然 500。以前只打一行日志就放弃 → 面板库有账号、节点配置没有，
	// 用户连上直接 EOF（实测）。这里重试几次；仍失败则回退到「重新生成配置 + 重启核心」，
	// 保证账号最终一定落到节点上。
	const attempts = 3
	lastStatus := 0
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		req, err := http.NewRequest("POST", url, strings.NewReader(body))
		if err != nil {
			log.Printf("xray add user: request error: %v", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		if token := m.generateInternalToken(); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			log.Printf("xray add user %s to %s (serverID=%d, %s) attempt %d/%d: %v",
				email, tag, serverID, protocol, attempt, attempts, err)
		} else {
			detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			resp.Body.Close()
			if resp.StatusCode == 200 {
				log.Printf("xray add user %s to %s (serverID=%d, %s): ok", email, tag, serverID, protocol)
				return
			}
			lastStatus = resp.StatusCode
			log.Printf("xray add user %s to %s (serverID=%d) attempt %d/%d: status %d body=%s",
				email, tag, serverID, attempt, attempts, resp.StatusCode, strings.TrimSpace(string(detail)))
		}
		if attempt < attempts {
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
		}
	}

	if m.coreReloader == nil {
		log.Printf("xray add user %s to %s (serverID=%d): giving up (status=%d err=%v)",
			email, tag, serverID, lastStatus, lastErr)
		return
	}
	// 兜底：重新生成配置并下发（远端走 SyncServerCore，本地会重启核心），
	// 用户账号随配置一起生效 —— 比让用户拿到"连不上的订阅"要好。
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	log.Printf("[inbound_manager] hot add failed (status=%d), reloading core: server=%d tag=%s", lastStatus, serverID, tag)
	if err := m.coreReloader(ctx, serverID, "xray"); err != nil {
		log.Printf("[inbound_manager] core reload fallback failed: server=%d tag=%s err=%v", serverID, tag, err)
	}
}

// callXrayRemoveUser calls xray control API to remove a user from the specified server.
// For local nodes (serverID=1), it connects to the local xray gRPC.
// For remote nodes (serverID>1), it proxies through the Agent gRPC channel.
func (m *InboundManager) callXrayRemoveUser(serverID int64, tag, email string) {
	// URL includes /node-servers/:id/ prefix for server routing
	url := fmt.Sprintf("%s/api/v1/node-servers/%d/xray/control/inbound/%s/user/%s", m.apiBaseURL, serverID, tag, email)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		log.Printf("xray remove user: request error: %v", err)
		return
	}
	if token := m.generateInternalToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("xray remove user %s from %s (serverID=%d): %v", email, tag, serverID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("xray remove user %s from %s (serverID=%d): status %d", email, tag, serverID, resp.StatusCode)
	} else {
		log.Printf("xray remove user %s from %s (serverID=%d): ok", email, tag, serverID)
	}
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

// generateInternalToken generates a JWT token for internal API calls.
func (m *InboundManager) generateInternalToken() string {
	if m.jwtSecret == "" {
		return ""
	}
	j := jwt.New(m.jwtSecret, 24*time.Hour, 7*24*time.Hour)
	token, err := j.GenerateAccessToken(0, "internal", "super_admin")
	if err != nil {
		log.Printf("xray: failed to generate internal token: %v", err)
		return ""
	}
	return token
}
