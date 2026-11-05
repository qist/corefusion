package client

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"corefusion/internal/domain"
	"corefusion/internal/model/entity"
)

// downloadTargetOf 解析上行入站引用的下行入站，并校验它能否与上行共用同一份凭据。
//
// XHTTP 的 downloadSettings 只替换 transport/security 层，协议认证仍复用上行
// outbound 的凭据（VLESS 的 uuid / trojan 的 password …）。因此下行入站必须：
//   - 同一节点、已启用、传输也是 xhttp
//   - 协议与上行一致（否则客户端拿上行的凭据在目标入站认证不过）
func (s *Service) downloadTargetOf(ctx context.Context, uplink entity.InboundConfig) *entity.InboundConfig {
	if s.inboundRepo == nil || !domain.IsXHTTPInbound(uplink.ConfigJSON) {
		return nil
	}
	targetID := domain.DownloadTargetID(uplink.ConfigJSON)
	if targetID <= 0 || targetID == uplink.ID {
		return nil
	}
	target, err := s.inboundRepo.GetByID(ctx, targetID)
	if err != nil || target == nil || !target.Enabled {
		return nil
	}
	if target.ServerID != uplink.ServerID {
		return nil
	}
	if !strings.EqualFold(target.Protocol, uplink.Protocol) {
		return nil
	}
	if !domain.IsXHTTPInbound(target.ConfigJSON) {
		return nil
	}
	return target
}

// followDownloadTarget 为上行入站引用的下行入站补建「同凭据」账号。
//
// email 用 「客户邮箱 + 下行入站后缀」：与上行账号一样按入站区分，xray 统计
// 时去掉后缀归一后仍落到同一个客户/订阅（见 domain.NormalizeClientEmail）。
func (s *Service) followDownloadTarget(ctx context.Context, uplink entity.InboundConfig, baseEmail, uuid, password, flow, method string) {
	if s.userManager == nil {
		return
	}
	target := s.downloadTargetOf(ctx, uplink)
	if target == nil {
		return
	}
	targetEmail := generateInboundEmail(baseEmail, target.Tag)
	if err := s.userManager.AddUserToInboundWithProtocol(ctx, target.Tag, target.Protocol, targetEmail, uuid, password, flow, method); err != nil {
		log.Printf("client: failed to add download account %s to inbound %s: %v", targetEmail, target.Tag, err)
	}
}

// SyncDownloadAccounts 为「已经存在的客户」补齐下行入站账号。
//
// 场景：客户与订阅先建好，之后才在入站编辑器里把下行入口关联到上行。
// 此时不会重新走创建流程，必须由保存动作触发一次补齐。
// 上游入站里的每个用户会被复制一份（同凭据）到目标入站，email 换成目标入站后缀。
func (s *Service) SyncDownloadAccounts(ctx context.Context, uplinkInboundID int64) error {
	if s.inboundRepo == nil || s.userManager == nil {
		log.Printf("client: sync download accounts skipped (repo/user manager not configured)")
		return nil
	}
	uplink, err := s.inboundRepo.GetByID(ctx, uplinkInboundID)
	if err != nil || uplink == nil || !uplink.Enabled {
		log.Printf("client: sync download accounts skipped for inbound %d (err=%v enabled=%v)", uplinkInboundID, err, uplink != nil && uplink.Enabled)
		return err
	}
	target := s.downloadTargetOf(ctx, *uplink)
	if target == nil {
		log.Printf("client: sync download accounts skipped for inbound %d: no usable download target (need same server/protocol/enabled xhttp)", uplinkInboundID)
		return nil
	}

	added := 0
	walkInboundUsers(uplink.ConfigJSON, func(u map[string]interface{}) {
		identity := inboundUserIdentity(u)
		if identity == "" || !strings.Contains(identity, "@") {
			return
		}
		baseEmail := domain.NormalizeClientEmail(identity)
		targetEmail := generateInboundEmail(baseEmail, target.Tag)
		uuid, password, flow, method := inboundUserCredentials(u, uplink.Protocol)
		if uuid == "" && password == "" {
			return
		}
		if err := s.userManager.AddUserToInboundWithProtocol(ctx, target.Tag, target.Protocol, targetEmail, uuid, password, flow, method); err != nil {
			log.Printf("client: sync download account %s to inbound %s failed: %v", targetEmail, target.Tag, err)
			return
		}
		added++
	})
	if added > 0 {
		log.Printf("client: synced %d download account(s) to inbound %s (uplink %s)", added, target.Tag, uplink.Tag)
	}
	return nil
}

// walkInboundUsers 遍历入站配置里的用户条目。
// xray 形态在 settings.users/clients/accounts；sing-box 形态在顶层 users。
func walkInboundUsers(configJSON string, fn func(map[string]interface{})) {
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return
	}
	containers := []map[string]interface{}{cfg}
	if settings, ok := cfg["settings"].(map[string]interface{}); ok {
		containers = append(containers, settings)
	}
	for _, c := range containers {
		for _, key := range []string{"users", "clients", "accounts"} {
			arr, ok := c[key].([]interface{})
			if !ok {
				continue
			}
			for _, item := range arr {
				if m, ok := item.(map[string]interface{}); ok {
					fn(m)
				}
			}
		}
	}
}

// inboundUserIdentity 返回用户条目的身份字段（xray: email/user；sing-box: name/username）。
func inboundUserIdentity(u map[string]interface{}) string {
	for _, key := range []string{"email", "user", "name", "username"} {
		if v, _ := u[key].(string); v != "" {
			return v
		}
	}
	return ""
}

// inboundUserCredentials 按协议取用户凭据，供复制到下行入站使用。
func inboundUserCredentials(u map[string]interface{}, protocol string) (uuid, password, flow, method string) {
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
