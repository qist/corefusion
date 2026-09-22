package chainlink

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
	"corefusion/internal/runtime/agent"
	"corefusion/internal/store/repository"
)

// HealthChecker checks if a server's Agent is online.
// Implemented by *agent.NodeServerManager.
type HealthChecker interface {
	IsOnline(serverID int64) bool
}

// ConfigSyncHook triggers config regeneration and push to remote nodes.
// Implemented by *configSyncHookAdapter in router.go.
// When chain_links change, the handler calls this
// to ensure the new topology takes effect immediately (design doc §13.2).
type ConfigSyncHook interface {
	SyncServerConfig(ctx context.Context, serverID int64, coreType string)
}

// Handler handles chain link and chain exit config API requests.
type Handler struct {
	linkRepo       *repository.ChainLinkRepository
	extServerRepo  *repository.ExternalNodeServerRepository
	serverRepo     *repository.NodeServerRepository
	extNodeRepo    *repository.ExternalNodeRepository
	inboundRepo    *repository.InboundConfigRepository
	healthChecker  HealthChecker  // optional, injected via SetHealthChecker
	configSyncHook ConfigSyncHook // optional, injected via SetConfigSyncHook
}

// NewHandler creates a new chainlink Handler.
func NewHandler(
	linkRepo *repository.ChainLinkRepository,
	extServerRepo *repository.ExternalNodeServerRepository,
	serverRepo *repository.NodeServerRepository,
	extNodeRepo *repository.ExternalNodeRepository,
	inboundRepo *repository.InboundConfigRepository,
) *Handler {
	return &Handler{
		linkRepo:      linkRepo,
		extServerRepo: extServerRepo,
		serverRepo:    serverRepo,
		extNodeRepo:   extNodeRepo,
		inboundRepo:   inboundRepo,
	}
}

// SetHealthChecker injects a health checker (typically *NodeServerManager).
func (h *Handler) SetHealthChecker(hc HealthChecker) {
	h.healthChecker = hc
}

// SetConfigSyncHook injects a config sync hook (typically *configSyncHookAdapter).
// When set, chain_links mutations automatically trigger
// config regeneration and push to affected remote nodes (design doc §13.2).
func (h *Handler) SetConfigSyncHook(hook ConfigSyncHook) {
	h.configSyncHook = hook
}

// triggerSync triggers config sync for the entry server and all exit servers
// referenced by the given links. This ensures both the entry node (which gets
// new chain-* outbounds / routing) and exit nodes (which get relay-in inbounds)
// receive updated configurations.
func (h *Handler) triggerSync(links ...entity.ChainLink) {
	if h.configSyncHook == nil {
		return
	}
	synced := make(map[int64]bool)
	for _, l := range links {
		if l.EntryServerID > 0 && !synced[l.EntryServerID] {
			synced[l.EntryServerID] = true
			h.configSyncHook.SyncServerConfig(context.Background(), l.EntryServerID, "xray")
		}
		if l.ExitServerID > 0 && !synced[l.ExitServerID] {
			synced[l.ExitServerID] = true
			h.configSyncHook.SyncServerConfig(context.Background(), l.ExitServerID, "xray")
		}
	}
}

// RegisterRoutes registers all chain proxy routes under the given group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/chain")
	// Chain links CRUD
	g.GET("/links", h.ListLinks)
	g.GET("/links/:id", h.GetLink)
	g.POST("/links", h.CreateLink)
	g.PUT("/links/:id", h.UpdateLink)
	g.DELETE("/links/:id", h.DeleteLink)

	// External node server registrations
	g.GET("/external-servers", h.ListExternalServers)
	g.POST("/external-servers", h.CreateExternalServer)
	g.DELETE("/external-servers/:id", h.DeleteExternalServer)

	// Topology view
	g.GET("/topology", h.GetTopology)

	// Health status of all chain links
	g.GET("/health", h.GetHealth)

	// 创建链式代理：选择入口 → 选择出口（单/多，多选自动判定为负载均衡）→ 自动关联
	// 见设计文档 §5.5 创建链式代理向导（端到端操作与后端自动行为）
	g.POST("/proxy", h.CreateChainProxy)
}

// ── Chain Links ──

