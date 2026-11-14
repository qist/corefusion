package node

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"corefusion/internal/coredata"
	"corefusion/internal/model/entity"
	agentsvc "corefusion/internal/runtime/agent"
	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/store/repository"
)

// Service handles node business logic.
type Service struct {
	repo           *repository.NodeRepository
	nodeServerRepo *repository.NodeServerRepository
	nodeOnline     nodeOnlineProvider
	subRepo        *repository.SubscriptionRepository
	clientRepo     *repository.ClientRepository
	planTmplRepo   *repository.PlanTemplateRepository
	// inboundRepo 用于把「核心账号名后缀」反查成入口（入站备注/端口）。
	// 未注入时入口字段留空，不影响其它展示。
	inboundRepo *repository.InboundConfigRepository
	// clashAPIAddrFn 解析 sing-box clash_api 地址（用户可能改过端口；
	// 未注入或解析为空时回退 coredata.ResolveClashAPIAddr）。
	clashAPIAddrFn   func() string
	clashAPISecretFn func() string
}

// nodeOnlineProvider 提供节点实时在线订阅人数据，由 runtime/agent.NodeServerManager 实现。
// 使用接口而非直接依赖 runtime/agent，避免 service/node 与 runtime/agent 形成循环依赖。
type nodeOnlineProvider interface {
	GetNodeOnlineUsers(serverID int64) ([]*agentpb.OnlineUserEntry, int64, bool)
	GetAllOnlineSummary() map[int64]*agentsvc.NodeOnlineSummary
	GetAllOnlineUsers() map[int64]*agentsvc.NodeOnlineSnapshot
}

// NodeOnlineResult 是返回给前端的节点在线订阅人聚合视图。
type NodeOnlineResult struct {
	OnlineSubscriptions int64                      `json:"online_subscriptions"`
	OnlineDevices       int64                      `json:"online_devices"`
	TotalUplinkRate     int64                      `json:"total_uplink_rate"`
	TotalDownlinkRate   int64                      `json:"total_downlink_rate"`
	UpdatedAt           int64                      `json:"updated_at"`
	FilteredTotal       int64                      `json:"filtered_total"` // 符合筛选条件的总数（分页 total）
	Users               []*agentpb.OnlineUserEntry `json:"users"`          // 本页数据
}

// NodeOnlineFilter 是单节点在线用户查询的过滤条件（服务端分页与筛选）。
type NodeOnlineFilter struct {
	Keyword string // 关键字（邮箱/客户端名/IP，大小写不敏感）
	Limit   int    // 每页条数（<=0 表示不分页，返回全部）
	Offset  int    // 偏移量
}

// NewService creates a new node service.
func NewService(repo *repository.NodeRepository, nodeOnline nodeOnlineProvider, subRepo *repository.SubscriptionRepository, clientRepo *repository.ClientRepository, planTmplRepo *repository.PlanTemplateRepository) *Service {
	return &Service{repo: repo, nodeOnline: nodeOnline, subRepo: subRepo, clientRepo: clientRepo, planTmplRepo: planTmplRepo}
}

// SetInboundRepo 注入入站仓储（在线列表显示「连接的是哪个入口」用）。
func (s *Service) SetInboundRepo(repo *repository.InboundConfigRepository) {
	s.inboundRepo = repo
}

// SetClashAPIAddr 注入 sing-box clash_api 地址解析器（读运行中的实验性配置）。
func (s *Service) SetClashAPIAddr(fn func() string) {
	s.clashAPIAddrFn = fn
}

// SetClashAPISecret 注入 clash_api 的 secret 解析器（用户可能给第三方面板配了鉴权）。
func (s *Service) SetClashAPISecret(fn func() string) {
	s.clashAPISecretFn = fn
}

// clashAPISecret 返回读取 clash_api 需要的 secret（空 = 不鉴权）。
func (s *Service) clashAPISecret() string {
	if s.clashAPISecretFn != nil {
		return strings.TrimSpace(s.clashAPISecretFn())
	}
	return ""
}

// clashAPIAddr 返回要读取的 clash_api 地址。
func (s *Service) clashAPIAddr() string {
	if s.clashAPIAddrFn != nil {
		if addr := strings.TrimSpace(s.clashAPIAddrFn()); addr != "" {
			return addr
		}
	}
	return coredata.ResolveClashAPIAddr()
}

