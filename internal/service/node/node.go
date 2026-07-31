package node

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"corefusion/internal/model/entity"
	agentsvc "corefusion/internal/runtime/agent"
	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/store/repository"
)

// Service handles node business logic.
type Service struct {
	repo         *repository.NodeRepository
	nodeOnline   nodeOnlineProvider
	subRepo      *repository.SubscriptionRepository
	clientRepo   *repository.ClientRepository
	planTmplRepo *repository.PlanTemplateRepository
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
	var subs, devs, upRate, downRate int64
	for _, u := range users {
		subs++
		devs += int64(len(u.Sessions))
		upRate += u.UplinkRate
		downRate += u.DownlinkRate
	}
	res.OnlineSubscriptions = subs
	res.OnlineDevices = devs
	res.TotalUplinkRate = upRate
	res.TotalDownlinkRate = downRate
	res.UpdatedAt = updatedAt
	if users == nil {
		return res
	}

	// 富化（填充客户端名等，关键字可据此筛选）
	s.enrichOnlineUsers(users)

	// 关键字筛选
	kw := strings.ToLower(strings.TrimSpace(f.Keyword))
	if kw != "" {
		filtered := make([]*agentpb.OnlineUserEntry, 0, len(users))
		for _, u := range users {
			if strings.Contains(strings.ToLower(u.GetEmail()), kw) ||
				strings.Contains(strings.ToLower(u.GetClientName()), kw) {
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
		if c.PlanID > 0 {
			if p, ok := plans[c.PlanID]; ok && p != nil {
				u.PlanName = p.Name
			}
		}
		u.SpeedLimitDownMbps = int32(c.SpeedLimitMbps)
		u.SpeedLimitUpMbps = int32(c.SpeedLimitUpMbps)
	}
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
	ServerID           int64                      `json:"server_id"`
	NodeName           string                     `json:"node_name"`
	SubscriptionID     int64                      `json:"subscription_id"`
	Email              string                     `json:"email"`
	ClientName         string                     `json:"client_name"`
	PlanName           string                     `json:"plan_name"`
	SpeedLimitDownMbps int32                      `json:"speed_limit_down_mbps"`
	SpeedLimitUpMbps   int32                      `json:"speed_limit_up_mbps"`
	FirstSeen          int64                      `json:"first_seen"`
	LastSeen           int64                      `json:"last_seen"`
	DownlinkRate       int64                      `json:"downlink_rate"`
	UplinkRate         int64                      `json:"uplink_rate"`
	DownlinkBytes      int64                      `json:"downlink_bytes"`
	UplinkBytes        int64                      `json:"uplink_bytes"`
	SessionCount       int                        `json:"session_count"`
	Sessions           []*agentpb.OnlineSession   `json:"sessions"`
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
	var totalSubs, totalDevs int64
	matched := make([]*GlobalOnlineUser, 0)
	for serverID, snap := range snaps {
		name, ok := nodeNameCache[serverID]
		if !ok {
			name = s.nodeName(serverID)
			nodeNameCache[serverID] = name
		}
		for _, u := range snap.Users {
			totalSubs++
			totalDevs += int64(len(u.Sessions))

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

	res.TotalSubscriptions = totalSubs
	res.TotalDevices = totalDevs
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
func (s *Service) nodeName(serverID int64) string {
	if s.repo == nil {
		return "#" + strconv.FormatInt(serverID, 10)
	}
	n, err := s.repo.GetByID(context.Background(), serverID)
	if err != nil || n == nil || n.Name == "" {
		return "#" + strconv.FormatInt(serverID, 10)
	}
	return n.Name
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

// Create creates a new node.
func (s *Service) Create(ctx context.Context, input CreateNodeInput) (*entity.Node, error) {
	node := &entity.Node{
		Name:      input.Name,
		CoreType:  input.CoreType,
		Protocol:  input.Protocol,
		Host:      input.Host,
		Port:      input.Port,
		Transport: input.Transport,
		TLSMode:   input.TLSMode,
		Region:    input.Region,
		Tags:      input.Tags,
		Status:    1,
	}

	if err := s.repo.Create(ctx, node); err != nil {
		return nil, err
	}

	return node, nil
}

// GetByID retrieves a node by ID.
func (s *Service) GetByID(ctx context.Context, id int64) (*entity.Node, error) {
	return s.repo.GetByID(ctx, id)
}

// List retrieves all nodes with pagination.
func (s *Service) List(ctx context.Context, coreType string, limit, offset int) ([]entity.Node, int64, error) {
	return s.repo.List(ctx, coreType, limit, offset)
}

// Update updates a node.
func (s *Service) Update(ctx context.Context, id int64, input UpdateNodeInput) (*entity.Node, error) {
	node, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Name != nil {
		node.Name = *input.Name
	}
	if input.CoreType != nil {
		node.CoreType = *input.CoreType
	}
	if input.Protocol != nil {
		node.Protocol = *input.Protocol
	}
	if input.Host != nil {
		node.Host = *input.Host
	}
	if input.Port != nil {
		node.Port = *input.Port
	}
	if input.Transport != nil {
		node.Transport = *input.Transport
	}
	if input.TLSMode != nil {
		node.TLSMode = *input.TLSMode
	}
	if input.Region != nil {
		node.Region = *input.Region
	}
	if input.Tags != nil {
		node.Tags = *input.Tags
	}
	if input.Status != nil {
		node.Status = *input.Status
	}

	if err := s.repo.Update(ctx, node); err != nil {
		return nil, err
	}

	return node, nil
}

// Delete deletes a node by ID.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
