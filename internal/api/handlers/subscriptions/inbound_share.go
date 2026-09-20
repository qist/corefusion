package subscriptions

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
	"corefusion/internal/subscription/sharelink"
)

// InboundShareLinks 为一个入站里的每个用户生成分享链接。
//
//	GET /api/v1/node-servers/:id/inbound-configs/:cfgId/share-links
//
// 与订阅**完全同源**：subscriptionView 归一（sing-box → xray 形态，且只保留该用户凭证）
// → resolveHostPort 解析接入域名/端口 → finalMaskWithPortHop 注入端口跳跃
// → sharelink.Generate 生成。
//
// 用途：编辑器里的二维码/复制链接。此前二维码走的是前端自己那套生成器，参数不全且与订阅
// 不一致（naive/anytls 直接返回空、vmess 的 JSON 字段错、缺 fp/spx、没有端口跳跃信息等）。
func (h *PublicHandler) InboundShareLinks(c *gin.Context) {
	cfgID, err := strconv.ParseInt(c.Param("cfgId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid inbound config id")
		return
	}
	cfg, err := h.inboundRepo.GetByID(c.Request.Context(), cfgID)
	if err != nil {
		response.BadRequest(c, "INBOUND_NOT_FOUND", err.Error())
		return
	}
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(cfg.ConfigJSON), &config); err != nil {
		response.BadRequest(c, "INVALID_CONFIG", err.Error())
		return
	}

	shareDomains := h.loadShareDomains(c.Request.Context())
	nodeName := cfg.Remark
	if nodeName == "" {
		nodeName = cfg.Tag
	}

	items := make([]gin.H, 0, 4)
	for _, u := range inboundUserRecords(config) {
		view := shareViewForUser(cfg.Tag, config, u)
		if view == nil {
			continue
		}
		settings := getMap(view, "settings")
		client := firstClient(settings)
		if client == nil {
			continue
		}
		host, port := h.hostForLink(c.Request.Context(), cfg, view, shareDomains)
		link := sharelink.Generate(sharelink.Node{
			Tag:      nodeName,
			Protocol: cfg.Protocol,
			Host:     host,
			Port:     port,
			Settings: map[string]interface{}{
				"clients":    []interface{}{client},
				"encryption": getString(settings, "encryption", "none"),
			},
			StreamSettings: getMap(view, "streamSettings"),
			FinalMask:      finalMaskWithPortHop(config),
		})
		if link != "" {
			items = append(items, gin.H{"user": u.label, "link": link})
		}
	}

	host, port := h.hostForLink(c.Request.Context(), cfg, config, shareDomains)
	response.OK(c, gin.H{
		"items":    items,
		"host":     host,
		"port":     port,
		"protocol": cfg.Protocol,
		"enabled":  cfg.Enabled,
	})
}

// hostForLink 计算分享链接里的主机名：
// 启用接入域名时用 resolveHostPort 的结果（与订阅一致）；
// 未启用时退回节点的公网/内网地址 —— 否则 listen=0.0.0.0 会兜底成 127.0.0.1，
// 二维码扫出来对客户端毫无意义。
func (h *PublicHandler) hostForLink(ctx context.Context, cfg *entity.InboundConfig, view map[string]interface{}, shareDomains []shareDomainEntry) (string, int) {
	host, port := resolveHostPort(view, cfg.Listen, cfg.Port, shareDomains)
	if ad, ok := view["_accessDomain"].(map[string]interface{}); ok {
		if enabled, _ := ad["enabled"].(bool); enabled {
			return host, port
		}
	}
	if h.nodeServerRepo != nil {
		if node, err := h.nodeServerRepo.GetByID(ctx, cfg.ServerID); err == nil && node != nil {
			if node.PublicIP != "" {
				return node.PublicIP, port
			}
			if node.InternalIP != "" {
				return node.InternalIP, port
			}
		}
	}
	return host, port
}

// inboundUser 一条用户记录 + 对外展示标识。
type inboundUser struct {
	label  string                 // 展示/前端选择用的标识（xray: email；sing-box: name 或 username）
	record map[string]interface{} // 原始记录
}

// inboundUserRecords 列出该入站里的用户记录：
//   - xray    : settings.clients[]
//   - sing-box: 顶层 users[]
func inboundUserRecords(config map[string]interface{}) []inboundUser {
	out := make([]inboundUser, 0, 4)
	seen := map[string]bool{}
	add := func(label string, rec map[string]interface{}) {
		key := strings.ToLower(strings.TrimSpace(label))
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		out = append(out, inboundUser{label: strings.TrimSpace(label), record: rec})
	}
	if settings := getMap(config, "settings"); settings != nil {
		if clients, ok := settings["clients"].([]interface{}); ok {
			for _, c := range clients {
				if m, ok := c.(map[string]interface{}); ok {
					add(getString(m, "email", ""), m)
				}
			}
		}
	}
	if users, ok := config["users"].([]interface{}); ok {
		for _, u := range users {
			m, ok := u.(map[string]interface{})
			if !ok {
				continue
			}
			label := getString(m, "name", "")
			if label == "" {
				label = getString(m, "username", "")
			}
			add(label, m)
		}
	}
	return out
}

// shareViewForUser 取出「该用户视角」的归一化配置：
// sing-box 走 singboxViewForUser（只保留该用户 + 反推 streamSettings）；
// xray 复制一份并把 settings.clients 收敛成该用户（绝不把他人凭证带进分享链接）。
func shareViewForUser(tag string, config map[string]interface{}, u inboundUser) map[string]interface{} {
	ensureAccessDomain(config)
	if isSingboxInboundConfig(config) {
		return singboxViewForUser(config, u.record, u.label)
	}
	out := make(map[string]interface{}, len(config))
	for k, v := range config {
		out[k] = v
	}
	out["settings"] = map[string]interface{}{
		"clients":    []interface{}{u.record},
		"encryption": getString(getMap(config, "settings"), "encryption", "none"),
	}
	return out
}

// firstClient 取归一化配置里唯一的那个 client。
func firstClient(settings map[string]interface{}) map[string]interface{} {
	if settings == nil {
		return nil
	}
	clients, ok := settings["clients"].([]interface{})
	if !ok || len(clients) == 0 {
		return nil
	}
	c, _ := clients[0].(map[string]interface{})
	return c
}