// GetOnlineUsers 返回某节点当前的实时在线订阅人（未过期则有效，否则返回空列表）。
// 支持服务端筛选（关键字：邮箱/客户端名/IP）与分页；汇总统计不受筛选/分页影响。
func (s *Service) GetOnlineUsers(serverID int64, f NodeOnlineFilter) *NodeOnlineResult {
	res := &NodeOnlineResult{Users: []*agentpb.OnlineUserEntry{}}
	if s.nodeOnline == nil {
		return res
	}
	users, updatedAt, ok := s.nodeOnline.GetNodeOnlineUsers(serverID)
	if !ok {
		return res
	}
	var upRate, downRate int64
	for _, u := range users {
		upRate += u.UplinkRate
		downRate += u.DownlinkRate
	}
	res.TotalUplinkRate = upRate
	res.TotalDownlinkRate = downRate
	res.UpdatedAt = updatedAt
	if users == nil {
		return res
	}

	// 富化（填充客户端名等，关键字可据此筛选）
	s.enrichOnlineUsers(users)
	// 标记每条记录连接的是哪个入口（xray / sing-box 都按账号名后缀反查）
	s.fillInboundForServer(context.Background(), serverID, users)

	// 汇总在富化之后做：同一订阅的多个入站账号（email 带不同 _tag）要按订阅 ID
	// 去重，否则「在线订阅人」会按账号数虚高；设备数按 IP 去重（同 IP 多入站算一台）。
	subs := make(map[string]struct{}, len(users))
	devices := make(map[string]struct{})
	for _, u := range users {
		if u.GetSubscriptionId() > 0 {
			subs[strconv.FormatInt(u.GetSubscriptionId(), 10)] = struct{}{}
		} else if u.GetEmail() != "" {
			subs["e:"+u.GetEmail()] = struct{}{}
		}
		for _, sess := range u.GetSessions() {
			if sess != nil && sess.GetIp() != "" {
				devices[sess.GetIp()] = struct{}{}
			}
		}
	}
	res.OnlineSubscriptions = int64(len(subs))
	res.OnlineDevices = int64(len(devices))

	// 关键字筛选
	kw := strings.ToLower(strings.TrimSpace(f.Keyword))
	if kw != "" {
		filtered := make([]*agentpb.OnlineUserEntry, 0, len(users))
		for _, u := range users {
			if strings.Contains(strings.ToLower(u.GetEmail()), kw) ||
				strings.Contains(strings.ToLower(u.GetClientName()), kw) ||
				strings.Contains(strings.ToLower(u.GetInboundName()), kw) ||
				strings.Contains(strings.ToLower(u.GetInboundTag()), kw) {
				filtered = append(filtered, u)
				continue
			}
			for _, sess := range u.Sessions {
				if sess != nil && strings.Contains(sess.GetIp(), kw) {
					filtered = append(filtered, u)
					break
				}
			}
		}
		users = filtered
	}
	res.FilteredTotal = int64(len(users))

	// 分页切片（Limit<=0 表示返回全部）
	if f.Limit > 0 {
		start := max(f.Offset, 0)
		start = min(start, len(users))
		end := min(start+f.Limit, len(users))
		users = users[start:end]
	}
	res.Users = users
	if res.Users == nil {
		res.Users = []*agentpb.OnlineUserEntry{}
	}
	return res
}

