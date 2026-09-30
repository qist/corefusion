package ratelimit

import (
	"fmt"
	"log"
	"net"
	"os/exec"
	"strings"
)

// ============================================================
// 面板级 IP 黑名单（节点核心入站阻断）
//
// 运维在面板安全设置维护 panel.ip_blacklist → 面板面（订阅拉取/门户）由
// 面板中间件拦截；**已拿到节点连接信息的客户端直连核心**则由本组件兜底：
// 在 nft 表 corefusion_ratelimit 的 blocked_chain（priority -160，早于 mark/计数链）
// 里维护两个 interval 集合 + drop 规则，对核心入站流量按源地址阻断。
//
// 设计要点：
//   - **不依赖** ratelimit.Manager/Backend 初始化（限速未启用的节点也能用黑名单），
//     表/链/集合全部按需幂等创建（已存在只记 note，与 initNftTable 同口径）；
//   - 集合原子刷新：flush set → 一次 add element 批量写入；
//   - 规则常驻：blocked_chain 不参与 initNftTable 的 flush，drop 规则跨 Agent
//     重启存活；节点重装/重启后由面板 on-connect / 设置变更重新下发。
// ============================================================

const (
	nftBlacklistSet  = "ip_blacklist"  // IPv4 interval 集合（typeof ip saddr）
	nftBlacklistSet6 = "ip_blacklist6" // IPv6 interval 集合（typeof ip6 saddr）
)

// ApplyIPBlacklist 将黑名单列表应用到本节点 nft（entries 为空 = 清空）。
// entry 支持单 IP 与 CIDR；非法条目忽略（面板侧已校验，这里是最后防线）。
func ApplyIPBlacklist(entries []string) error {
	// ① 幂等创建表 / blocked 链（限速后端可能从未初始化）
	if err := runCmd("nft", "add", "table", "inet", nftTable); err != nil {
		log.Printf("[IPBlacklist] nft table create note: %v (may already exist)", err)
	}
	blockChainDef := "{ type filter hook prerouting priority -160 ; policy accept ; }"
	if err := runCmd("nft", "add", "chain", "inet", nftTable, nftBlockChain, blockChainDef); err != nil {
		log.Printf("[IPBlacklist] nft blocked chain create note: %v (may already exist)", err)
	}
	// ② 幂等创建 v4/v6 interval 集合。
	// ⚠ 用**显式类型**（type ipv4_addr/ipv6_addr）而非 typeof —— 老版 nft（CentOS8 的
	// 0.9.0）对 "typeof ip saddr + interval" 组合直接报错 → 集合建不出来，flush 才暴露
	// "No such file or directory"（真机踩过 ✓）。ipv4_addr/ipv6_addr 是经典类型全版本支持。
	// （connacct 集合用 typeof 是因为 meta mark 没有等价的经典类型，与本处不同。）
	if err := runCmd("nft", "add", "set", "inet", nftTable, nftBlacklistSet,
		"{", "type", "ipv4_addr", ";", "flags", "interval", ";", "}"); err != nil {
		log.Printf("[IPBlacklist] nft set create note: %v (may already exist)", err)
	}
	if err := runCmd("nft", "add", "set", "inet", nftTable, nftBlacklistSet6,
		"{", "type", "ipv6_addr", ";", "flags", "interval", ";", "}"); err != nil {
		log.Printf("[IPBlacklist] nft set6 create note: %v (may already exist)", err)
	}
	// ③ 集合必须真实存在（"may already exist" 的 note 也可能是真失败 —— 校验兜底）
	for _, set := range []string{nftBlacklistSet, nftBlacklistSet6} {
		if out, err := exec.Command("nft", "list", "set", "inet", nftTable, set).Output(); err != nil || len(out) == 0 {
			return fmt.Errorf("nft set %s unavailable (create failed?): %v", set, err)
		}
	}
	// ④ 幂等挂 drop 规则（先按匹配删旧再挂，防重复累积；drop 早于一切 mark/计数）
	ensureBlacklistDropRule(nftBlacklistSet, "ip")
	ensureBlacklistDropRule(nftBlacklistSet6, "ip6")

	// ⑤ 拆分 v4/v6 并批量写入集合（flush → 一次 add element）
	var v4, v6 []string
	for _, e := range entries {
		entry := strings.TrimSpace(e)
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		ipStr := entry
		if strings.Contains(entry, "/") {
			// CIDR：net.ParseCIDR 校验合法性
			if _, _, err := net.ParseCIDR(entry); err != nil {
				log.Printf("[IPBlacklist] skip invalid CIDR %q: %v", entry, err)
				continue
			}
		} else if ip := net.ParseIP(entry); ip == nil {
			log.Printf("[IPBlacklist] skip invalid IP %q", entry)
			continue
		} else {
			ipStr = ip.String()
		}
		if strings.Contains(ipStr, ":") {
			v6 = append(v6, ipStr)
		} else {
			v4 = append(v4, ipStr)
		}
	}

	if err := flushAndAddElements(nftBlacklistSet, v4); err != nil {
		return fmt.Errorf("apply ipv4 blacklist: %w", err)
	}
	if err := flushAndAddElements(nftBlacklistSet6, v6); err != nil {
		return fmt.Errorf("apply ipv6 blacklist: %w", err)
	}
	log.Printf("[IPBlacklist] applied %d ipv4 + %d ipv6 entries to nft (%s.%s/%s)",
		len(v4), len(v6), nftTable, nftBlockChain, nftBlacklistSet)
	return nil
}

// ensureBlacklistDropRule 幂等挂 `ip[saddr] @set counter drop` 规则到 blocked_chain。
func ensureBlacklistDropRule(setName, family string) {
	// 删掉本集合的旧 drop 规则（按 "@集合名" 匹配 handle）
	out, err := exec.Command("nft", "-a", "list", "chain", "inet", nftTable, nftBlockChain).Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if !strings.Contains(line, "@"+setName) {
				continue
			}
			handleIdx := strings.LastIndex(line, "handle ")
			if handleIdx < 0 {
				continue
			}
			handleStr := strings.TrimSpace(line[handleIdx+7:])
			runCmd("nft", "delete", "rule", "inet", nftTable, nftBlockChain, "handle", handleStr)
		}
	}
	if err := runCmd("nft", "add", "rule", "inet", nftTable, nftBlockChain,
		family, "saddr", "@"+setName, "counter", "drop"); err != nil {
		log.Printf("[IPBlacklist] add drop rule for %s failed: %v", setName, err)
	}
}

// flushAndAddElements 清空集合并批量写入新元素（entries 为空只 flush）。
// nft 会把 argv 用空格拼接后解析，花括号列表作为单个参数传入即可。
func flushAndAddElements(setName string, entries []string) error {
	if err := runCmd("nft", "flush", "set", "inet", nftTable, setName); err != nil {
		return fmt.Errorf("flush set: %w", err)
	}
	if len(entries) == 0 {
		return nil
	}
	list := "{ " + strings.Join(entries, ", ") + " }"
	if err := runCmd("nft", "add", "element", "inet", nftTable, setName, list); err != nil {
		return fmt.Errorf("add %d elements: %w", len(entries), err)
	}
	return nil
}