func (h *Handler) ListLinks(c *gin.Context) {
	ctx := c.Request.Context()
	entryServerID, _ := strconv.ParseInt(c.Query("entry_server_id"), 10, 64)
	exitServerID, _ := strconv.ParseInt(c.Query("exit_server_id"), 10, 64)

	if entryServerID > 0 {
		links, err := h.linkRepo.ListAllByEntry(ctx, entryServerID)
		if err != nil {
			response.InternalError(c, "DB_ERROR", err.Error())
			return
		}
		response.OK(c, links)
		return
	}

	if exitServerID > 0 {
		links, err := h.linkRepo.ListByExit(ctx, exitServerID)
		if err != nil {
			response.InternalError(c, "DB_ERROR", err.Error())
			return
		}
		response.OK(c, links)
		return
	}

	links, err := h.linkRepo.ListAll(ctx)
	if err != nil {
		response.InternalError(c, "DB_ERROR", err.Error())
		return
	}
	response.OK(c, links)
}

func (h *Handler) GetLink(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid link ID")
		return
	}
	link, err := h.linkRepo.GetByID(ctx, id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "chain link not found")
		return
	}
	response.OK(c, link)
}

func (h *Handler) CreateLink(c *gin.Context) {
	ctx := c.Request.Context()
	var req struct {
		EntryServerID  int64  `json:"entry_server_id"`
		EntryInboundID int64  `json:"entry_inbound_id"`
		ExitServerID   int64  `json:"exit_server_id"`
		ExitExternalID int64  `json:"exit_external_id"`
		ExitInboundID  int64  `json:"exit_inbound_id"`
		ExitTag        string `json:"exit_tag"`
		Weight         int    `json:"weight"`
		Priority       int    `json:"priority"`
		Enabled        *bool  `json:"enabled"`
		SortOrder      int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_BODY", err.Error())
		return
	}

	if req.ExitServerID == 0 && req.ExitExternalID == 0 {
		response.BadRequest(c, "INVALID_EXIT", "either exit_server_id or exit_external_id must be set")
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	weight := req.Weight
	if weight == 0 {
		weight = 100
	}
	priority := req.Priority
	if priority == 0 {
		priority = 1
	}

	// 选了"出口已有的入站"就校验一下：入站存在、属于该出口机、协议在本入口核心的支持名单里
	// （实测 xray 没有 hysteria2 出站，所以 hy2 出口只能配给 sing-box 入口）。
	if req.ExitInboundID > 0 && req.ExitServerID > 0 {
		exitIb, ierr := h.inboundRepo.GetByID(ctx, req.ExitInboundID)
		if ierr != nil || exitIb == nil || exitIb.ServerID != req.ExitServerID {
			response.BadRequest(c, "EXIT_INBOUND_NOT_FOUND", "exit inbound not found on this exit server")
			return
		}
		entryIb, _ := h.inboundRepo.GetByID(ctx, req.EntryInboundID)
		entryCore := ""
		if entryIb != nil {
			entryCore = entryIb.CoreType
		}
		if !agent.ChainSupportedProtocol(entryCore, exitIb.Protocol) {
			response.BadRequest(c, "EXIT_PROTOCOL_UNSUPPORTED", fmt.Sprintf(
				"exit inbound protocol %q is not chain-capable for entry core %q (supported: %s)",
				exitIb.Protocol, entryCore, strings.Join(agent.ChainSupportedProtocols(entryCore), "/")))
			return
		}
	}

	link := &entity.ChainLink{
		EntryServerID:  req.EntryServerID,
		EntryInboundID: req.EntryInboundID,
		ExitServerID:   req.ExitServerID,
		ExitExternalID: req.ExitExternalID,
		ExitInboundID:  req.ExitInboundID,
		ExitTag:        req.ExitTag,
		Weight:         weight,
		Priority:       priority,
		Enabled:        enabled,
		SortOrder:      req.SortOrder,
	}

	if err := h.linkRepo.Create(ctx, link); err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}

	// Trigger config sync for entry and exit nodes (§13.2)
	h.triggerSync(*link)

	response.OK(c, link)
}