// enrichOnlineUsers 根据 subscription_id 联表查询（订阅→客户端→套餐），
// 填充富化字段：客户端名称、套餐名称、限速(Mbps)。查询失败时静默跳过（保留原始 email 展示）。
func (s *Service) enrichOnlineUsers(users []*agentpb.OnlineUserEntry) {
	if len(users) == 0 || s.subRepo == nil || s.clientRepo == nil {
		return
	}
	ctx := context.Background()

	subIDs := make([]int64, 0, len(users))
	for _, u := range users {
		if u.GetSubscriptionId() > 0 {
			subIDs = append(subIDs, u.GetSubscriptionId())
			continue
		}
		// Agent 侧解析不出订阅 ID（管理端账号 email 形如 user@example.com_vless12345，
		// 或旧版 Agent 直接上报 0）→ 面板按邮箱回查客户，补上订阅 ID。
		if subID := s.resolveOnlineSubIDByEmail(ctx, u.GetEmail()); subID > 0 {
			u.SubscriptionId = subID
			subIDs = append(subIDs, subID)
		}
	}
	if len(subIDs) == 0 {
		return
	}

	subs, err := s.subRepo.FindByIDs(ctx, subIDs)
	if err != nil || len(subs) == 0 {
		return
	}

	clientIDs := make([]int64, 0, len(subs))
	for _, sub := range subs {
		if sub != nil && sub.ClientID > 0 {
			clientIDs = append(clientIDs, sub.ClientID)
		}
	}
	clients := map[int64]*entity.Client{}
	if len(clientIDs) > 0 {
		if cs, e := s.clientRepo.FindByIDs(ctx, clientIDs); e == nil {
			clients = cs
		}
	}

	planIDs := make([]int64, 0)
	for _, c := range clients {
		if c != nil && c.PlanID > 0 {
			planIDs = append(planIDs, c.PlanID)
		}
	}
	plans := map[int64]*entity.PlanTemplate{}
	if len(planIDs) > 0 && s.planTmplRepo != nil {
		if ps, e := s.planTmplRepo.FindByIDs(ctx, planIDs); e == nil {
			plans = ps
		}
	}

	for _, u := range users {
		sub, ok := subs[u.GetSubscriptionId()]
		if !ok || sub == nil {
			continue
		}
		c, ok := clients[sub.ClientID]
		if !ok || c == nil {
			continue
		}
		name := c.Nickname
		if name == "" {
			name = c.Email
		}
		if name == "" {
			name = c.Remark
		}
		u.ClientName = name
		// 限速展示：客户端字段 0=继承模板 → 回退模板值显示实际生效的限速
		downLimit, upLimit := c.SpeedLimitMbps, c.SpeedLimitUpMbps
		if c.PlanID > 0 {
			if p, ok := plans[c.PlanID]; ok && p != nil {
				u.PlanName = p.Name
				if downLimit == 0 {
					downLimit = p.SpeedLimitMbps
				}
				if upLimit == 0 {
					upLimit = p.SpeedLimitUpMbps
				}
			}
		}
		u.SpeedLimitDownMbps = int32(downLimit)
		u.SpeedLimitUpMbps = int32(upLimit)
	}
}

// shortInboundTag 复刻 client.generateInboundEmail 的后缀规则：
// "inbound-vless-20296" -> "vless20296"；其它形式原样（统一小写）。
func shortInboundTag(tag string) string {
	tag = strings.TrimSpace(tag)
	parts := strings.Split(tag, "-")
	if len(parts) >= 3 {
		return strings.ToLower(parts[1] + parts[len(parts)-1])
	}
	return strings.ToLower(tag)
}

// accountSuffix 取核心账号名的 _shortTag 后缀：
// "user@example.com_vless20296" -> "vless20296"；没有后缀返回空串。
func accountSuffix(account string) string {
	account = strings.TrimSpace(account)
	idx := strings.LastIndex(account, "_")
	if idx <= 0 {
		return ""
	}
	tail := account[idx+1:]
	if tail == "" || strings.Contains(tail, "@") {
		return ""
	}
	return strings.ToLower(tail)
}

// fillInboundForServer 用「核心账号名后缀」反查这条在线记录连接的入口。
//
// xray 与 sing-box 的账号名都是 <baseEmail>_<shortTag>（shortTag 由入站 tag 派生，
// 见 client.generateInboundEmail），而 OnlineUserEntry.email 已被剥掉后缀，
// 因此用 account_email 还原；面板据此显示「连接的是哪个入口」。
// 匹配不到（旧版 Agent 没上报 account_email）时保持原样，不猜。
func (s *Service) fillInboundForServer(ctx context.Context, serverID int64, users []*agentpb.OnlineUserEntry) {
	if s.inboundRepo == nil || serverID <= 0 || len(users) == 0 {
		return
	}
	cfgs, err := s.inboundRepo.ListByServerID(ctx, serverID)
	if err != nil || len(cfgs) == 0 {
		return
	}
	bySuffix := make(map[string]*entity.InboundConfig, len(cfgs))
	for i := range cfgs {
		c := &cfgs[i]
		key := shortInboundTag(c.Tag)
		if key == "" {
			continue
		}
		if _, exists := bySuffix[key]; !exists {
			bySuffix[key] = c
		}
	}
	for _, u := range users {
		if u == nil {
			continue
		}
		account := u.GetAccountEmail()
		if account == "" {
			account = u.GetEmail()
		}
		var cfg *entity.InboundConfig
		if suffix := accountSuffix(account); suffix != "" {
			cfg = bySuffix[suffix]
		}
		if cfg == nil && account != "" {
			lower := strings.ToLower(account)
			for key, c := range bySuffix {
				if strings.HasSuffix(lower, "_"+key) {
					cfg = c
					break
				}
			}
		}
		if cfg == nil {
			continue
		}
		u.InboundTag = cfg.Tag
		name := strings.TrimSpace(cfg.Remark)
		if name == "" {
			name = cfg.Tag
		}
		u.InboundName = name
		u.InboundPort = int32(cfg.Port)
	}
}

