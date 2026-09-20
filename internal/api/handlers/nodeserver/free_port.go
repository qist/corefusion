package nodeserver

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
)

// reservedPorts 是面板/核心自身占用的端口，不允许分配给业务入站。
var reservedPorts = map[int]bool{
	22:    true, // ssh
	80:    true,
	443:   true,
	8080:  true, // 面板 HTTP
	8443:  true, // 节点 agent / https
	9090:  true, // clash api 默认
	10085: true, // xray stats api
	10086: true, // sing-box v2ray api
	10087: true,
	18443: true, // agent gRPC
	5353:  true,
	53:    true,
	853:   true,
}

const (
	freePortMin = 10000
	freePortMax = 60000
	freePortTry = 200
)

// GetFreePort handles GET /api/v1/node-servers/:id/free-port
//
// 返回一个「该节点尚未被占用」的候选端口，供前端创建入站/端点时使用。
//
// 判定口径：**同一端口号只要被任何入站/端点用过，无论 TCP 还是 UDP 都算已占用**，
// 避免出现「A 入站用 443/TCP、B 入站又拿到 443/UDP」这类同端口重复。
// 数据来源：
//  1. inbound_configs 中该节点的 Port 字段与 config_json 里的 port / listen_port / settings.port
//  2. core_configs 各核心 inbounds 段的 listen_port / port（历史段落数据）
//  3. core_configs 各核心 endpoints 段的 listen_port
//  4. 面板/核心保留端口（reservedPorts）
//
// 对本地节点（id=1）还会真实探测 TCP/UDP 能否绑定，避免与其它进程冲突。
func (h *Handler) GetFreePort(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}

	used := h.collectNodeUsedPorts(c.Request.Context(), serverID, c.Query("core_type"))
	port := pickFreePort(used, serverID == 1)

	response.OK(c, gin.H{
		"port":      port,
		"server_id": serverID,
		"used":      len(used),
	})
}

// collectNodeUsedPorts 汇总某节点已占用的端口。
func (h *Handler) collectNodeUsedPorts(ctx context.Context, serverID int64, coreType string) map[int]bool {
	used := make(map[int]bool, 32)
	for p := range reservedPorts {
		used[p] = true
	}

	coreTypes := []string{"xray", "singbox", "hysteria2", "tuic"}
	if coreType != "" {
		coreTypes = []string{coreType}
	}

	// 1) inbound_configs（入站表）
	if h.inboundConfigSvc != nil {
		for _, ct := range coreTypes {
			cfgs, err := h.inboundConfigSvc.List(ctx, serverID, ct)
			if err != nil {
				continue
			}
			for _, cfg := range cfgs {
				if cfg.Port > 0 {
					used[cfg.Port] = true
				}
				addPortsFromJSON(used, json.RawMessage(cfg.ConfigJSON))
			}
		}
	}

	// 2) endpoint_configs（端点表）——端点编辑器写的是这张表，必须一起算，
	// 否则「入站端口」会撞上「端点端口」（反之亦然）。
	if h.endpointConfigSvc != nil {
		for _, ct := range coreTypes {
			eps, err := h.endpointConfigSvc.List(ctx, serverID, ct)
			if err != nil {
				continue
			}
			for _, ep := range eps {
				addPortsFromJSON(used, json.RawMessage(ep.ConfigJSON))
			}
		}
	}

	// 3) core_configs 各核心的 inbounds / endpoints 段
	if h.coreConfigSvc != nil {
		for _, ct := range coreTypes {
			full, err := h.coreConfigSvc.GetFull(ctx, serverID, ct)
			if err != nil || full == nil {
				continue
			}
			// services 段也含本地监听端口（api/ccm/derp/ssm-api/usbip-server 等）
			for _, key := range []string{"inbounds", "endpoints", "services"} {
				raw, ok := full[key]
				if !ok {
					continue
				}
				if b, err := json.Marshal(raw); err == nil {
					addPortsFromJSON(used, b)
				}
			}
		}
	}
	return used
}

// addPortsFromJSON 从入站/端点 JSON 中抽取端口字段（含数组形式）。
func addPortsFromJSON(used map[int]bool, raw json.RawMessage) {
	if len(raw) == 0 {
		return
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return
	}
	collectPorts(v, used, 0)
}

// collectPorts 递归遍历 JSON，收集端口类字段。
func collectPorts(v interface{}, used map[int]bool, depth int) {
	if depth > 6 {
		return
	}
	switch val := v.(type) {
	case []interface{}:
		for _, item := range val {
			collectPorts(item, used, depth+1)
		}
	case map[string]interface{}:
		for k, item := range val {
			switch k {
			// 只统计「本地监听」意义的端口：
			//   port        —— xray inbound 的监听端口（含嵌套 settings.port）
			//   listen_port —— sing-box inbound / endpoint 的监听端口
			// 以下字段是「远端/转发目标」端口，不能算作本地占用，否则会把合法端口也排除：
			//   server_port / remote_port / peer_port（对端）、override_port（direct 转发目标）
			case "port", "listen_port":
				if p, ok := toPort(item); ok {
					used[p] = true
				}
				continue
			case "portMap":
				// xray dokodemo-door: {"80": "1.2.3.4:8080"}
				if m, ok := item.(map[string]interface{}); ok {
					for pk := range m {
						if p, err := strconv.Atoi(pk); err == nil && p > 0 && p <= 65535 {
							used[p] = true
						}
					}
				}
				continue
			}
			collectPorts(item, used, depth+1)
		}
	}
}

// toPort 把 JSON 里的端口值转成 int（数字或数字字符串）。
func toPort(v interface{}) (int, bool) {
	switch n := v.(type) {
	case float64:
		if n > 0 && n <= 65535 {
			return int(n), true
		}
	case string:
		if p, err := strconv.Atoi(n); err == nil && p > 0 && p <= 65535 {
			return p, true
		}
	}
	return 0, false
}

// pickFreePort 在可用范围内随机取一个未被占用的端口。
// localProbe=true 时额外做 TCP/UDP 真实绑定探测（用于本地节点）。
func pickFreePort(used map[int]bool, localProbe bool) int {
	for i := 0; i < freePortTry; i++ {
		p := freePortMin + rand.Intn(freePortMax-freePortMin)
		if used[p] {
			continue
		}
		if localProbe && !portBindable(p) {
			continue
		}
		return p
	}
	// 极端情况：顺序扫一遍
	for p := freePortMin; p < freePortMax; p++ {
		if !used[p] && (!localProbe || portBindable(p)) {
			return p
		}
	}
	return 0
}

// portBindable 报告 TCP/UDP 是否能绑定该端口（本地节点用，避免与其它进程冲突）。
func portBindable(port int) bool {
	addr := fmt.Sprintf(":%d", port)
	if ln, err := net.Listen("tcp", addr); err != nil {
		return false
	} else {
		ln.Close()
	}
	if pc, err := net.ListenPacket("udp", addr); err != nil {
		return false
	} else {
		pc.Close()
	}
	return true
}
