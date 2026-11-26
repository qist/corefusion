package system

import (
	"strconv"

	"corefusion/internal/api/response"
	"corefusion/internal/service/nodecheckup"
	"github.com/gin-gonic/gin"
)

// NodeCheckupHandler 提供节点安全体检的只读查询与配置接口。
type NodeCheckupHandler struct {
	svc *nodecheckup.Service
}

func NewNodeCheckupHandler(svc *nodecheckup.Service) *NodeCheckupHandler {
	return &NodeCheckupHandler{svc: svc}
}

// GetNodeReport handles GET /api/v1/node-servers/:id/checkup?refresh=1
func (h *NodeCheckupHandler) GetNodeReport(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	refresh := c.Query("refresh") == "1" || c.Query("refresh") == "true"
	report, err := h.svc.GetReport(c.Request.Context(), id, refresh)
	if err != nil {
		response.InternalError(c, "CHECKUP_FAILED", err.Error())
		return
	}
	response.OK(c, report)
}

// GetGuide handles GET /api/v1/node-servers/:id/checkup/guide
// 返回一份可直接整段复制的 Markdown：内核参数 + SSH 加固 + SSH 来源限制 + 回滚。
func (h *NodeCheckupHandler) GetGuide(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	report, err := h.svc.GetReport(c.Request.Context(), id, false)
	if err != nil {
		response.InternalError(c, "CHECKUP_FAILED", err.Error())
		return
	}
	sshPort := 22
	fwType := ""
	for _, it := range report.Items {
		switch it.ID {
		case "ENV-01":
			if v, ok := it.Params["ssh_port"]; ok {
				if n, ok := v.(float64); ok && n > 0 {
					sshPort = int(n)
				}
				if n, ok := v.(int); ok && n > 0 {
					sshPort = n
				}
			}
		case "FW-01":
			if v, ok := it.Params["value"].(string); ok && v != "" {
				fwType = v
			}
			if v, ok := it.Params["type"].(string); ok && v != "" {
				fwType = v
			}
		}
	}
	markdown := nodecheckup.BuildGuideWithConflicts(report.Env, sshPort, fwType, report.Items)
	response.OK(c, gin.H{
		"server_id":    id,
		"generated_at": report.GeneratedAt,
		"ssh_port":     sshPort,
		"firewall":     fwType,
		"markdown":     markdown,
	})
}

// Summary handles GET /api/v1/node-servers/checkup/summary
func (h *NodeCheckupHandler) Summary(c *gin.Context) {
	rows, err := h.svc.ListSummary(c.Request.Context())
	if err != nil {
		response.InternalError(c, "CHECKUP_SUMMARY_FAILED", err.Error())
		return
	}
	response.OK(c, rows)
}

// GetSettings handles GET /api/v1/system/node-checkup
func (h *NodeCheckupHandler) GetSettings(c *gin.Context) {
	settings, err := h.svc.GetSettings(c.Request.Context())
	if err != nil {
		response.InternalError(c, "CHECKUP_SETTINGS_FAILED", err.Error())
		return
	}
	response.OK(c, settings)
}

// SaveSettings handles PUT /api/v1/system/node-checkup
func (h *NodeCheckupHandler) SaveSettings(c *gin.Context) {
	var settings nodecheckup.Settings
	if err := c.ShouldBindJSON(&settings); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	saved, err := h.svc.SaveSettings(c.Request.Context(), settings)
	if err != nil {
		response.BadRequest(c, "INVALID_SETTINGS", err.Error())
		return
	}
	response.OK(c, saved)
}