// resolveOnlineSubIDByEmail 按账号 email 反查客户订阅 ID。
// xray 账号 email 带 _tag 后缀（user@example.com_vless12345），客户表里是 base email。
func (s *Service) resolveOnlineSubIDByEmail(ctx context.Context, email string) int64 {
	if s.clientRepo == nil || email == "" {
		return 0
	}
	candidates := []string{email}
	if idx := strings.LastIndex(email, "_"); idx > 0 {
		if tail := email[idx+1:]; !strings.Contains(tail, "@") {
			candidates = append(candidates, email[:idx])
		}
	}
	for _, candidate := range candidates {
		cl, err := s.clientRepo.GetByEmail(ctx, candidate)
		if err == nil && cl != nil && cl.SubscriptionID != nil && *cl.SubscriptionID > 0 {
			return *cl.SubscriptionID
		}
	}
	return 0
}

// OnlineSummaryItem 节点在线汇总项。
type OnlineSummaryItem struct {
	OnlineSubscriptions int64 `json:"online_subscriptions"`
	OnlineDevices       int64 `json:"online_devices"`
}

// OnlineSummaryResponse 是所有节点实时在线汇总的响应，供节点列表与 Dashboard 使用。
type OnlineSummaryResponse struct {
	Nodes              map[string]*OnlineSummaryItem `json:"nodes"`
	TotalSubscriptions int64                         `json:"total_subscriptions"`
	TotalDevices       int64                         `json:"total_devices"`
}

// GetAllOnlineSummary 返回所有节点实时在线汇总，避免前端逐节点轮询（N+1）。
func (s *Service) GetAllOnlineSummary() *OnlineSummaryResponse {
	res := &OnlineSummaryResponse{Nodes: map[string]*OnlineSummaryItem{}}
	if s.nodeOnline == nil {
		return res
	}
	raw := s.nodeOnline.GetAllOnlineSummary()
	var totalSubs, totalDevs int64
	for sid, v := range raw {
		if v == nil {
			continue
		}
		key := strconv.FormatInt(sid, 10)
		res.Nodes[key] = &OnlineSummaryItem{
			OnlineSubscriptions: v.OnlineSubscriptions,
			OnlineDevices:       v.OnlineDevices,
		}
		totalSubs += v.OnlineSubscriptions
		totalDevs += v.OnlineDevices
	}
	res.TotalSubscriptions = totalSubs
	res.TotalDevices = totalDevs
	return res
}

// GlobalOnlineUser 是全局在线用户视图中的一行（一个订阅在某节点上的在线聚合）。
type GlobalOnlineUser struct {
	ServerID           int64                    `json:"server_id"`
	NodeName           string                   `json:"node_name"`
	SubscriptionID     int64                    `json:"subscription_id"`
	CoreType           string                   `json:"core_type"` // xray / singbox
	InboundTag         string                   `json:"inbound_tag"`
	InboundName        string                   `json:"inbound_name"`
	InboundPort        int32                    `json:"inbound_port"`
	Email              string                   `json:"email"`
	ClientName         string                   `json:"client_name"`
	PlanName           string                   `json:"plan_name"`
	SpeedLimitDownMbps int32                    `json:"speed_limit_down_mbps"`
	SpeedLimitUpMbps   int32                    `json:"speed_limit_up_mbps"`
	FirstSeen          int64                    `json:"first_seen"`
	LastSeen           int64                    `json:"last_seen"`
	DownlinkRate       int64                    `json:"downlink_rate"`
	UplinkRate         int64                    `json:"uplink_rate"`
	DownlinkBytes      int64                    `json:"downlink_bytes"`
	UplinkBytes        int64                    `json:"uplink_bytes"`
	SessionCount       int                      `json:"session_count"`
	Sessions           []*agentpb.OnlineSession `json:"sessions"`
}

