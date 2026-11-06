package system

import (
	"corefusion/internal/api/response"
	"corefusion/internal/service/healthprobe"

	"github.com/gin-gonic/gin"
)

// ProbeHandler 暴露「优选域名 / 优选 IP」的手动探活入口。
type ProbeHandler struct {
	prober *healthprobe.Prober
}

// NewProbeHandler 创建探活处理器。
func NewProbeHandler(prober *healthprobe.Prober) *ProbeHandler {
	return &ProbeHandler{prober: prober}
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
