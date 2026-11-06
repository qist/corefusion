package system

import (
	"corefusion/internal/api/response"
	"corefusion/internal/service/binaries"
	"corefusion/internal/service/healthprobe"
	"corefusion/internal/store/repository"

	"github.com/gin-gonic/gin"
)

// ProbeHandler 暴露「优选域名 / 优选 IP」的探活入口与探活配置。
type ProbeHandler struct {
	prober   *healthprobe.Prober
	nodeRepo *repository.ExternalNodeRepository
	resolver *binaries.Resolver
}

// NewProbeHandler 创建探活处理器。
func NewProbeHandler(prober *healthprobe.Prober, nodeRepo *repository.ExternalNodeRepository, resolver *binaries.Resolver) *ProbeHandler {
	return &ProbeHandler{prober: prober, nodeRepo: nodeRepo, resolver: resolver}
}

// Check 立即探测全部优选域名/IP，返回本次探测汇总。
func (h *ProbeHandler) Check(c *gin.Context) {
	res, err := h.prober.CheckAll(c.Request.Context())
	if err != nil {
		response.InternalError(c, "PROBE_FAILED", err.Error())
		return
	}
	response.OK(c, res)
}

// GetConfig 返回探活配置 + 可选入口节点 + 当前已安装的拨号核心。
func (h *ProbeHandler) GetConfig(c *gin.Context) {
	ctx := c.Request.Context()
	cfg := h.prober.ReadConfig(ctx)

	type nodeItem struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		Protocol string `json:"protocol"`
		Host     string `json:"host"`
	}
	nodes := make([]nodeItem, 0, 32)
	if h.nodeRepo != nil {
		if list, err := h.nodeRepo.ListAll(ctx); err == nil {
			for _, n := range list {
				if !n.Enabled || n.ProxyJSON == "" {
					continue
				}
				nodes = append(nodes, nodeItem{ID: n.ID, Name: n.Name, Protocol: n.Protocol, Host: n.Host})
			}
		}
	}

	// 拨号核心：只列出真正装了的（mihomo 需要用户自行安装到 binaries/active/mihomo）
	dialers := make([]string, 0, 3)
	if h.resolver != nil {
		for _, ct := range []string{"singbox", "xray", "mihomo"} {
			if _, _, err := h.resolver.Resolve(ct); err == nil {
				dialers = append(dialers, ct)
			}
		}
	}

	// 配置字段平铺，附带入可选入口节点与已安装核心；前端不再单独拼。
	response.OK(c, struct {
		healthprobe.Config
		Nodes   []nodeItem `json:"nodes"`
		Dialers []string   `json:"dialers"`
	}{Config: cfg, Nodes: nodes, Dialers: dialers})
}

// SaveConfig 更新探活配置（间隔 / 延迟上限 / 入口节点 / 拨号核心）。
func (h *ProbeHandler) SaveConfig(c *gin.Context) {
	var req struct {
		IntervalMinutes *int    `json:"interval_minutes"`
		MaxLatencyMs    *int    `json:"max_latency_ms"`
		EntryNodeID     *int64  `json:"entry_node_id"`
		Dialer          *string `json:"dialer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if req.IntervalMinutes != nil && (*req.IntervalMinutes < 1 || *req.IntervalMinutes > 1440) {
		response.BadRequest(c, "INVALID_REQUEST", "interval_minutes 需在 1-1440 之间")
		return
	}
	if req.MaxLatencyMs != nil && *req.MaxLatencyMs < 0 {
		response.BadRequest(c, "INVALID_REQUEST", "max_latency_ms 不能为负")
		return
	}
	if req.EntryNodeID != nil && *req.EntryNodeID > 0 && h.nodeRepo != nil {
		if _, err := h.nodeRepo.GetByID(c.Request.Context(), *req.EntryNodeID); err != nil {
			response.BadRequest(c, "INVALID_REQUEST", "入口节点不存在")
			return
		}
	}
	if err := h.prober.SaveConfig(c.Request.Context(), req.IntervalMinutes, req.MaxLatencyMs, req.EntryNodeID, req.Dialer); err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}
	response.OK(c, h.prober.ReadConfig(c.Request.Context()))
}
