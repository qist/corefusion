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

	// format=uri（默认，分享链接）/ mihomo / singbox / xray（可直接粘贴的配置片段）。
	format := normalizeShareFormat(c.Query("format"))
	shareDomains := h.loadShareDomains(c.Request.Context())
	nodeName := cfg.Remark
	if nodeName == "" {
		nodeName = cfg.Tag
	}

	// 协议是否在该客户端里受支持（与订阅同一份矩阵）——决定前端是显示
	// "该客户端不支持此协议" 还是 "无可渲染内容"。
	matrix := snippetSupportMatrix(format)
	matrixProto := strings.ToLower(cfg.Protocol)
	if matrixProto == "hysteria" {
		matrixProto = "hysteria2"
	}
	protocolSupported := matrix[matrixProto]
	// 生不出片段时要把**原因**告诉前端（协议不支持 / 传输不支持），而不是只丢一个空框
	reason := ""
	if !protocolSupported {
		reason = "protocol=" + matrixProto
	}

	items := make([]gin.H, 0, 4)
	userRecords := inboundUserRecords(config)
	for idx, u := range userRecords {
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
		// 注意：这里传**完整的 settings**（而不是只挑 clients/encryption）——
		// ss 的加密方式（method）、http/socks 的认证等都从这里取，
		// 只挑两个键会让链接用错 cipher（如把 2022-blake3 写成 aes-256-gcm）。
		link := sharelink.Generate(sharelink.Node{
			Tag:            nodeName,
			Protocol:       cfg.Protocol,
			Host:           host,
			Port:           port,
			Settings:       settings,
			StreamSettings: getMap(view, "streamSettings"),
			FinalMask:      finalMaskWithPortHop(config),
		})
		if !protocolSupported {
			continue
		}
		node := formatterNodeForShare(nodeName, cfg, view, client, host, port)
		text, ok := renderShareSnippet(format, link, node)
		if !ok {
			if reason == "" {
				if r := shareUnsupportedReason(format, node); r != "" {
					reason = r
				} else {
					reason = "protocol=" + matrixProto
				}
			}
			continue
		}
		items = append(items, gin.H{
			"user":  u.label,
			"keys":  inboundUserKeys(u),
			"index": idx,
			"link":  link,
			"text":  text,
		})
	}

	// 用户列表与 format **无关**：前端的选择框用它（后端过滤掉已暂停的用户，前端不再
	// 自己枚举用户、也不再按 email/name 做兜底匹配），items 里的 index 指回这里。
	users := make([]gin.H, 0, len(userRecords))
	for idx, u := range userRecords {
		users = append(users, gin.H{"index": idx, "label": u.label})
	}

	if len(users) == 0 {
		// 一个可用用户都没有（没有任何用户记录，或全部已暂停）——原因要说清楚，
		// 否则前端只能显示"不支持该协议"，把"没有用户"误报成"协议不支持"。
		protocolSupported = false
		if reason == "" {
			reason = "users"
		}
	} else if protocolSupported && len(items) == 0 {
		// 协议在矩阵里、但一条都没渲染出来（如 sing-box 表达不了的 xhttp 传输）时，
		// 同样按"不支持"返回 —— 前端才能给出明确提示，而不是一个空框。
		protocolSupported = false
		if reason == "" {
			reason = "protocol=" + matrixProto
		}
	}

	host, port := h.hostForLink(c.Request.Context(), cfg, config, shareDomains)
	response.OK(c, gin.H{
		"items":     items,
		"users":     users,
		"host":      host,
		"port":      port,
		"protocol":  cfg.Protocol,
		"enabled":   cfg.Enabled,
		"format":    format,
		"supported": protocolSupported,
		"reason":    reason,
	})
}

// mihomoProxySnippet 把分享链接转成可直接粘进 mihomo(clash) 配置的 proxies 片段。
// 刻意复用 **mihomo 订阅的链路**（mihomoconv + 订阅归一化 + writeMihomoProxyMap），
// 这样"编辑器里复制的片段"与"订阅里的节点"逐字段一致，不会出现两套口径。
func mihomoProxySnippet(link string) string {
	if strings.TrimSpace(link) == "" {
		return ""
	}
	converted := convertLinksToMihomoProxyMaps([]string{link})
	if len(converted) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("proxies:\n")
	writeMihomoProxyMap(&b, converted[0])
	return b.String()
}

// hostForLink 计算分享链接里的主机名（与订阅**同一套**逻辑，见 shareAddress）：
// 接入域名里已选择的域名/IP → 节点公网/内网地址 → 监听地址。
// 关键点：启用了接入域名但下拉**没选**时不能凭空取一个面板域名（否则 server 与
// 编辑器里选的东西对不上），也不能让 listen=0.0.0.0 兜底成 127.0.0.1 就完事。
func (h *PublicHandler) hostForLink(ctx context.Context, cfg *entity.InboundConfig, view map[string]interface{}, shareDomains []shareDomainEntry) (string, int) {
	if cfg == nil {
		return "", 0
	}
	return h.shareAddress(ctx, *cfg, view, shareDomains)
}

