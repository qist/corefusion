package subscriptions

import (
	"encoding/json"
	"strconv"
	"strings"

	"corefusion/internal/model/entity"
)

// parseConfigMap 把 config_json 解成 map；坏数据返回 nil（调用方按"没有引用关系"处理）。
func parseConfigMap(raw string) map[string]interface{} {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	return m
}

// xray 的 fallback 结构：一个**对外**入口入站（典型是 443 上的 REALITY/TLS）
// 把请求按 SNI 转给若干个**内部**入站（监听 @abstract 或 /dev/shm/*.sock）。
//
//	any_SNI_No_SNI (vless+REALITY, :59026)
//	  fallbacks:
//	    - {name: "",           dest: "@vless-grpc"}   ← 空 SNI
//	    - {name: "qomvix.com", dest: "@trojan-grpc"}  ← 内部 trojan+gRPC
//	    - {name: "ravexo.com", dest: "@vless-h2"}     ← 内部 vless+xhttp
//
// 内部入站自己是 `security: none`（TLS 已在外层终止，fallback 转发的是**已解密**的流），
// 所以直接拿它的 streamSettings 生成客户端配置会得到"明文 + 抽象 socket 地址"，
// 客户端根本连不上。正确的客户端配置是：
//
//	地址/端口/security(REALITY|TLS)  → 外层入口入站的（含其访问域名）
//	SNI                              → 该条目在 fallback 里的 name
//	协议/传输/凭据/服务名            → 内部入站自己的
//
// 这里只做"识别引用关系 + 合成客户端视图"，不改任何入站配置。

// fallbackEntry 描述"内部入站 ← 外层入口入站"的引用关系。
type fallbackEntry struct {
	Entry *entity.InboundConfig // 外层入口（负责 TLS / REALITY 与对外端口）
	Name  string                // fallback.name：该内部入站对外的 SNI（可能为空）
}

// normalizeFallbackDest 把 fallback.dest / inbound.listen 归一成可比较的地址：
// 去掉 `unix:` 前缀与 `,0666` 权限后缀。
func normalizeFallbackDest(v string) string {
	d := strings.TrimSpace(v)
	d = strings.TrimPrefix(d, "unix:")
	if i := strings.LastIndex(d, ","); i > 0 && isOctalPerm(d[i+1:]) {
		d = d[:i]
	}
	return strings.TrimSpace(d)
}

func isOctalPerm(s string) bool {
	if len(s) < 3 || len(s) > 4 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '7' {
			return false
		}
	}
	return true
}

func fallbackListenKey(serverID int64, coreType, listen string) string {
	return coreType + "|" + strconv.FormatInt(serverID, 10) + "|" + normalizeFallbackDest(listen)
}

// buildFallbackEntryIndex 扫描所有入站的 settings.fallbacks，把 dest 指向的内部入站
// 映射到它对应的外层入口。同一个内部入站被多个入口引用时，优先保留带 name 的那条
// （带 name 才有明确的对外 SNI）。
func buildFallbackEntryIndex(cfgs []entity.InboundConfig) map[int64]fallbackEntry {
	byListen := make(map[string]*entity.InboundConfig, len(cfgs))
	for i := range cfgs {
		key := fallbackListenKey(cfgs[i].ServerID, cfgs[i].CoreType, cfgs[i].Listen)
		if strings.TrimSpace(cfgs[i].Listen) == "" {
			continue
		}
		if _, ok := byListen[key]; !ok {
			byListen[key] = &cfgs[i]
		}
	}
	out := make(map[int64]fallbackEntry)
	for i := range cfgs {
		entry := &cfgs[i]
		cfg := parseConfigMap(entry.ConfigJSON)
		settings := getMap(cfg, "settings")
		if settings == nil {
			continue
		}
		list, _ := settings["fallbacks"].([]interface{})
		for _, raw := range list {
			fb, _ := raw.(map[string]interface{})
			dest := getString(fb, "dest", "")
			if dest == "" {
				continue
			}
			inner := byListen[fallbackListenKey(entry.ServerID, entry.CoreType, dest)]
			if inner == nil || inner.ID == entry.ID {
				continue
			}
			name := strings.TrimSpace(getString(fb, "name", ""))
			if cur, ok := out[inner.ID]; ok && cur.Name != "" && name == "" {
				continue
			}
			out[inner.ID] = fallbackEntry{Entry: entry, Name: name}
		}
	}
	return out
}

