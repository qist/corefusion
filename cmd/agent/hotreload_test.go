package main

import (
	"testing"
)

const baseConfig = `{
	"log": {"loglevel": "warning"},
	"inbounds": [
		{"tag": "in-a", "port": 10001, "protocol": "vless", "settings": {"clients": [{"id": "u1"}]}},
		{"tag": "in-b", "port": 10002, "protocol": "trojan", "settings": {"clients": [{"password": "p1"}]}}
	],
	"outbounds": [{"tag": "direct", "protocol": "freedom"}]
}`

// 配置完全一致 → 没有任何需要处理的差异（周期同步应直接跳过，不碰进程）
func TestDiffRuntimeConfigUnchanged(t *testing.T) {
	d, err := diffRuntimeConfig([]byte(baseConfig), []byte(baseConfig))
	if err != nil {
		t.Fatal(err)
	}
	if !d.empty() {
		t.Fatalf("期望无差异，实际 %+v", d)
	}
}

// 仅键序/空白不同 → 语义一致，仍应判定为无差异
func TestDiffRuntimeConfigSemanticEquality(t *testing.T) {
	reordered := `{
		"inbounds": [
			{"protocol": "vless", "tag": "in-a", "settings": {"clients": [{"id": "u1"}]}, "port": 10001},
			{"tag": "in-b", "protocol": "trojan", "settings": {"clients": [{"password": "p1"}]}, "port": 10002}
		],
		"outbounds": [{"protocol": "freedom", "tag": "direct"}],
		"log": {"loglevel": "warning"}
	}`
	d, err := diffRuntimeConfig([]byte(baseConfig), []byte(reordered))
	if err != nil {
		t.Fatal(err)
	}
	if !d.empty() {
		t.Fatalf("键序差异不应产生差异，实际 %+v", d)
	}
}

// 入站的增 / 删 / 改都要被识别为可热应用
func TestDiffRuntimeConfigInboundChanges(t *testing.T) {
	oldCfg := `{
		"inbounds": [
			{"tag": "keep", "port": 1, "protocol": "vless"},
			{"tag": "removed", "port": 2, "protocol": "vless"},
			{"tag": "changed", "port": 3, "protocol": "vless", "settings": {"clients": [{"id": "old"}]}}
		]
	}`
	newCfg := `{
		"inbounds": [
			{"tag": "keep", "port": 1, "protocol": "vless"},
			{"tag": "changed", "port": 3, "protocol": "vless", "settings": {"clients": [{"id": "new"}]}},
			{"tag": "added", "port": 4, "protocol": "vless"}
		]
	}`

	d, err := diffRuntimeConfig([]byte(oldCfg), []byte(newCfg))
	if err != nil {
		t.Fatal(err)
	}
	if d.Unsupported {
		t.Fatalf("仅入站差异应可热应用: %s", d.OutsideReason)
	}
	if len(d.Removals) != 1 || d.Removals[0] != "removed" {
		t.Fatalf("Removals 不符: %v", d.Removals)
	}
	if len(d.Additions) != 1 || d.Additions[0].Tag != "added" {
		t.Fatalf("Additions 不符: %v", d.Additions)
	}
	if len(d.Alters) != 1 || d.Alters[0].Tag != "changed" {
		t.Fatalf("Alters 不符: %v", d.Alters)
	}
}

// routing 变更：RouterService.AddRule + ShouldAppend=false 可整体替换 → 热应用
func TestDiffRuntimeConfigRoutingHotAppliable(t *testing.T) {
	newCfg := `{
		"log": {"loglevel": "warning"},
		"inbounds": [
			{"tag": "in-a", "port": 10001, "protocol": "vless"},
			{"tag": "in-b", "port": 10002, "protocol": "trojan"}
		],
		"outbounds": [{"tag": "direct", "protocol": "freedom"}],
		"routing": {"rules": [{"type": "field", "outboundTag": "direct"}]}
	}`
	d, err := diffRuntimeConfig([]byte(baseConfig), []byte(newCfg))
	if err != nil {
		t.Fatal(err)
	}
	if d.Unsupported {
		t.Fatalf("routing 变更应可热应用: %s", d.OutsideReason)
	}
	if d.Routing == nil {
		t.Fatal("应识别出 routing 需要替换")
	}
}

