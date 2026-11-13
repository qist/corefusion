package ratelimitsvc

import (
	"context"
	"log"
	"strings"

	"corefusion/internal/model/entity"
	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/store/repository"
	"corefusion/pkg/netutil"
)

// ObservedUser 是「核心观测到的一条在线用户」：某个订阅当前用哪些源 IP 连接。
//
// 权威来源是 xray 的 GetUsersStats（远端节点由 Agent 上报 ONLINE_USERS，
// 本地节点由面板进程内轮询）。它替代了旧的「按订阅 HTTP 请求 IP」关联：
// 订阅域名可能走代理/加速器，HTTP 层的 IP 并不是客户端连到节点时的源 IP。
type ObservedUser struct {
	Email          string
	SubscriptionID int64
	IPs            []string
}

// ObservedUsersFromProto 把 Agent 上报的 ONLINE_USERS 转成面板内部结构。
func ObservedUsersFromProto(users []*agentpb.OnlineUserEntry) []ObservedUser {
	out := make([]ObservedUser, 0, len(users))
	for _, u := range users {
		if u == nil || u.GetEmail() == "" {
			continue
		}
		ips := make([]string, 0, len(u.GetSessions()))
		for _, s := range u.GetSessions() {
			if s != nil && s.GetIp() != "" {
				ips = append(ips, s.GetIp())
			}
		}
		if len(ips) == 0 {
			continue
		}
		out = append(out, ObservedUser{
			Email:          u.GetEmail(),
			SubscriptionID: u.GetSubscriptionId(),
			IPs:            ips,
		})
	}
	return out
}

// SetObservedDeps 注入「核心观测 IP → 订阅」归户所需的只读依赖。
func (s *Service) SetObservedDeps(clientRepo *repository.ClientRepository, planTmplRepo *repository.PlanTemplateRepository) {
	s.observedClientRepo = clientRepo
	s.observedPlanTmplRepo = planTmplRepo
}

// HandleObservedUsers 把一批「核心观测到的用户/IP」落成限速关联。
//
// 只在**观测到该 IP 的那个节点**下发（serverID），绝不跨节点扩散；
// 解析不出订阅、或 IP 不是公网客户端地址时直接跳过（宁可不管，不要管错）。
func (s *Service) HandleObservedUsers(ctx context.Context, serverID int64, users []ObservedUser) {
	if s.nodeOp == nil || serverID <= 0 || len(users) == 0 {
		return
	}
	// 每个节点只查一次配置：未启用限速直接跳过（避免每个 IP 查一次库）。
	if cfg, err := s.repo.GetByServerID(ctx, serverID); err != nil || cfg == nil || !cfg.Enabled {
		return
	}

	type candidate struct {
		client *entity.Client
		ips    map[string]struct{}
	}
	bySub := make(map[int64]*candidate)

	for _, u := range users {
		subID, cl := s.resolveObservedSub(ctx, u)
		if subID <= 0 {
			continue
		}
		c := bySub[subID]
		if c == nil {
			c = &candidate{client: cl, ips: make(map[string]struct{})}
			bySub[subID] = c
		} else if c.client == nil {
			c.client = cl
		}
		for _, ip := range u.IPs {
			if netutil.IsPublicClientIP(ip) {
				c.ips[ip] = struct{}{}
			}
		}
	}

	for subID, c := range bySub {
		if len(c.ips) == 0 {
			continue
		}
		maxDown, maxUp, connLimit, deviceLimit := s.observedLimits(ctx, c.client)
		for ip := range c.ips {
			if err := s.pushIPAssociation(ctx, serverID, ip, subID, maxDown, maxUp, connLimit, deviceLimit); err != nil {
				log.Printf("[RateLimitObserved] server=%d sub=%d ip=%s association failed: %v",
					serverID, subID, ip, err)
			}
		}
	}
}

// resolveObservedSub 把一条观测记录归到订阅：优先用上报的 subscription_id，
// 否则按邮箱回查 clients（完整邮箱 / 去掉 _tag 后缀的 base 邮箱）。
func (s *Service) resolveObservedSub(ctx context.Context, u ObservedUser) (int64, *entity.Client) {
	if s.observedClientRepo == nil {
		return u.SubscriptionID, nil
	}
	if u.SubscriptionID > 0 {
		if cl, err := s.observedClientRepo.GetBySubscriptionID(ctx, u.SubscriptionID); err == nil && cl != nil {
			return u.SubscriptionID, cl
		}
		return u.SubscriptionID, nil
	}
	for _, email := range observedEmailCandidates(u.Email) {
		cl, err := s.observedClientRepo.GetByEmail(ctx, email)
		if err != nil || cl == nil {
			continue
		}
		if cl.SubscriptionID != nil && *cl.SubscriptionID > 0 {
			return *cl.SubscriptionID, cl
		}
	}
	return 0, nil
}

// observedLimits 解析该客户实际生效的限速参数：客户级覆盖 > 套餐模板。
// 0 表示不限。与订阅下发/客户端限速推送使用同一口径。
func (s *Service) observedLimits(ctx context.Context, cl *entity.Client) (maxDown, maxUp, connLimit, deviceLimit int32) {
	if cl == nil {
		return 0, 0, 0, 0
	}
	maxDown = int32(cl.SpeedLimitMbps)
	maxUp = int32(cl.SpeedLimitUpMbps)
	connLimit = int32(cl.ConcurrentLimit)
	deviceLimit = int32(cl.DeviceLimit)
	if maxDown > 0 && maxUp > 0 && connLimit > 0 && deviceLimit > 0 {
		return
	}
	if s.observedPlanTmplRepo == nil || cl.PlanID <= 0 {
		return
	}
	tmpl, err := s.observedPlanTmplRepo.GetByID(ctx, cl.PlanID)
	if err != nil || tmpl == nil {
		return
	}
	if maxDown == 0 {
		maxDown = int32(tmpl.SpeedLimitMbps)
	}
	if maxUp == 0 {
		maxUp = int32(tmpl.SpeedLimitUpMbps)
	}
	if connLimit == 0 {
		connLimit = int32(tmpl.ConcurrentLimit)
	}
	if deviceLimit == 0 {
		deviceLimit = int32(tmpl.DeviceLimit)
	}
	return
}

// observedEmailCandidates 返回可用于回查 clients 的邮箱形态。
// xray 的账号 email 形如 user@example.com_vless12345，客户表里是 user@example.com。
func observedEmailCandidates(email string) []string {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil
	}
	out := []string{email}
	if base := stripObservedEmailTag(email); base != email {
		out = append(out, base)
	}
	return out
}

// stripObservedEmailTag 去掉 _<tagSuffix>（tag 段不含 '@'）。
func stripObservedEmailTag(email string) string {
	if idx := strings.LastIndex(email, "_"); idx > 0 {
		tail := email[idx+1:]
		if !strings.Contains(tail, "@") {
			return email[:idx]
		}
	}
	return email
}