func (h *Handler) UpdateLink(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid link ID")
		return
	}
	link, err := h.linkRepo.GetByID(ctx, id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "chain link not found")
		return
	}

	var req struct {
		ExitTag       *string `json:"exit_tag"`
		Weight        *int    `json:"weight"`
		Priority      *int    `json:"priority"`
		Enabled       *bool   `json:"enabled"`
		SortOrder     *int    `json:"sort_order"`
		ExitInboundID *int64  `json:"exit_inbound_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_BODY", err.Error())
		return
	}

	if req.ExitTag != nil {
		link.ExitTag = *req.ExitTag
	}
	if req.Weight != nil {
		link.Weight = *req.Weight
	}
	if req.Priority != nil {
		link.Priority = *req.Priority
	}
	if req.Enabled != nil {
		link.Enabled = *req.Enabled
	}
	if req.SortOrder != nil {
		link.SortOrder = *req.SortOrder
	}
	if req.ExitInboundID != nil {
		if *req.ExitInboundID > 0 && link.ExitServerID > 0 {
			exitIb, ierr := h.inboundRepo.GetByID(ctx, *req.ExitInboundID)
			if ierr != nil || exitIb == nil || exitIb.ServerID != link.ExitServerID {
				response.BadRequest(c, "EXIT_INBOUND_NOT_FOUND", "exit inbound not found on this exit server")
				return
			}
			entryIb, _ := h.inboundRepo.GetByID(ctx, link.EntryInboundID)
			entryCore := ""
			if entryIb != nil {
				entryCore = entryIb.CoreType
			}
			if !agent.ChainSupportedProtocol(entryCore, exitIb.Protocol) {
				response.BadRequest(c, "EXIT_PROTOCOL_UNSUPPORTED", fmt.Sprintf(
					"exit inbound protocol %q is not chain-capable for entry core %q",
					exitIb.Protocol, entryCore))
				return
			}
		}
		link.ExitInboundID = *req.ExitInboundID
	}

	if err := h.linkRepo.Update(ctx, link); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}

	// Trigger config sync for entry and exit nodes (§13.2)
	h.triggerSync(*link)

	response.OK(c, link)
}

func (h *Handler) DeleteLink(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid link ID")
		return
	}
	// Capture link info before delete for config sync
	link, _ := h.linkRepo.GetByID(ctx, id)
	if err := h.linkRepo.Delete(ctx, id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	// Trigger config sync for affected nodes (§13.2)
	if link != nil {
		h.triggerSync(*link)
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// ── Chain Exit Configs ──

// ── External Node Servers ──

func (h *Handler) ListExternalServers(c *gin.Context) {
	ctx := c.Request.Context()
	servers, err := h.extServerRepo.ListAll(ctx)
	if err != nil {
		response.InternalError(c, "DB_ERROR", err.Error())
		return
	}
	response.OK(c, servers)
}

func (h *Handler) CreateExternalServer(c *gin.Context) {
	ctx := c.Request.Context()
	var req struct {
		ExternalNodeID int64  `json:"external_node_id" binding:"required"`
		Name           string `json:"name"`
		Region         string `json:"region"`
		RegionLabel    string `json:"region_label"`
		Enabled        *bool  `json:"enabled"`
		SortOrder      int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_BODY", err.Error())
		return
	}

	// Check external node exists
	extNode, err := h.extNodeRepo.GetByID(ctx, req.ExternalNodeID)
	if err != nil {
		response.BadRequest(c, "EXT_NODE_NOT_FOUND", "external node not found")
		return
	}

	name := req.Name
	if name == "" {
		name = extNode.Name
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	srv := &entity.ExternalNodeServer{
		ExternalNodeID: req.ExternalNodeID,
		Name:           name,
		Hostname:       extNode.Host,
		Region:         req.Region,
		RegionLabel:    req.RegionLabel,
		Enabled:        enabled,
		SortOrder:      req.SortOrder,
	}

	if err := h.extServerRepo.Create(ctx, srv); err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}

	response.OK(c, srv)
}

func (h *Handler) DeleteExternalServer(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid ID")
		return
	}
	if err := h.extServerRepo.Delete(ctx, id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// ── Topology ──

func (h *Handler) GetTopology(c *gin.Context) {
	ctx := c.Request.Context()

	links, err := h.linkRepo.ListAll(ctx)
	if err != nil {
		response.InternalError(c, "DB_ERROR", fmt.Sprintf("list links: %v", err))
		return
	}

	extServers, err := h.extServerRepo.ListAll(ctx)
	if err != nil {
		response.InternalError(c, "DB_ERROR", fmt.Sprintf("list external servers: %v", err))
		return
	}

	servers, err := h.serverRepo.ListAll(ctx)
	if err != nil {
		response.InternalError(c, "DB_ERROR", fmt.Sprintf("list servers: %v", err))
		return
	}

	// Build node list
	type topoNode struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"` // "server" or "external"
		Role string `json:"role"` // "entry", "exit", "entry+exit", "external_entry"
	}

	// 入口/出口角色完全由 chain_links 决定（v2.0 起没有 chain_exit_configs 了）：
	//   入口 = 有链路以它为 entry；出口 = 有链路以它为 exit。
	entrySet := make(map[int64]bool)
	exitSet := make(map[int64]bool)
	for _, l := range links {
		if l.EntryServerID > 0 {
			entrySet[l.EntryServerID] = true
		}
		if l.IsExitServer() {
			exitSet[l.ExitServerID] = true
		}
	}

	var nodes []topoNode
	for _, s := range servers {
		role := "standalone"
		isEntry := entrySet[s.ID]
		isExit := exitSet[s.ID]

		if isEntry && isExit {
			role = "entry+exit"
		} else if isEntry {
			role = "entry"
		} else if isExit {
			role = "exit"
		}
		// Skip standalone nodes that aren't involved in chain proxy
		if role == "standalone" && !s.Enabled {
			continue
		}
		// Only include nodes that are part of the chain topology
		if role == "standalone" {
			continue
		}

		nodes = append(nodes, topoNode{
			ID:   s.ID,
			Name: s.Name,
			Type: "server",
			Role: role,
		})
	}

	for _, es := range extServers {
		nodes = append(nodes, topoNode{
			ID:   es.ID,
			Name: es.Name,
			Type: "external",
			Role: "external_entry",
		})
	}

	response.OK(c, gin.H{
		"nodes": nodes,
		"links": links,
	})
}