// fallbackShareView 按外层入口合成"对外可用"的入站视图：
//   - 传输参数（network / grpcSettings / xhttpSettings / wsSettings …）保留内部入站自己的；
//   - security / tlsSettings / realitySettings 用外层入口的（客户端要做 REALITY/TLS 握手）；
//   - 客户端 SNI 用 fallback.name（空则保留入口自己的 serverName）。
func fallbackShareView(inner, entry map[string]interface{}, sni string) map[string]interface{} {
	if inner == nil || entry == nil {
		return inner
	}
	out := deepCopyMap(inner)
	innerSS := getMap(out, "streamSettings")
	if innerSS == nil {
		innerSS = map[string]interface{}{}
		out["streamSettings"] = innerSS
	}
	for _, k := range []string{"security", "tlsSettings", "realitySettings"} {
		delete(innerSS, k)
	}
	entrySS := getMap(entry, "streamSettings")
	if entrySS != nil {
		if sec := getString(entrySS, "security", "none"); sec != "" && sec != "none" {
			innerSS["security"] = sec
		}
		if ts := getMap(entrySS, "tlsSettings"); ts != nil {
			innerSS["tlsSettings"] = deepCopyMap(ts)
		}
		if rs := getMap(entrySS, "realitySettings"); rs != nil {
			// 服务端私钥/抗量子种子不往客户端配置里带（PublicKey 已单独存在）。
			cp := deepCopyMap(rs)
			delete(cp, "privateKey")
			delete(cp, "mldsa65Seed")
			innerSS["realitySettings"] = cp
		}
	}
	sni = strings.TrimSpace(sni)
	if sni != "" {
		if rs := getMap(innerSS, "realitySettings"); rs != nil {
			rs["serverNames"] = []interface{}{sni}
			rs["serverName"] = sni
			if settings, ok := rs["settings"].([]interface{}); ok && len(settings) > 0 {
				if first, ok := settings[0].(map[string]interface{}); ok {
					first["serverName"] = sni
				}
			}
		}
		if ts := getMap(innerSS, "tlsSettings"); ts != nil {
			ts["serverName"] = sni
		}
	} else {
		// fallback 没配 name（catch-all）：它接收所有"未被命名 fallback 匹配"的 SNI，
		// 所以客户端可用的 SNI = 入口 serverNames 里剩下的（含空串）+ 随机。
		// 命名 fallback 的 SNI 必须剔除，否则客户端会被路由到别的内部入站。
		filterClaimedServerNames(getMap(innerSS, "realitySettings"), claimedFallbackNames(entry))
	}
	// 访问域名（对外地址/端口）属于"入口"的属性：不带上就会退回内部入站的
	// 抽象 socket / 内网端口（客户端根本连不到）。
	if ad, ok := entry["_accessDomain"]; ok {
		out["_accessDomain"] = deepCopyMapAny(ad)
	}
	// sockopt 是服务端监听参数（acceptProxyProtocol 等），客户端不需要。
	delete(innerSS, "sockopt")
	return out
}

// deepCopyMapAny 深拷贝任意 JSON 结构（map / slice / 标量）。
func deepCopyMapAny(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		cp := make(map[string]interface{}, len(t))
		for k, val := range t {
			cp[k] = deepCopyMapAny(val)
		}
		return cp
	case []interface{}:
		cp := make([]interface{}, len(t))
		for i, val := range t {
			cp[i] = deepCopyMapAny(val)
		}
		return cp
	default:
		return v
	}
}

// applyFallbackEntry 若 cfg 是被外层入口 fallback 引用的内部入站，返回
// （入口配置, 合成的对外视图）；否则原样返回。
func (h *PublicHandler) applyFallbackEntry(cfg entity.InboundConfig, config map[string]interface{}, idx map[int64]fallbackEntry) (entity.InboundConfig, map[string]interface{}, map[string]interface{}) {
	e, ok := idx[cfg.ID]
	if !ok || e.Entry == nil {
		// 入口入站自身：REALITY 认证通过后不会再走 fallback（fallback 只在协议头解析失败时
		// 触发），所以任何 serverNames 里的 SNI 都能用，包括 fallback 配置的那些。
		return cfg, config, nil
	}
	entryCfg := parseConfigMap(e.Entry.ConfigJSON)
	if entryCfg == nil {
		return cfg, config, nil
	}
	// 第三个返回值是**入口入站**的配置：对外端口要按入口判断（入口可能只监听
	// unix socket / 要求 PROXY 协议头，只能从前置 SNI 端口进）。
	return *e.Entry, fallbackShareView(config, entryCfg, e.Name), entryCfg
}

// claimedFallbackNames 收集入口入站里"命名 fallback"的 SNI（name 非空）。
// 这些 SNI 只属于对应的内部入站，catch-all（name 为空）的客户端不能用。
func claimedFallbackNames(entry map[string]interface{}) map[string]bool {
	settings := getMap(entry, "settings")
	if settings == nil {
		return nil
	}
	fbs, _ := settings["fallbacks"].([]interface{})
	claimed := make(map[string]bool, len(fbs))
	for _, raw := range fbs {
		fb, _ := raw.(map[string]interface{})
		if name := strings.ToLower(strings.TrimSpace(getString(fb, "name", ""))); name != "" {
			claimed[name] = true
		}
	}
	if len(claimed) == 0 {
		return nil
	}
	return claimed
}

// filterClaimedServerNames 从 realitySettings.serverNames 里剔除被命名 fallback 认领的
// SNI（保留 "" = 允许无 SNI）。用于"没配置 SNI 的 fallback"：它只接收剩余 SNI。
func filterClaimedServerNames(rs map[string]interface{}, claimed map[string]bool) {
	if rs == nil || len(claimed) == 0 {
		return
	}
	var names []string
	switch list := rs["serverNames"].(type) {
	case []interface{}:
		for _, raw := range list {
			if v, ok := raw.(string); ok {
				names = append(names, v)
			}
		}
	case []string:
		names = append(names, list...)
	default:
		return
	}
	kept := make([]interface{}, 0, len(names))
	for _, n := range names {
		if n == "" || !claimed[strings.ToLower(strings.TrimSpace(n))] {
			kept = append(kept, n)
		}
	}
	if len(kept) == 0 {
		kept = []interface{}{""}
	}
	rs["serverNames"] = kept
	// 标量 serverName 可能是被认领的那个 SNI，删掉让随机从列表里挑。
	delete(rs, "serverName")
}