// GlobalOnlineFilter 是全局在线用户查询的过滤条件（服务端分页与筛选）。
type GlobalOnlineFilter struct {
	ServerID int64  // 按节点筛选（0 = 全部）
	Keyword  string // 关键字（邮箱/客户端名/节点名/IP，大小写不敏感）
	Limit    int    // 每页条数（<=0 表示不分页，返回全部）
	Offset   int    // 偏移量
}

// GlobalOnlineResponse 是全局在线用户页的响应。
type GlobalOnlineResponse struct {
	TotalSubscriptions int64               `json:"total_subscriptions"` // 全部节点在线订阅总数（不受筛选/分页影响）
	TotalDevices       int64               `json:"total_devices"`       // 全部节点在线设备总数（不受筛选/分页影响）
	FilteredTotal      int64               `json:"filtered_total"`      // 符合筛选条件的订阅总数（分页 total）
	Users              []*GlobalOnlineUser `json:"users"`               // 本页数据
}

// GetAllOnlineUsersGlobal 返回所有节点在线订阅人的合并视图（全局在线用户页）。
// 每行带节点名，富化字段（客户端/套餐/限速）已随 Agent 上报由 enrichOnlineUsers 填充逻辑补全。
// 支持服务端筛选（节点/关键字）与分页，避免一次返回全量。
func (s *Service) GetAllOnlineUsersGlobal(f GlobalOnlineFilter) *GlobalOnlineResponse {
	res := &GlobalOnlineResponse{Users: []*GlobalOnlineUser{}}
	if s.nodeOnline == nil {
		return res
	}
	snaps := s.nodeOnline.GetAllOnlineUsers()

	// 收集所有 user 以便统一富化
	all := make([]*agentpb.OnlineUserEntry, 0)
	for _, snap := range snaps {
		all = append(all, snap.Users...)
	}
	s.enrichOnlineUsers(all)

	kw := strings.ToLower(strings.TrimSpace(f.Keyword))
	nodeNameCache := map[int64]string{}
	// 汇总同样按订阅 ID / IP 去重（与单节点视图、online-summary 口径一致），
	// 否则一个客户在 N 个入站都有账号时会显示 N 个在线订阅人。
	subs := make(map[string]struct{})
	devices := make(map[string]struct{})
	matched := make([]*GlobalOnlineUser, 0)
	for serverID, snap := range snaps {
		name, ok := nodeNameCache[serverID]
		if !ok {
			name = s.nodeName(serverID)
			nodeNameCache[serverID] = name
		}
		// 每个节点的入站不同：按 serverID 反查各自入口
		s.fillInboundForServer(context.Background(), serverID, snap.Users)
		for _, u := range snap.Users {
			for _, sess := range u.GetSessions() {
				if sess != nil && sess.GetIp() != "" {
					devices[sess.GetIp()] = struct{}{}
				}
			}
			if u.GetSubscriptionId() > 0 {
				subs[strconv.FormatInt(u.GetSubscriptionId(), 10)] = struct{}{}
			} else if u.GetEmail() != "" {
				subs["e:"+u.GetEmail()] = struct{}{}
			}

			// 节点筛选
			if f.ServerID > 0 && serverID != f.ServerID {
				continue
			}
			// 关键字筛选（邮箱/客户端名/节点名/IP）
			if kw != "" && !matchOnlineUser(u, name, kw) {
				continue
			}

			matched = append(matched, &GlobalOnlineUser{
				ServerID:           serverID,
				NodeName:           name,
				SubscriptionID:     u.GetSubscriptionId(),
				CoreType:           u.GetCoreType(),
				InboundTag:         u.GetInboundTag(),
				InboundName:        u.GetInboundName(),
				InboundPort:        u.GetInboundPort(),
				Email:              u.GetEmail(),
				ClientName:         u.GetClientName(),
				PlanName:           u.GetPlanName(),
				SpeedLimitDownMbps: u.GetSpeedLimitDownMbps(),
				SpeedLimitUpMbps:   u.GetSpeedLimitUpMbps(),
				FirstSeen:          u.GetFirstSeen(),
				LastSeen:           u.GetLastSeen(),
				DownlinkRate:       u.GetDownlinkRate(),
				UplinkRate:         u.GetUplinkRate(),
				DownlinkBytes:      u.GetDownlinkBytes(),
				UplinkBytes:        u.GetUplinkBytes(),
				SessionCount:       len(u.Sessions),
				Sessions:           u.Sessions,
			})
		}
	}
	// 最近活跃的排前面，便于运营关注当前活跃用户（稳定排序保证翻页不乱）
	sort.Slice(matched, func(i, j int) bool {
		if matched[i].LastSeen != matched[j].LastSeen {
			return matched[i].LastSeen > matched[j].LastSeen
		}
		if matched[i].ServerID != matched[j].ServerID {
			return matched[i].ServerID < matched[j].ServerID
		}
		return matched[i].SubscriptionID < matched[j].SubscriptionID
	})

	res.TotalSubscriptions = int64(len(subs))
	res.TotalDevices = int64(len(devices))
	res.FilteredTotal = int64(len(matched))

	// 分页切片（Limit<=0 表示返回全部）
	if f.Limit > 0 {
		start := max(f.Offset, 0)
		start = min(start, len(matched))
		end := min(start+f.Limit, len(matched))
		matched = matched[start:end]
	}
	res.Users = matched
	if res.Users == nil {
		res.Users = []*GlobalOnlineUser{}
	}
	return res
}

