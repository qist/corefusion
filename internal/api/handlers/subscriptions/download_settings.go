package subscriptions

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"corefusion/internal/model/entity"
	"corefusion/internal/subscription/sharelink"
)

// downloadInboundKey 是入站配置里「下行入口」的引用键，存放在
// streamSettings.xhttpSettings.extra 下。
//
// 只存目标入站 ID，不存地址/端口等快照：渲染分享链接/订阅时再按目标入站
// 的当前配置解析，目标被禁用/删除/改成非 xhttp 时自动回退为单入口，
// 避免出现"配了一个不存在的下行入口"。
const downloadInboundKey = "_downloadInboundId"

// resolveDownloadSettings 按引用把目标入站解析成完整的 XHTTP downloadSettings
// （上下行分离），写进 uploadConfig 的 xhttp extra 里。
//
// 仅 XHTTP 入站、且目标满足以下条件时生效：
//   - 同一节点、已启用、不是自己
//   - 传输为 xhttp
//   - security 为 tls / reality（Xray 的 downloadSettings 只接受这两种）
func (h *PublicHandler) resolveDownloadSettings(
	ctx context.Context,
	cfg entity.InboundConfig,
	config map[string]interface{},
	all []entity.InboundConfig,
	shareDomains []shareDomainEntry,
) {
	if !strings.EqualFold(cfg.CoreType, "xray") || config == nil {
		return
	}
	xhttp := getMap(getMap(config, "streamSettings"), "xhttpSettings")
	if xhttp == nil {
		return
	}
	// stream-one 与 downloadSettings 互斥（Xray/mihomo 都会拒绝这种组合），
	// 此时回退为单入口，避免下发出客户端加载不了的配置。
	if strings.EqualFold(getString(xhttp, "mode", "auto"), "stream-one") {
		return
	}
	ext := getMap(xhttp, "extra")
	if ext == nil {
		return
	}
	targetID := toInt64(ext[downloadInboundKey])
	if targetID <= 0 || targetID == cfg.ID {
		return
	}

	var target *entity.InboundConfig
	for i := range all {
		if all[i].ID == targetID {
			target = &all[i]
			break
		}
	}
	if target == nil || !target.Enabled || target.ServerID != cfg.ServerID {
		return
	}

	var tcfg map[string]interface{}
	if err := json.Unmarshal([]byte(target.ConfigJSON), &tcfg); err != nil {
		return
	}
	tss := getMap(tcfg, "streamSettings")
	if !strings.EqualFold(getString(tss, "network", ""), "xhttp") {
		return
	}
	security := strings.ToLower(getString(tss, "security", ""))
	if security != "tls" && security != "reality" {
		return
	}

	host, port := h.hostForLink(ctx, target, tcfg, shareDomains)
	if host == "" || port <= 0 {
		return
	}
	txh := getMap(tss, "xhttpSettings")
	// 下行 path 必须与上行一致（Xray XHTTP 上下行分离的硬约束）。
	uploadPath := getString(xhttp, "path", "/")
	if uploadPath == "" {
		uploadPath = "/"
	}
	targetPath := getString(txh, "path", "/")
	if targetPath == "" {
		targetPath = "/"
	}
	if uploadPath != targetPath {
		return
	}

	dl := map[string]interface{}{
		"address":  host,
		"port":     port,
		"network":  "xhttp",
		"security": security,
	}

	switch security {
	case "tls":
		tls := map[string]interface{}{}
		if t := getMap(tss, "tlsSettings"); t != nil {
			sni := getString(t, "serverName", "")
			if sni == "" {
				sni = host
			}
			tls["serverName"] = sni
			if alpn := getStringSlice(t, "alpn"); len(alpn) > 0 {
				tls["alpn"] = alpn
			}
			if fp := getString(t, "fingerprint", ""); fp != "" {
				tls["fingerprint"] = fp
			}
		}
		dl["tlsSettings"] = tls
	case "reality":
		rp := sharelink.RealityClientParams(tss)
		reality := map[string]interface{}{}
		if rp.ServerName != "" {
			reality["serverName"] = rp.ServerName
		}
		if rp.PublicKey != "" {
			// xray 客户端侧公钥字段是 password
			reality["password"] = rp.PublicKey
		}
		if rp.ShortID != "" {
			reality["shortId"] = rp.ShortID
		}
		if rp.Fingerprint != "" {
			reality["fingerprint"] = rp.Fingerprint
		}
		if rp.SpiderX != "" {
			reality["spiderX"] = rp.SpiderX
		}
		dl["realitySettings"] = reality
	}

	// 下行侧的 xhttp 参数（路径必须与上行一致，其余按目标入站自身的设置）
	xh := map[string]interface{}{"path": targetPath}
	if host2 := getString(txh, "host", ""); host2 != "" {
		xh["host"] = host2
	}
	if txhExtra := getMap(txh, "extra"); txhExtra != nil {
		safe := stripPanelKeys(txhExtra)
		if xmux := getMap(safe, "xmux"); len(xmux) > 0 {
			xh["extra"] = map[string]interface{}{"xmux": xmux}
		}
	}
	dl["xhttpSettings"] = xh

	ext["downloadSettings"] = dl
}

// stripPanelKeys 递归移除面板内部的下划线字段。
func stripPanelKeys(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		if strings.HasPrefix(k, "_") {
			continue
		}
		if child, ok := v.(map[string]interface{}); ok {
			out[k] = stripPanelKeys(child)
			continue
		}
		out[k] = v
	}
	return out
}

// toInt64 兼容 JSON number（float64）、int 与字符串形式的 ID。
func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case int:
		return int64(val)
	case int64:
		return val
	case json.Number:
		n, _ := val.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
		return n
	}
	return 0
}
