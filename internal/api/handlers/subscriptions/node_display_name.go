package subscriptions

import "strings"

// nodeDisplayName 生成订阅/分享里给用户看的节点名。
//
// mihomo 的 name、sing-box/xray JSON 的 outbound tag、分享链接的 `#fragment`
// 用的都是它，所以这里同时决定了「客户端列表里显示什么」和「tag 里带什么」。
//
// 为什么不能直接用 Tag 兜底：入站 tag 的约定是 `inbound-<协议>-<端口>`
// （前端 genInboundTag），xray-ui / 3x-ui 迁移过来的入站 remark 往往为空、
// tag 形如 `inbound-20001` 或 `in-443-tcp`。直接拿来当节点名，会把**入站真实
// 监听端口**暴露给所有订阅用户 —— 尤其是 nginx / unix socket 入站：对外是 443，
// tag 里还留着 20001。
//
// 规则：
//  1. Remark 非空 → 原样使用（那是管理员显式起的名字，含端口也是他自愿的）；
//  2. Remark 为空 → 从 tag 去掉端口段（`inbound-vless-20001` → `vless`、
//     `in-443-tcp` → `in-tcp`）；
//  3. 仍然为空 → `node`。
func nodeDisplayName(remark, tag string) string {
	if name := strings.TrimSpace(remark); name != "" {
		return name
	}
	return portFreeNameFromTag(tag)
}

// portFreeNameFromTag 把入站 tag 转成不含端口的展示名。
func portFreeNameFromTag(tag string) string {
	t := strings.TrimSpace(tag)
	if t == "" {
		return "node"
	}
	// 标准入站 tag 形如 inbound-<协议>-<端口>：前缀与端口都没有展示价值。
	if len(t) > len("inbound-") && strings.EqualFold(t[:len("inbound-")], "inbound-") {
		t = t[len("inbound-"):]
	}

	fields := strings.FieldsFunc(t, func(r rune) bool {
		switch r {
		case '-', '_', ':', '.', '/', ' ', '\t':
			return true
		}
		return false
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f == "" || isAllDigits(f) {
			continue
		}
		out = append(out, f)
	}
	if len(out) == 0 {
		return "node"
	}
	return strings.Join(out, "-")
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