// inboundUser 一条用户记录 + 对外展示标识。
type inboundUser struct {
	label  string                 // 展示/前端选择用的标识（xray: email；sing-box: name 或 username）
	record map[string]interface{} // 原始记录
}

// inboundUserRecords 列出该入站里的用户记录：
//   - xray    : settings.clients[]（vless/vmess/trojan/ss/hysteria…）
//     settings.accounts[]（http/socks，形状是 {user, pass} —— 不认这个形状
//     会导致这类入站在分享/二维码/片段里"一个用户都没有"）
//   - sing-box: 顶层 users[]
func inboundUserRecords(config map[string]interface{}) []inboundUser {
	out := make([]inboundUser, 0, 4)
	seen := map[string]bool{}
	add := func(label string, rec map[string]interface{}) {
		key := strings.ToLower(strings.TrimSpace(label))
		if key == "" || seen[key] {
			return
		}
		// 暂停用户在运行中的核里已经不存在了（coreconfig/prepare.go 会把 _paused 的用户
		// 从下发的配置里删掉），分享/二维码列出来只会让人以为"这条能用"。
		// 过滤放在**后端**：前端只管渲染后端给的条目，不再自己枚举用户
		// （两套口径必然漂移 —— 之前就靠 email/name 做兜底匹配）。
		if paused, _ := rec["_paused"].(bool); paused {
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
		if accounts, ok := settings["accounts"].([]interface{}); ok {
			for _, a := range accounts {
				m, ok := a.(map[string]interface{})
				if !ok {
					continue
				}
				label := getString(m, "user", "")
				if label == "" {
					label = getString(m, "username", "")
				}
				// 归一成 client 形状，下游（链接生成 / 节点构造）只认 username/password
				rec := make(map[string]interface{}, len(m)+3)
				for k, v := range m {
					rec[k] = v
				}
				rec["email"] = label
				rec["username"] = label
				if pw := getString(m, "pass", ""); pw != "" {
					rec["password"] = pw
				}
				add(label, rec)
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
	if len(out) == 0 {
		// 服务端级别的"单用户"入站：凭证直接写在 settings / 配置顶层，不在
		// clients[] / accounts[] / users[] 里（xray 的 shadowsocks 单用户
		// settings.password + settings.method、sing-box 的 ss / hysteria2 单用户
		// 顶层 password…）。不合成时这类入站在分享/二维码/片段里"一个用户都没有"，
		// 前端会显示"不支持该协议"，而 mihomo / sing-box / xray 其实都直接支持它。
		if u, ok := defaultUserRecord(config); ok {
			add(u.label, u.record)
		}
	}
	return out
}

// defaultUserRecord 为"单用户入站"合成一条用户记录。
// label 固定 "default"，与前端 XrayInbounds.getUsers() 的虚拟用户口径一致（email: 'default'），
// 否则前后端的用户 key 对不上，弹窗里选中用户后拿不到片段。
func defaultUserRecord(config map[string]interface{}) (inboundUser, bool) {
	settings := getMap(config, "settings")
	password := getString(settings, "password", "")
	if password == "" {
		password = getString(config, "password", "")
	}
	if password != "" {
		rec := map[string]interface{}{
			"email":    "default",
			"password": password,
		}
		// ss 的加密方式在 settings.method（链接/片段仍从 settings 取，这里带一份便于节点渲染）
		if method := getString(settings, "method", ""); method != "" {
			rec["method"] = method
		}
		return inboundUser{label: "default", record: rec}, true
	}
	// 无认证的 socks / http（xray 的 settings.auth 缺省或 "noauth"）也能分享出去
	switch strings.ToLower(getString(config, "protocol", "")) {
	case "socks", "socks5", "http", "mixed":
		if auth := strings.ToLower(getString(settings, "auth", "noauth")); auth == "" || auth == "noauth" {
			return inboundUser{label: "default", record: map[string]interface{}{"email": "default"}}, true
		}
	}
	return inboundUser{}, false
}

// inboundUserKeys 该用户的所有候选标识（小写）。
// 前端选择用户用的键与后端 label 口径未必一致（xray 用 email、sing-box 用 name/username），
// 把候选标识一并返回，前端"任一命中即算同一个用户"，避免匹配不上导致空片段。
func inboundUserKeys(u inboundUser) []string {
	out := make([]string, 0, 4)
	seen := map[string]bool{}
	add := func(v string) {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" || seen[v] {
			return
		}
		seen[v] = true
		out = append(out, v)
	}
	add(u.label) // 最权威的标识放最前
	for _, k := range []string{"email", "name", "username", "id", "uuid", "user"} {
		add(getString(u.record, k, ""))
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
	// 保留原有 settings 的其它键（method / network / auth …）：只留 clients+encryption
	// 会让 ss 的加密方式退回默认值，生成出**连不上**的链接（2022-blake3 被写成 aes-256-gcm）。
	merged := map[string]interface{}{}
	if settings := getMap(config, "settings"); settings != nil {
		for k, v := range settings {
			merged[k] = v
		}
	}
	merged["clients"] = []interface{}{u.record}
	out["settings"] = merged
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
