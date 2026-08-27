package chainlink

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// Handler handles chain link and chain exit config API requests.
type Handler struct {
	linkRepo      *repository.ChainLinkRepository
	exitCfgRepo   *repository.ChainExitConfigRepository
	extServerRepo *repository.ExternalNodeServerRepository
	serverRepo    *repository.NodeServerRepository
	extNodeRepo   *repository.ExternalNodeRepository
}

// NewHandler creates a new chainlink Handler.
func NewHandler(
	linkRepo *repository.ChainLinkRepository,
	exitCfgRepo *repository.ChainExitConfigRepository,
	extServerRepo *repository.ExternalNodeServerRepository,
	serverRepo *repository.NodeServerRepository,
	extNodeRepo *repository.ExternalNodeRepository,
) *Handler {
	return &Handler{
		linkRepo:      linkRepo,
		exitCfgRepo:   exitCfgRepo,
		extServerRepo: extServerRepo,
		serverRepo:    serverRepo,
		extNodeRepo:   extNodeRepo,
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

	// Chain exit configs
	g.GET("/exit-configs", h.ListExitConfigs)
	g.GET("/exit-configs/:server_id", h.GetExitConfig)
	g.POST("/exit-configs", h.CreateExitConfig)
	g.PUT("/exit-configs/:server_id", h.UpdateExitConfig)
	g.DELETE("/exit-configs/:server_id", h.DeleteExitConfig)

	// External node server registrations
	g.GET("/external-servers", h.ListExternalServers)
	g.POST("/external-servers", h.CreateExternalServer)
	g.DELETE("/external-servers/:id", h.DeleteExternalServer)

	// Topology view
	g.GET("/topology", h.GetTopology)
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

	link := &entity.ChainLink{
		EntryServerID:  req.EntryServerID,
		EntryInboundID: req.EntryInboundID,
		ExitServerID:   req.ExitServerID,
		ExitExternalID: req.ExitExternalID,
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
		ExitTag   *string `json:"exit_tag"`
		Weight    *int    `json:"weight"`
		Priority  *int    `json:"priority"`
		Enabled   *bool   `json:"enabled"`
		SortOrder *int    `json:"sort_order"`
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

	if err := h.linkRepo.Update(ctx, link); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}

	response.OK(c, link)
}

func (h *Handler) DeleteLink(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid link ID")
		return
	}
	if err := h.linkRepo.Delete(ctx, id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// ── Chain Exit Configs ──

func (h *Handler) ListExitConfigs(c *gin.Context) {
	ctx := c.Request.Context()
	configs, err := h.exitCfgRepo.ListAll(ctx)
	if err != nil {
		response.InternalError(c, "DB_ERROR", err.Error())
		return
	}
	response.OK(c, configs)
}

func (h *Handler) GetExitConfig(c *gin.Context) {
	ctx := c.Request.Context()
	serverID, err := strconv.ParseInt(c.Param("server_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	cfg, err := h.exitCfgRepo.GetByServerID(ctx, serverID)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "chain exit config not found")
		return
	}
	response.OK(c, cfg)
}

func (h *Handler) CreateExitConfig(c *gin.Context) {
	ctx := c.Request.Context()
	var req struct {
		ServerID       int64  `json:"server_id" binding:"required"`
		RelayPort      int    `json:"relay_port"`
		RelayProtocol  string `json:"relay_protocol"`
		RelayTransport string `json:"relay_transport"`
		RelaySecurity  string `json:"relay_security"`
		RelayUUID      string `json:"relay_uuid"`
		RelayOnly      bool   `json:"relay_only"`
		Enabled        *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_BODY", err.Error())
		return
	}

	relayProtocol := req.RelayProtocol
	if relayProtocol == "" {
		relayProtocol = "vless"
	}
	relayTransport := req.RelayTransport
	if relayTransport == "" {
		relayTransport = "tcp"
	}
	relaySecurity := req.RelaySecurity
	if relaySecurity == "" {
		relaySecurity = "none"
	}
	relayUUID := req.RelayUUID
	if relayUUID == "" {
		relayUUID = uuid.New().String()
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	cfg := &entity.ChainExitConfig{
		ServerID:       req.ServerID,
		RelayPort:      req.RelayPort,
		RelayProtocol:  relayProtocol,
		RelayTransport: relayTransport,
		RelaySecurity:  relaySecurity,
		RelayUUID:      relayUUID,
		RelayOnly:      req.RelayOnly,
		Enabled:        enabled,
	}

	if err := h.exitCfgRepo.Create(ctx, cfg); err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}

	response.OK(c, cfg)
}

func (h *Handler) UpdateExitConfig(c *gin.Context) {
	ctx := c.Request.Context()
	serverID, err := strconv.ParseInt(c.Param("server_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	cfg, err := h.exitCfgRepo.GetByServerID(ctx, serverID)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "chain exit config not found")
		return
	}

	var req struct {
		RelayPort      *int    `json:"relay_port"`
		RelayProtocol  *string `json:"relay_protocol"`
		RelayTransport *string `json:"relay_transport"`
		RelaySecurity  *string `json:"relay_security"`
		RelayOnly      *bool   `json:"relay_only"`
		Enabled        *bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_BODY", err.Error())
		return
	}

	if req.RelayPort != nil {
		cfg.RelayPort = *req.RelayPort
	}
	if req.RelayProtocol != nil {
		cfg.RelayProtocol = *req.RelayProtocol
	}
	if req.RelayTransport != nil {
		cfg.RelayTransport = *req.RelayTransport
	}
	if req.RelaySecurity != nil {
		cfg.RelaySecurity = *req.RelaySecurity
	}
	if req.RelayOnly != nil {
		cfg.RelayOnly = *req.RelayOnly
	}
	if req.Enabled != nil {
		cfg.Enabled = *req.Enabled
	}

	if err := h.exitCfgRepo.Update(ctx, cfg); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}

	response.OK(c, cfg)
}

func (h *Handler) DeleteExitConfig(c *gin.Context) {
	ctx := c.Request.Context()
	serverID, err := strconv.ParseInt(c.Param("server_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	// Also delete any chain links pointing to this server as exit
	if err := h.linkRepo.DeleteByExit(ctx, serverID); err != nil {
		// Log but don't fail — the exit config is the primary target
		_ = err
	}
	if err := h.exitCfgRepo.Delete(ctx, serverID); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

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
		Name          string `json:"name"`
		Region        string `json:"region"`
		RegionLabel   string `json:"region_label"`
		Enabled       *bool  `json:"enabled"`
		SortOrder     int    `json:"sort_order"`
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

	exitConfigs, err := h.exitCfgRepo.ListAll(ctx)
	if err != nil {
		response.InternalError(c, "DB_ERROR", fmt.Sprintf("list exit configs: %v", err))
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
		ID         int64  `json:"id"`
		Name       string `json:"name"`
		Type       string `json:"type"` // "server" or "external"
		Role       string `json:"role"` // "entry", "exit", "entry+exit", "external_entry"
		RelayOnly  bool   `json:"relay_only"`
	}

	exitCfgMap := make(map[int64]*entity.ChainExitConfig)
	for i := range exitConfigs {
		exitCfgMap[exitConfigs[i].ServerID] = &exitConfigs[i]
	}

	entrySet := make(map[int64]bool)
	for _, l := range links {
		if l.EntryServerID > 0 {
			entrySet[l.EntryServerID] = true
		}
	}

	var nodes []topoNode
	for _, s := range servers {
		role := "standalone"
		isEntry := entrySet[s.ID]
		exitCfg, isExit := exitCfgMap[s.ID]
		relayOnly := false
		if isExit && exitCfg != nil {
			relayOnly = exitCfg.RelayOnly
		}

		if isEntry && isExit {
			role = "entry+exit"
		} else if isEntry {
			role = "entry"
		} else if isExit {
			if relayOnly {
				role = "exit"
			} else {
				role = "exit+entry"
			}
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
			ID:        s.ID,
			Name:      s.Name,
			Type:      "server",
			Role:      role,
			RelayOnly: relayOnly,
		})
	}

	for _, es := range extServers {
		nodes = append(nodes, topoNode{
			ID:        es.ID,
			Name:      es.Name,
			Type:      "external",
			Role:      "external_entry",
			RelayOnly: false,
		})
	}

	response.OK(c, gin.H{
		"nodes": nodes,
		"links": links,
		"exit_configs": exitConfigs,
	})
}

// Ensure context is used (avoids unused import warning in some build configs)
var _ = context.Background