// matchOnlineUser 判断在线用户是否命中关键字（邮箱/客户端名/节点名/任一 session IP）。
func matchOnlineUser(u *agentpb.OnlineUserEntry, nodeName, kw string) bool {
	if strings.Contains(strings.ToLower(u.GetEmail()), kw) ||
		strings.Contains(strings.ToLower(u.GetClientName()), kw) ||
		strings.Contains(strings.ToLower(nodeName), kw) {
		return true
	}
	for _, s := range u.Sessions {
		if s != nil && strings.Contains(s.GetIp(), kw) {
			return true
		}
	}
	return false
}

// nodeName 取节点名，失败回退为 "#<id>"。
// 在线用户的 serverID 是 node_servers 的主键（Agent 上报的节点 ID），
// 必须查 node_servers；旧实现查 nodes 表必然查不到，面板上只能显示 "#<id>"。
func (s *Service) nodeName(serverID int64) string {
	ctx := context.Background()
	if s.nodeServerRepo != nil {
		if srv, err := s.nodeServerRepo.GetByID(ctx, serverID); err == nil && srv != nil && srv.Name != "" {
			return srv.Name
		}
	}
	if s.repo != nil {
		if n, err := s.repo.GetByID(ctx, serverID); err == nil && n != nil && n.Name != "" {
			return n.Name
		}
	}
	return "#" + strconv.FormatInt(serverID, 10)
}

// CreateNodeInput is the input for creating a node.
type CreateNodeInput struct {
	Name      string `json:"name" binding:"required"`
	CoreType  string `json:"core_type" binding:"required"`
	Protocol  string `json:"protocol" binding:"required"`
	Host      string `json:"host" binding:"required"`
	Port      int    `json:"port" binding:"required"`
	Transport string `json:"transport"`
	TLSMode   string `json:"tls_mode"`
	Region    string `json:"region"`
	Tags      string `json:"tags"`
}

// UpdateNodeInput is the input for updating a node.
type UpdateNodeInput struct {
	Name      *string `json:"name"`
	CoreType  *string `json:"core_type"`
	Protocol  *string `json:"protocol"`
	Host      *string `json:"host"`
	Port      *int    `json:"port"`
	Transport *string `json:"transport"`
	TLSMode   *string `json:"tls_mode"`
	Region    *string `json:"region"`
	Tags      *string `json:"tags"`
	Status    *int    `json:"status"`
}

// SetNodeServerRepo 注入节点服务仓储（判断本机节点用；未注入时按 serverID==1 兜底）。
func (s *Service) SetNodeServerRepo(repo *repository.NodeServerRepository) {
	s.nodeServerRepo = repo
}

