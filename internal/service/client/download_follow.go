package client

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"corefusion/internal/domain"
	"corefusion/internal/model/entity"
)

// downloadAccount 是「某客户在某个下行入站上应有的账号」。
type downloadAccount struct {
	Target   *entity.InboundConfig
	Email    string
	UUID     string
	Password string
	Flow     string
	Method   string
}

type downloadCred struct {
	uuid, password, flow, method string
}

func (c downloadCred) key() string {
	return strings.Join([]string{c.uuid, c.password, c.flow, c.method}, "\x00")
}

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

// resolveDownloadAccount 计算客户 clientEmail 在 target 上应有的账号。
//
// 凭据以「所有引用该 target 的上行入站里、该客户的用户条目」为准：
//   - 找不到 → (nil, nil)，说明该客户不在任何引用它的上行里
//   - 找到且凭据唯一 → 返回该凭据（保证下行与上行完全对齐）
//   - 凭据不唯一 → 返回冲突错误，绝不随意挑一个（否则会把别人的凭据写进去，
//     导致另一条上行连不上、或流量记到错误的人头上）
func (s *Service) resolveDownloadAccount(ctx context.Context, target *entity.InboundConfig, clientEmail string) (*downloadAccount, error) {
	if s.inboundRepo == nil || target == nil || clientEmail == "" {
		return nil, nil
	}
	cfgs, err := s.inboundRepo.ListAll(ctx, "xray")
	if err != nil {
		return nil, err
	}
	return s.resolveDownloadAccountFrom(cfgs, target, clientEmail), nil
}

// resolveDownloadAccountFrom 是 resolveDownloadAccount 的批量版本：cfgs 由调用方
// 预取一次，避免按客户逐个全表扫描。
func (s *Service) resolveDownloadAccountFrom(cfgs []entity.InboundConfig, target *entity.InboundConfig, clientEmail string) *downloadAccount {
	if target == nil || clientEmail == "" {
		return nil
	}
	seen := make(map[string]downloadCred)
	for i := range cfgs {
		up := &cfgs[i]
		if up.ID == target.ID || up.ServerID != target.ServerID {
			continue
		}
		if domain.DownloadTargetID(up.ConfigJSON) != target.ID {
			continue
		}
		u := findInboundUserByClientEmail(up.ConfigJSON, clientEmail)
		if u == nil {
			continue
		}
		uuid, password, flow, method := inboundUserCredentials(u, up.Protocol)
		cred := downloadCred{uuid: uuid, password: password, flow: flow, method: method}
		seen[cred.key()] = cred
	}
	if len(seen) == 0 {
		return nil
	}
	if len(seen) > 1 {
		log.Printf("client: 凭据冲突：客户 %s 被多个上行入站以不同凭据引用下行入站 %s，无法对齐（请让这些上行使用同一 uuid 或只保留一个引用）", clientEmail, target.Tag)
		return nil
	}
	var cred downloadCred
	for _, c := range seen {
		cred = c
	}
	return &downloadAccount{
		Target:   target,
		Email:    generateInboundEmail(clientEmail, target.Tag),
		UUID:     cred.uuid,
		Password: cred.password,
		Flow:     cred.flow,
		Method:   cred.method,
	}
}

// applyDownloadAccount 把账号写入下行入站（存在则按上行的凭据修正，保证对齐）。
func (s *Service) applyDownloadAccount(ctx context.Context, acct *downloadAccount) error {
	if acct == nil || acct.Target == nil || s.userManager == nil {
		return nil
	}
	return s.userManager.UpsertUserInbound(ctx, acct.Target.Tag, acct.Target.Protocol,
		acct.Email, acct.UUID, acct.Password, acct.Flow, acct.Method)
}

// followDownloadTarget 为上行入站引用的下行入站补建/修正「同凭据」账号。
func (s *Service) followDownloadTarget(ctx context.Context, uplink entity.InboundConfig, baseEmail string) {
	if s.userManager == nil || s.clientRepo == nil {
		return
	}
	target := s.downloadTargetOf(ctx, uplink)
	if target == nil {
		return
	}
	client, err := s.clientRepo.GetByEmail(ctx, baseEmail)
	if err != nil || client == nil {
		// 不是订阅客户（手工账号）不复制：否则会建出一个统计归不到任何客户的账号。
		log.Printf("client: skip download account for %s: not a subscription client", baseEmail)
		return
	}
	acct, err := s.resolveDownloadAccount(ctx, target, client.Email)
	if err != nil {
		log.Printf("client: %v", err)
		return
	}
	if acct == nil {
		return
	}
	if err := s.applyDownloadAccount(ctx, acct); err != nil {
		log.Printf("client: failed to apply download account %s to inbound %s: %v", acct.Email, target.Tag, err)
	}
}

