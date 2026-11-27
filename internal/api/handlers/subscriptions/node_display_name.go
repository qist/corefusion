package subscriptions

import (
	"strconv"
	"strings"
)

// nodeDisplayName 生成订阅/分享里给用户看的节点名。
//
// mihomo 的 name、sing-box/xray JSON 的 outbound tag、分享链接的 `#fragment`
// 用的都是它，所以这里同时决定了「客户端列表里显示什么」和「tag 里带什么」。
//
// 为什么不能直接用 Tag 兜底：入站 tag 的约定是 `inbound-<协议>-<端口>`
// （前端 genInboundTag），xray-ui 迁移过来的入站 remark 往往为空、
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
	name := strings.TrimSpace(remark)
	if name != "" && !isAutoInboundName(name) {
		// remark 可以是任意语言（中文/日文/俄文/波斯文…），原样保留。
		return name
	}
	if name != "" {
		// remark 本身是自动名（inbound-<协议>-<端口>）：按 tag 脱敏，端口不展示。
		return portFreeNameFromTag(name)
	}
	return portFreeNameFromTag(tag)
}

// isAutoInboundName 判断 remark 是否是前端/迁移脚本自动生成的入站名
// （`inbound-<协议>-<端口>`）。只有这种 ASCII 形式才脱敏；
// 管理员起的任何语言的名字都不会被改动。
func isAutoInboundName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if !strings.HasPrefix(lower, "inbound-") {
		return false
	}
	parts := strings.Split(lower, "-")
	if len(parts) < 3 {
		return false
	}
	return isAllDigits(parts[len(parts)-1])
}

// newNodeNameRegistry 返回节点名登记表，并预先占用订阅生成器保留的出站 tag：
// proxy / direct / block。节点如果正好叫这些名字，会被改成 proxy-2 之类，
// 保证面板注入的 `tag: proxy` 组永远存在——否则模板里的 route.final / 规则引用
// 会变成悬空引用，用户还得手动改配置。
func newNodeNameRegistry() map[string]int {
	return map[string]int{
		"proxy":  1,
		"direct": 1,
		"block":  1,
	}
}

// uniqueNodeName 保证同一份订阅里的节点名唯一：重名时追加 -2/-3…
// 原因：sing-box / xray 的 outbound tag 重名会整份配置拒载；mihomo 也会报
// duplicate name。remark 是任意语言，只做后缀，不做任何语言相关的改写。
func uniqueNodeName(used map[string]int, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "node"
	}
	if used[name] == 0 {
		used[name] = 1
		return name
	}
	for i := 2; ; i++ {
		candidate := name + "-" + strconv.Itoa(i)
		if used[candidate] == 0 {
			used[candidate] = 1
			return candidate
		}
	}
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