// routing 段被删除 → 也要热应用（用空配置清空规则），不能留下旧规则
func TestDiffRuntimeConfigRoutingRemovedHotAppliable(t *testing.T) {
	oldCfg := `{
		"inbounds": [{"tag": "in-a", "port": 10001, "protocol": "vless"}],
		"routing": {"rules": [{"type": "field", "outboundTag": "direct"}]}
	}`
	newCfg := `{
		"inbounds": [{"tag": "in-a", "port": 10001, "protocol": "vless"}]
	}`
	d, err := diffRuntimeConfig([]byte(oldCfg), []byte(newCfg))
	if err != nil {
		t.Fatal(err)
	}
	if d.Unsupported || d.Routing == nil {
		t.Fatalf("routing 删除应产生空配置替换: unsupported=%v routing=%s", d.Unsupported, d.Routing)
	}
}

// outbounds 变更：可热应用（改 = 先删后加，增/删按 tag）
func TestDiffRuntimeConfigOutboundHotAppliable(t *testing.T) {
	oldCfg := `{
		"outbounds": [
			{"tag": "keep", "protocol": "freedom"},
			{"tag": "removed", "protocol": "blackhole"},
			{"tag": "changed", "protocol": "freedom", "settings": {"domainStrategy": "AsIs"}}
		]
	}`
	newCfg := `{
		"outbounds": [
			{"tag": "keep", "protocol": "freedom"},
			{"tag": "changed", "protocol": "freedom", "settings": {"domainStrategy": "UseIP"}},
			{"tag": "added", "protocol": "blackhole"}
		]
	}`
	d, err := diffRuntimeConfig([]byte(oldCfg), []byte(newCfg))
	if err != nil {
		t.Fatal(err)
	}
	if d.Unsupported {
		t.Fatalf("出站差异应可热应用: %s", d.OutsideReason)
	}
	if len(d.OutboundRemovals) != 1 || d.OutboundRemovals[0] != "removed" {
		t.Fatalf("OutboundRemovals 不符: %v", d.OutboundRemovals)
	}
	if len(d.OutboundAdditions) != 1 || d.OutboundAdditions[0].Tag != "added" {
		t.Fatalf("OutboundAdditions 不符: %v", d.OutboundAdditions)
	}
	if len(d.OutboundAlters) != 1 || d.OutboundAlters[0].Tag != "changed" {
		t.Fatalf("OutboundAlters 不符: %v", d.OutboundAlters)
	}
}

// dns 等 API 覆盖不到的段 → 仍必须重启
func TestDiffRuntimeConfigDNSRequiresRestart(t *testing.T) {
	newCfg := `{
		"log": {"loglevel": "warning"},
		"inbounds": [
			{"tag": "in-a", "port": 10001, "protocol": "vless"},
			{"tag": "in-b", "port": 10002, "protocol": "trojan"}
		],
		"outbounds": [{"tag": "direct", "protocol": "freedom"}],
		"dns": {"servers": ["1.1.1.1"]}
	}`
	d, err := diffRuntimeConfig([]byte(baseConfig), []byte(newCfg))
	if err != nil {
		t.Fatal(err)
	}
	if !d.Unsupported {
		t.Fatal("dns 变更必须判定为需要重启")
	}
}

// 无 tag 的出站/入站无法通过 API 定位：有变化时必须重启（不能漏应用）
func TestDiffRuntimeConfigUntaggedRequiresRestart(t *testing.T) {
	oldCfg := `{"inbounds":[{"port":1,"protocol":"vless"}]}`
	newCfg := `{"inbounds":[{"port":2,"protocol":"vless"}]}`
	d, err := diffRuntimeConfig([]byte(oldCfg), []byte(newCfg))
	if err != nil {
		t.Fatal(err)
	}
	if !d.Unsupported {
		t.Fatal("无 tag 条目变化必须判定为需要重启")
	}
}

// 没有旧配置（首次写入）→ 无法 diff，交给调用方重启兜底
func TestDiffRuntimeConfigNoOldConfig(t *testing.T) {
	d, err := diffRuntimeConfig(nil, []byte(baseConfig))
	if err != nil {
		t.Fatal(err)
	}
	if !d.Unsupported {
		t.Fatal("缺旧配置时应判定为需要重启")
	}
}
