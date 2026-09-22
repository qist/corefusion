package agent

import (
	"testing"

	"corefusion/internal/model/entity"
)

// coreOfFixture 模拟"入口入站 → 所属核心"的查表：
// 入站 4 是 xray 的，入站 118 是 sing-box 的（同机双核心的真实场景）。
func coreOfFixture(id int64) string {
	switch id {
	case 4:
		return "xray"
	case 118:
		return "singbox"
	}
	return "xray" // entryCoreType 的默认值
}

// TestFilterLinksByEntryCoreKeepsOnlyMatchingCore 回归"节点起不来"那个事故：
// 生成**某个核心**的配置时，不能带上"入口入站属于另一个核心"的链 ——
// 否则入口是 hysteria2 的链会把 hy2 出站塞进 xray 配置 → xray 直接拒绝启动
// （实测：`failed to build outbound config with tag chain-118-1 > … unknown config id: hysteria2`）。
func TestFilterLinksByEntryCoreKeepsOnlyMatchingCore(t *testing.T) {
	links := []entity.ChainLink{
		{EntryInboundID: 4, ExitServerID: 2, ExitInboundID: 76},    // 入口是 xray
		{EntryInboundID: 118, ExitServerID: 1, ExitInboundID: 120}, // 入口是 sing-box（hy2 出口）
	}

	xrayLinks := filterLinksByEntryCore(links, "xray", coreOfFixture)
	if len(xrayLinks) != 1 || xrayLinks[0].EntryInboundID != 4 {
		t.Errorf("xray 只应保留入口为 xray 的链，得到 %+v", xrayLinks)
	}

	sbLinks := filterLinksByEntryCore(links, "singbox", coreOfFixture)
	if len(sbLinks) != 1 || sbLinks[0].EntryInboundID != 118 {
		t.Errorf("sing-box 只应保留入口为 sing-box 的链，得到 %+v", sbLinks)
	}

	// 反向确认：这才是事故的直接判据 —— sing-box 入口的链（hy2 出口）绝不能被 xray 拿到
	for _, l := range xrayLinks {
		if l.EntryInboundID == 118 {
			t.Fatal("入口为 sing-box 的链泄漏进了 xray 配置 —— 这正是让 xray 起不来的原因")
		}
	}
}

// 空 coreType（老调用方/未指定）→ 原样返回，保持兼容。
func TestFilterLinksByEntryCorePassthrough(t *testing.T) {
	links := []entity.ChainLink{{EntryInboundID: 4}, {EntryInboundID: 118}}
	if got := filterLinksByEntryCore(links, "", coreOfFixture); len(got) != 2 {
		t.Errorf("coreType 为空时应原样返回，得到 %+v", got)
	}
	if got := filterLinksByEntryCore(nil, "xray", coreOfFixture); got != nil {
		t.Errorf("空切片应原样返回，得到 %+v", got)
	}
}

// 同一入口入站都属于目标核心时全部保留（多出口场景）。
func TestFilterLinksByEntryCoreKeepsAllExitsOfSameEntry(t *testing.T) {
	links := []entity.ChainLink{
		{EntryInboundID: 4, ExitServerID: 2, ExitInboundID: 76},
		{EntryInboundID: 4, ExitServerID: 3, ExitInboundID: 77},
	}
	if got := filterLinksByEntryCore(links, "xray", coreOfFixture); len(got) != 2 {
		t.Errorf("同一入口的多个出口都应保留，得到 %+v", got)
	}
}