// ── Create Chain Proxy (wizard, see design doc §5.5) ──

// ExitTarget describes a single chosen exit in the create-chain-proxy wizard.
type ExitTarget struct {
	IsExternal bool  `json:"is_external"`
	ID         int64 `json:"id"` // cluster server_id (IsExternal=false) or external node server id (IsExternal=true)
	Priority   int   `json:"priority"`
	Weight     int   `json:"weight"`
	SortOrder  int   `json:"sort_order"`
	// ExitInboundID：该出口节点上**已有的入站**（推荐用法）。
	// >0 时链式这边不创建任何入站，直接把入口的流量交给这个入站；
	// cluster 出口必须指定"出口节点上已有的入站"（exit_inbound_id）。
	ExitInboundID int64 `json:"exit_inbound_id"`
	// ExitTag：出站 tag 自定义名（留空则自动生成 chain-<入口入站ID>-<出口ID>）。
	// tag 会出现在节点配置与路由规则里，便于人工排查。
	ExitTag string `json:"exit_tag"`
}

// CreateChainProxy handles the create-chain-proxy wizard flow (§5.5):
//  1. validate entry server + entry inbound exist;
//  2. validate every cluster exit has an existing inbound (exit_inbound_id) and every external exit is enabled;
//  3. SyncFromEntry: clear-then-insert chain_links for this entry;
//  4. multi-exit (≥2) auto-selects load balancing strategy (§6.6); single exit → direct.
//
// Config regeneration is automatic: nodes pull their config via configsync on the
// next heartbeat, which calls PrepareFullConfig with the updated chain_links.
func (h *Handler) CreateChainProxy(c *gin.Context) {
	ctx := c.Request.Context()
	var req struct {
		EntryServerID  int64        `json:"entry_server_id" binding:"required"`
		EntryInboundID int64        `json:"entry_inbound_id" binding:"required"`
		Exits          []ExitTarget `json:"exits" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_BODY", err.Error())
		return
	}

	// 1. validate entry server + inbound exist
	entryServer, err := h.serverRepo.GetByID(ctx, req.EntryServerID)
	if err != nil || entryServer == nil {
		response.BadRequest(c, "ENTRY_NOT_FOUND", "entry server not found")
		return
	}
	inbound, err := h.inboundRepo.GetByID(ctx, req.EntryInboundID)
	if err != nil || inbound == nil || inbound.ServerID != req.EntryServerID {
		response.BadRequest(c, "INBOUND_NOT_FOUND", "entry inbound not found on this server")
		return
	}

	// 2. validate every exit target
	for _, ex := range req.Exits {
		if ex.IsExternal {
			extSrv, eerr := h.extServerRepo.GetByID(ctx, ex.ID)
			if eerr != nil || extSrv == nil || !extSrv.Enabled {
				response.BadRequest(c, "EXIT_EXT_NOT_FOUND", fmt.Sprintf("external exit %d not found or disabled", ex.ID))
				return
			}
			// 外部出口的协议也要在**入口核心**的支持名单里（与集群出口同一套规则）。
			// 协议在它关联的 external_node 上 → 需要再跳一跳。
			// ⚠ 这里此前**没有**协议校验 ✗：入口 xray + 外部 hy2 会被 API 接受并落库，只有生成器
			//   才报错 → 用户看到"创建成功"，DB 里却留下一条永远生成不出配置的链接 ✗。
			if extNode, nerr := h.extNodeRepo.GetByID(ctx, extSrv.ExternalNodeID); nerr == nil && extNode != nil {
				if !agent.ChainSupportedProtocol(inbound.CoreType, extNode.Protocol) {
					response.BadRequest(c, "EXIT_PROTOCOL_UNSUPPORTED", fmt.Sprintf(
						"external exit %d protocol %q is not chain-capable for entry core %q (supported: %s)",
						ex.ID, extNode.Protocol, inbound.CoreType,
						strings.Join(agent.ChainSupportedProtocols(inbound.CoreType), "/")))
					return
				}
			}
		} else if ex.ExitInboundID > 0 {
			// 出口用**该节点上已有的入站** —— 链式不创建任何入站。
			// 只校验：入站存在、属于这台出口机、协议在本入口核心的链式支持名单里
			// （实测：xray 没有 hysteria2 出站 → `unknown config id: hysteria2`，所以 hy2 出口
			//  只能给 sing-box 入口用）。
			exitIb, ierr := h.inboundRepo.GetByID(ctx, ex.ExitInboundID)
			if ierr != nil || exitIb == nil {
				response.BadRequest(c, "EXIT_INBOUND_NOT_FOUND", fmt.Sprintf("exit inbound %d not found", ex.ExitInboundID))
				return
			}
			if exitIb.ServerID != ex.ID {
				response.BadRequest(c, "EXIT_INBOUND_MISMATCH", fmt.Sprintf("exit inbound %d does not belong to server %d", ex.ExitInboundID, ex.ID))
				return
			}
			if !agent.ChainSupportedProtocol(inbound.CoreType, exitIb.Protocol) {
				response.BadRequest(c, "EXIT_PROTOCOL_UNSUPPORTED", fmt.Sprintf(
					"exit inbound %d protocol %q is not chain-capable for entry core %q (supported: %s)",
					ex.ExitInboundID, exitIb.Protocol, inbound.CoreType,
					strings.Join(agent.ChainSupportedProtocols(inbound.CoreType), "/")))
				return
			}
		} else {
			// v2.0：集群出口**必须**指定"出口节点上已有的入站"
			// （旧的 chain_exit_configs / 自动建 relay-in 入站模式已移除）
			response.BadRequest(c, "EXIT_INBOUND_REQUIRED", fmt.Sprintf(
				"exit server %d: exit_inbound_id is required (legacy chain_exit_configs mode has been removed)", ex.ID))
			return
		}
	}

	// 3. clear-then-insert (SyncFromEntry, §5.4/§5.5)
	if err := h.linkRepo.DeleteByEntry(ctx, req.EntryServerID); err != nil {
		response.InternalError(c, "SYNC_FAILED", err.Error())
		return
	}
	for i, ex := range req.Exits {
		priority := ex.Priority
		if priority == 0 {
			priority = i + 1 // default priority by selection order
		}
		weight := ex.Weight
		if weight == 0 {
			weight = 100
		}
		link := &entity.ChainLink{
			EntryServerID:  req.EntryServerID,
			EntryInboundID: req.EntryInboundID,
			Priority:       priority,
			Weight:         weight,
			ExitTag:        ex.ExitTag,
			SortOrder:      ex.SortOrder,
			Enabled:        true,
		}
		if ex.IsExternal {
			link.ExitExternalID = ex.ID
		} else {
			link.ExitServerID = ex.ID
			link.ExitInboundID = ex.ExitInboundID
		}
		if err := h.linkRepo.Create(ctx, link); err != nil {
			response.InternalError(c, "CREATE_LINK_FAILED", err.Error())
			return
		}
	}

	// 4. strategy auto-selection: ≥2 exits ⇒ load balancing (§6.6); single ⇒ direct
	strategy := "single"
	if len(req.Exits) > 1 {
		strategy = "loadbalance"
	}

	// 5. Trigger config sync for entry + all exit nodes (§13.2)
	if h.configSyncHook != nil {
		h.configSyncHook.SyncServerConfig(context.Background(), req.EntryServerID, "xray")
		synced := make(map[int64]bool)
		synced[req.EntryServerID] = true
		for _, ex := range req.Exits {
			if !ex.IsExternal && ex.ID > 0 && !synced[ex.ID] {
				synced[ex.ID] = true
				h.configSyncHook.SyncServerConfig(context.Background(), ex.ID, "xray")
			}
		}
		log.Printf("[chainlink] CreateChainProxy: triggered config sync for entry %d + %d exit nodes",
			req.EntryServerID, len(synced)-1)
	}

	response.OK(c, gin.H{
		"message":          "chain proxy created",
		"strategy":         strategy,
		"entry_server_id":  req.EntryServerID,
		"entry_inbound_id": req.EntryInboundID,
		"exit_count":       len(req.Exits),
	})
}

// Ensure context is used (avoids unused import warning in some build configs)
var _ = context.Background

// ── Health ──

// GetHealth returns the health status of all chain links.
// For each entry→exit pair, returns whether both nodes are online and
// the relay-in inbound is reachable.
func (h *Handler) GetHealth(c *gin.Context) {
	ctx := c.Request.Context()

	links, err := h.linkRepo.ListAll(ctx)
	if err != nil {
		response.InternalError(c, "DB_ERROR", fmt.Sprintf("list links: %v", err))
		return
	}

	type LinkHealth struct {
		LinkID         int64  `json:"link_id"`
		EntryServerID  int64  `json:"entry_server_id"`
		EntryInboundID int64  `json:"entry_inbound_id"`
		ExitServerID   int64  `json:"exit_server_id"`
		ExitExternalID int64  `json:"exit_external_id"`
		EntryOnline    bool   `json:"entry_online"`
		ExitOnline     bool   `json:"exit_online"`
		Healthy        bool   `json:"healthy"`
		Status         string `json:"status"` // healthy | degraded | down
	}

	result := make([]LinkHealth, 0, len(links))
	for _, link := range links {
		if !link.Enabled {
			continue
		}

		entryOnline := false
		exitOnline := false
		status := "down"

		if h.healthChecker != nil {
			entryOnline = h.healthChecker.IsOnline(link.EntryServerID)
			if link.ExitExternalID > 0 {
				exitOnline = true // external exits are always "online"
			} else if link.ExitServerID > 0 {
				exitOnline = h.healthChecker.IsOnline(link.ExitServerID)
			}
		}

		healthy := entryOnline && exitOnline
		if healthy {
			status = "healthy"
		} else if entryOnline || exitOnline {
			status = "degraded"
		}

		result = append(result, LinkHealth{
			LinkID:         link.ID,
			EntryServerID:  link.EntryServerID,
			EntryInboundID: link.EntryInboundID,
			ExitServerID:   link.ExitServerID,
			ExitExternalID: link.ExitExternalID,
			EntryOnline:    entryOnline,
			ExitOnline:     exitOnline,
			Healthy:        healthy,
			Status:         status,
		})
	}

	response.OK(c, result)
}