// SyncDownloadAccounts 为「已经存在的客户」补齐/修正下行入站账号。
//
// 场景：客户与订阅先建好，之后才在入站编辑器里把下行入口关联到上行。
// 只处理真实订阅客户，凭据一律以引用该下行入站的上行为准。
func (s *Service) SyncDownloadAccounts(ctx context.Context, uplinkInboundID int64) error {
	if s.inboundRepo == nil || s.userManager == nil || s.clientRepo == nil {
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

	cfgs, err := s.inboundRepo.ListAll(ctx, "xray")
	if err != nil {
		return err
	}
	synced, skipped := 0, 0
	walkInboundUsers(uplink.ConfigJSON, func(u map[string]interface{}) {
		identity := inboundUserIdentity(u)
		if identity == "" || !strings.Contains(identity, "@") {
			return
		}
		client, err := s.clientRepo.GetByEmail(ctx, domain.NormalizeClientEmail(identity))
		if err != nil || client == nil {
			// 不是订阅客户：不复制，避免出现无法归户的账号。
			skipped++
			return
		}
		acct := s.resolveDownloadAccountFrom(cfgs, target, client.Email)
		if acct == nil {
			skipped++
			return
		}
		if err := s.applyDownloadAccount(ctx, acct); err != nil {
			log.Printf("client: sync download account %s to inbound %s failed: %v", acct.Email, target.Tag, err)
			return
		}
		synced++
	})
	if synced > 0 || skipped > 0 {
		log.Printf("client: synced %d download account(s) to inbound %s (uplink %s, skipped=%d)", synced, target.Tag, uplink.Tag, skipped)
	}
	return nil
}

// PruneDownloadAccounts 在上行入站的下行引用被移除/改指向后，清理旧下行入站里
// 由该上行复制过去的账号，避免残留账号被误用或造成统计混乱。
//
// 若还有其他上行仍引用旧下行入站且包含同一客户，则保留（别人还要用）。
func (s *Service) PruneDownloadAccounts(ctx context.Context, uplinkInboundID, oldTargetID int64) error {
	if s.inboundRepo == nil || s.userManager == nil || s.clientRepo == nil || oldTargetID <= 0 {
		return nil
	}
	oldTarget, err := s.inboundRepo.GetByID(ctx, oldTargetID)
	if err != nil || oldTarget == nil {
		return nil
	}
	uplink, err := s.inboundRepo.GetByID(ctx, uplinkInboundID)
	if err != nil || uplink == nil {
		return nil
	}

	removed := 0
	walkInboundUsers(uplink.ConfigJSON, func(u map[string]interface{}) {
		identity := inboundUserIdentity(u)
		if identity == "" || !strings.Contains(identity, "@") {
			return
		}
		client, err := s.clientRepo.GetByEmail(ctx, domain.NormalizeClientEmail(identity))
		if err != nil || client == nil {
			return
		}
		if s.otherUplinkReferences(ctx, oldTargetID, uplinkInboundID, client.Email) {
			return
		}
		targetEmail := generateInboundEmail(client.Email, oldTarget.Tag)
		if err := s.userManager.RemoveUserFromInbound(ctx, oldTarget.Tag, targetEmail); err != nil {
			log.Printf("client: prune download account %s from inbound %s failed: %v", targetEmail, oldTarget.Tag, err)
			return
		}
		removed++
	})
	if removed > 0 {
		log.Printf("client: pruned %d download account(s) from inbound %s (uplink %s released it)", removed, oldTarget.Tag, uplink.Tag)
	}
	return nil
}

// otherUplinkReferences 判断除 exclude 之外是否还有上行引用 targetID 且包含该客户。
func (s *Service) otherUplinkReferences(ctx context.Context, targetID, excludeUplinkID int64, clientEmail string) bool {
	cfgs, err := s.inboundRepo.ListAll(ctx, "xray")
	if err != nil {
		return false
	}
	for i := range cfgs {
		up := &cfgs[i]
		if up.ID == excludeUplinkID {
			continue
		}
		if domain.DownloadTargetID(up.ConfigJSON) != targetID {
			continue
		}
		if findInboundUserByClientEmail(up.ConfigJSON, clientEmail) != nil {
			return true
		}
	}
	return false
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

// findInboundUserByClientEmail 按「归一后的客户邮箱」在入站用户里找条目。
// 只匹配含 "@" 的身份字段；同一客户在入站里的 email 可能带本入站后缀。
func findInboundUserByClientEmail(configJSON, clientEmail string) map[string]interface{} {
	if clientEmail == "" {
		return nil
	}
	var found map[string]interface{}
	walkInboundUsers(configJSON, func(u map[string]interface{}) {
		if found != nil {
			return
		}
		identity := inboundUserIdentity(u)
		if identity == "" || !strings.Contains(identity, "@") {
			return
		}
		if domain.NormalizeClientEmail(identity) == clientEmail {
			found = u
		}
	})
	return found
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
