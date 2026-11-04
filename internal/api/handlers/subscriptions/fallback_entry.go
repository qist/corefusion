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
func (h *PublicHandler) applyFallbackEntry(cfg entity.InboundConfig, config map[string]interface{}, idx map[int64]fallbackEntry) (entity.InboundConfig, map[string]interface{}) {
	e, ok := idx[cfg.ID]
	if !ok || e.Entry == nil {
		// 入口入站自身：可用 SNI = 未被 fallback 认领的 + 空串。
		return cfg, entryShareView(config)
	}
	entryCfg := parseConfigMap(e.Entry.ConfigJSON)
	if entryCfg == nil {
		return cfg, entryShareView(config)
	}
	return *e.Entry, fallbackShareView(config, entryCfg, e.Name)
}

// entryShareView 返回"入口入站"的客户端视图：把 REALITY serverNames 里被 fallback
// 认领的 SNI 剔除。语义（用户明确）：
//   - 入口入站（如 any_SNI_No_SNI）允许无 SNI，也可用**未被认领**的 SNI；
//   - fallback 上配置的 name 是该内部入站的**唯一**入口 SNI，其它客户端不能拿它当入口。
//
// 因此外层节点的分享只应在"剩余 SNI + 空串"里随机。
func entryShareView(config map[string]interface{}) map[string]interface{} {
	if config == nil {
		return config
	}
	settings := getMap(config, "settings")
	if settings == nil {
		return config
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
		return config
	}
	ss := getMap(config, "streamSettings")
	rs := getMap(ss, "realitySettings")
	if rs == nil {
		return config
	}
	var names []string
	switch list := rs["serverNames"].(type) {
	case []interface{}:
		for _, raw := range list {
			if s, ok := raw.(string); ok {
				names = append(names, s)
			}
		}
	case []string:
		names = append(names, list...)
	default:
		return config
	}
	kept := make([]interface{}, 0, len(names))
	for _, s := range names {
		if s == "" || !claimed[strings.ToLower(strings.TrimSpace(s))] {
			kept = append(kept, s)
		}
	}
	if len(kept) == 0 {
		// 全被 fallback 认领：入口只剩"无 SNI"这一种可用方式。
		kept = []interface{}{""}
	}
	out := deepCopyMap(config)
	outRS := getMap(getMap(out, "streamSettings"), "realitySettings")
	outRS["serverNames"] = kept
	// 标量 serverName 是旧值（可能正是被认领的 SNI），删掉让随机从列表里挑。
	delete(outRS, "serverName")
	return out
}