// SingboxConnection 是 sing-box 的实时连接（来自 clash_api /connections）。
//
// sing-box 的 connections **不含用户字段**，所以这里只能给「源 IP + 入站」，
// 无法像 xray 一样归户到订阅；订阅级在线用户仍以 xray 的 GetUsersStats 为准。
type SingboxConnection struct {
	ID          string `json:"id"`
	SourceIP    string `json:"source_ip"`
	SourcePort  string `json:"source_port"`
	Inbound     string `json:"inbound"`      // sing-box 原始写法：anytls/inbound-anytls-49146
	InboundTag  string `json:"inbound_tag"`  // 入站 tag
	InboundName string `json:"inbound_name"` // 入站备注（为空时前端显示 tag）
	InboundPort int32  `json:"inbound_port"`
	Network     string `json:"network"`
	Destination string `json:"destination"`
	Upload      int64  `json:"upload"`
	Download    int64  `json:"download"`
	Start       int64  `json:"start"`
	Chains      string `json:"chains"`
	Rule        string `json:"rule"`
}

// SingboxInboundSummary 按入口聚合的连接数/流量（几千条连接时前端先看这个）。
type SingboxInboundSummary struct {
	InboundTag  string `json:"inbound_tag"`
	InboundName string `json:"inbound_name"`
	Count       int    `json:"count"`
	Upload      int64  `json:"upload"`
	Download    int64  `json:"download"`
}

// SingboxConnectionFilter 是实时连接的筛选/分页条件。
//
// ⚠ sing-box 活跃连接可能有几千条：接口必须分页返回（默认 50、上限 200），
// 否则一次吐出全部 JSON + 前端渲染整表会把页面卡死。
type SingboxConnectionFilter struct {
	Keyword string // 源 IP / 目标 / 入口名，大小写不敏感
	Inbound string // 按入口 tag 过滤（空 = 全部）
	Limit   int
	Offset  int
}

// SingboxConnectionsResult 是「实时连接」接口响应。
type SingboxConnectionsResult struct {
	Items []SingboxConnection `json:"items"` // 本页数据（最多 Limit 条）
	Total int                 `json:"total"` // 筛选后的总条数
	// ByInbound 按入口聚合（不受分页影响），前端用它做下拉筛选与概览。
	ByInbound     []SingboxInboundSummary `json:"by_inbound"`
	UploadTotal   int64                   `json:"upload_total"`
	DownloadTotal int64                   `json:"download_total"`
	UpdatedAt     int64                   `json:"updated_at"`
	// Unsupported=true 时 Reason 说明为什么拿不到（远端节点 / 未开 clash_api）。
	Unsupported bool   `json:"unsupported"`
	Reason      string `json:"reason"`
}

