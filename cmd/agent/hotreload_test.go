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

// routing 变更：xray API 无法整体替换 → 必须重启
func TestDiffRuntimeConfigRoutingRequiresRestart(t *testing.T) {
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
	if !d.Unsupported {
		t.Fatal("routing 变更必须判定为需要重启")
	}
}

// outbounds 变更：与 routing 引用耦合，保守处理为需要重启
func TestDiffRuntimeConfigOutboundRequiresRestart(t *testing.T) {
	newCfg := `{
		"log": {"loglevel": "warning"},
		"inbounds": [
			{"tag": "in-a", "port": 10001, "protocol": "vless"},
			{"tag": "in-b", "port": 10002, "protocol": "trojan"}
		],
		"outbounds": [{"tag": "direct", "protocol": "blackhole"}]
	}`
	d, err := diffRuntimeConfig([]byte(baseConfig), []byte(newCfg))
	if err != nil {
		t.Fatal(err)
	}
	if !d.Unsupported {
		t.Fatal("outbounds 变更必须判定为需要重启")
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
