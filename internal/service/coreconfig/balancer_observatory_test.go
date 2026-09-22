package coreconfig

import (
	"encoding/json"
	"os"
	"testing"
)

// TestEnsureBalancerObservatory 覆盖"面板只配了策略、没配 observatory"这条很容易踩的路径：
// xray 对 leastPing/leastLoad 的 balancer **强制要求** observatory，缺了直接拒绝启动
// （`failed to build routing configuration > ... requires observatory`）。
// 产物 dump 到 /tmp/balancer_obs.json 供 `xray run -test` 做真实核心校验。
func TestEnsureBalancerObservatory(t *testing.T) {
	newCfg := func(extra map[string]interface{}) map[string]interface{} {
		cfg := map[string]interface{}{
			"log": map[string]interface{}{"loglevel": "warning"},
			"inbounds": []interface{}{
				map[string]interface{}{"listen": "127.0.0.1", "port": float64(45991), "protocol": "socks",
					"settings": map[string]interface{}{"auth": "noauth"}},
			},
			"outbounds": []interface{}{
				map[string]interface{}{"tag": "chain-1-2", "protocol": "freedom", "settings": map[string]interface{}{}},
				map[string]interface{}{"tag": "chain-1-3", "protocol": "freedom", "settings": map[string]interface{}{}},
			},
			"routing": map[string]interface{}{
				"balancers": []interface{}{
					map[string]interface{}{
						"tag":      "bal-chain-1",
						"selector": []interface{}{"chain-1-"},
						"strategy": map[string]interface{}{"type": "leastLoad"},
					},
				},
			},
		}
		for k, v := range extra {
			cfg[k] = v
		}
		return cfg
	}

	// ① 缺 observatory → 应补上，且 subjectSelector 取自 balancer 的 selector
	cfg := newCfg(nil)
	if !EnsureBalancerObservatory(cfg) {
		t.Fatalf("应当补出 observatory")
	}
	obs, _ := cfg["observatory"].(map[string]interface{})
	if obs == nil {
		t.Fatalf("observatory 未写入")
	}
	if sel, _ := obs["subjectSelector"].([]interface{}); len(sel) != 1 || sel[0] != "chain-1-" {
		t.Errorf("subjectSelector 应取自 balancer.selector，实际 %v", obs["subjectSelector"])
	}
	if obs["probeUrl"] == nil || obs["probeInterval"] == nil {
		t.Errorf("observatory 缺少必要字段: %v", obs)
	}

	// ② 已配 observatory → 不覆盖（尊重用户配置）
	cfg2 := newCfg(map[string]interface{}{
		"observatory": map[string]interface{}{"subjectSelector": []interface{}{"custom"}, "probeUrl": "https://example.com"},
	})
	if EnsureBalancerObservatory(cfg2) {
		t.Errorf("已有 observatory 时不应重复注入")
	}
	if got := cfg2["observatory"].(map[string]interface{})["probeUrl"]; got != "https://example.com" {
		t.Errorf("用户配置被覆盖了: %v", got)
	}

	// ③ 策略是 roundRobin → 不需要 observatory
	cfg3 := newCfg(nil)
	cfg3["routing"].(map[string]interface{})["balancers"].([]interface{})[0].(map[string]interface{})["strategy"] = map[string]interface{}{"type": "roundRobin"}
	if EnsureBalancerObservatory(cfg3) {
		t.Errorf("roundRobin 不需要 observatory")
	}

	// ④ 没有 balancers → 什么都不做
	cfg4 := newCfg(nil)
	delete(cfg4["routing"].(map[string]interface{}), "balancers")
	if EnsureBalancerObservatory(cfg4) {
		t.Errorf("无 balancer 时不应注入")
	}

	b, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile("/tmp/balancer_obs.json", b, 0o644); err != nil {
		t.Logf("dump 失败（不影响断言）: %v", err)
	}
}