// GetSingboxConnections 读取某节点 sing-box 的实时连接。
//
// 目前只支持**面板本机节点**：sing-box 的 clash_api 只监听回环，远端节点需要
// Agent 转发该 HTTP 请求（尚未实现），因此远端返回 Unsupported 并给出原因。
func (s *Service) GetSingboxConnections(ctx context.Context, serverID int64, f SingboxConnectionFilter) *SingboxConnectionsResult {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Limit > 200 {
		f.Limit = 200
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	res := &SingboxConnectionsResult{
		Items:     []SingboxConnection{},
		ByInbound: []SingboxInboundSummary{},
		UpdatedAt: time.Now().Unix(),
	}
	if s.repo == nil {
		res.Unsupported, res.Reason = true, "node repository not configured"
		return res
	}
	if s.nodeServerRepo != nil {
		srv, err := s.nodeServerRepo.GetByID(ctx, serverID)
		if err != nil || srv == nil {
			res.Unsupported, res.Reason = true, "node not found"
			return res
		}
		if !srv.IsLocal {
			res.Unsupported, res.Reason = true, "remote_node"
			return res
		}
	} else if serverID != 1 {
		res.Unsupported, res.Reason = true, "remote_node"
		return res
	}
	resp, err := coredata.FetchClashConnectionsWithSecret(ctx, s.clashAPIAddr(), s.clashAPISecret())
	if err != nil {
		res.Unsupported, res.Reason = true, "clash_api_unavailable: "+err.Error()
		return res
	}
	res.UploadTotal = resp.UploadTotal
	res.DownloadTotal = resp.DownloadTotal

	meta := s.inboundMeta(ctx, serverID)
	all := make([]SingboxConnection, 0, len(resp.Connections))
	summary := make(map[string]*SingboxInboundSummary)
	for _, c := range resp.Connections {
		dest := c.Metadata.Host
		if dest == "" {
			dest = c.Metadata.DestinationIP
		}
		if c.Metadata.DestinationPort != "" {
			dest += ":" + c.Metadata.DestinationPort
		}
		var start int64
		if t, err := time.Parse(time.RFC3339Nano, c.Start); err == nil {
			start = t.Unix()
		}
		tag := singboxInboundTag(c.Metadata.Type)
		name, port := "", int32(0)
		if m, ok := meta[tag]; ok {
			name, port = m.name, m.port
		}
		all = append(all, SingboxConnection{
			ID:          c.ID,
			SourceIP:    c.Metadata.SourceIP,
			SourcePort:  c.Metadata.SourcePort,
			Inbound:     c.Metadata.Type,
			InboundTag:  tag,
			InboundName: name,
			InboundPort: port,
			Network:     c.Metadata.Network,
			Destination: dest,
			Upload:      c.Upload,
			Download:    c.Download,
			Start:       start,
			Chains:      strings.Join(c.Chains, " → "),
			Rule:        c.Rule,
		})

		key := tag
		if key == "" {
			key = c.Metadata.Type
		}
		sum, ok := summary[key]
		if !ok {
			sum = &SingboxInboundSummary{InboundTag: tag, InboundName: name}
			summary[key] = sum
		}
		sum.Count++
		sum.Upload += c.Upload
		sum.Download += c.Download
	}

	// 概览按连接数倒序，头部入口优先。
	for _, sum := range summary {
		res.ByInbound = append(res.ByInbound, *sum)
	}
	sort.Slice(res.ByInbound, func(i, j int) bool {
		if res.ByInbound[i].Count != res.ByInbound[j].Count {
			return res.ByInbound[i].Count > res.ByInbound[j].Count
		}
		return res.ByInbound[i].InboundName < res.ByInbound[j].InboundName
	})

	// 筛选：入口 + 关键字（源 IP / 目标 / 入口名）。
	kw := strings.ToLower(strings.TrimSpace(f.Keyword))
	inbound := strings.TrimSpace(f.Inbound)
	filtered := make([]SingboxConnection, 0, len(all))
	for _, c := range all {
		if inbound != "" && c.InboundTag != inbound {
			continue
		}
		if kw != "" {
			hay := strings.ToLower(c.SourceIP + " " + c.Destination + " " + c.InboundName + " " + c.InboundTag + " " + c.Inbound)
			if !strings.Contains(hay, kw) {
				continue
			}
		}
		filtered = append(filtered, c)
	}
	// 最近建立的连接排前面。
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Start > filtered[j].Start })
	res.Total = len(filtered)

	if f.Offset < len(filtered) {
		end := min(f.Offset+f.Limit, len(filtered))
		res.Items = filtered[f.Offset:end]
	}
	if res.Items == nil {
		res.Items = []SingboxConnection{}
	}
	return res
}

// singboxInboundTag 从 clash_api 的 metadata.type 取入站 tag：
// "anytls/inbound-anytls-49146" -> "inbound-anytls-49146"。
func singboxInboundTag(typ string) string {
	typ = strings.TrimSpace(typ)
	if idx := strings.LastIndex(typ, "/"); idx >= 0 {
		return strings.TrimSpace(typ[idx+1:])
	}
	return typ
}

type inboundMetaInfo struct {
	name string
	port int32
}

// inboundMeta 返回该节点「入站 tag -> 备注/端口」，用于给连接补入口名。
func (s *Service) inboundMeta(ctx context.Context, serverID int64) map[string]inboundMetaInfo {
	if s.inboundRepo == nil || serverID <= 0 {
		return nil
	}
	cfgs, err := s.inboundRepo.ListByServerID(ctx, serverID)
	if err != nil {
		return nil
	}
	out := make(map[string]inboundMetaInfo, len(cfgs))
	for _, c := range cfgs {
		name := strings.TrimSpace(c.Remark)
		if name == "" {
			name = c.Tag
		}
		out[c.Tag] = inboundMetaInfo{name: name, port: int32(c.Port)}
	}
	return out
}
