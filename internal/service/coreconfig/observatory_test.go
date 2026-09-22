package coreconfig

import "testing"

// leastLoad / leastPing 依赖根 observatory，缺了它 xray 启动直接失败
// （实测：`failed to create server > core: not all dependencies are resolved`）。
// 面板路由编辑器能选这两种策略，但 observatory 是顶层键、编辑器产出不到 → 后端兜底补。
func TestEnsureObservatoryForLeastLoad(t *testing.T) {
	// 1) leastLoad + 无 observatory → 自动补，subjectSelector = balancer 的 selector
	cfg := map[string]interface{}{
		"routing": map[string]interface{}{
			"balancers": []interface{}{
				map[string]interface{}{
					"tag":      "b1",
					"selector": []interface{}{"exit-1", "exit-2"},
					"strategy": map[string]interface{}{"type": "leastLoad"},
				},
			},
		},
	}
	ensureObservatoryForLeastLoad(cfg)
	obs, ok := cfg["observatory"].(map[string]interface{})
	if !ok {
		t.Fatal("leastLoad 应自动补 observatory")
	}
	sel := selectorStrings(obs["subjectSelector"])
	if len(sel) != 2 || sel[0] != "exit-1" || sel[1] != "exit-2" {
		t.Errorf("subjectSelector = %v, want [exit-1 exit-2]", sel)
	}
	if obs["probeURL"] == nil || obs["probeInterval"] == nil {
		t.Errorf("observatory 缺 probeURL/probeInterval: %v", obs)
	}

	// 2) 已有 observatory（如链式负载均衡注入的）→ 只合并，不覆盖
	cfg2 := map[string]interface{}{
		"observatory": map[string]interface{}{
			"subjectSelector": []interface{}{"chain-"},
			"probeURL":        "https://already.example/204",
		},
		"routing": map[string]interface{}{
			"balancers": []interface{}{
				map[string]interface{}{
					"selector": []interface{}{"chain-", "exit-9"},
					"strategy": map[string]interface{}{"type": "leastPing"},
				},
			},
		},
	}
	ensureObservatoryForLeastLoad(cfg2)
	obs2 := cfg2["observatory"].(map[string]interface{})
	if obs2["probeURL"] != "https://already.example/204" {
		t.Errorf("不该覆盖已有 observatory 的其他字段: %v", obs2)
	}
	if sel2 := selectorStrings(obs2["subjectSelector"]); len(sel2) != 2 {
		t.Errorf("应去重合并 selector，got %v", sel2)
	}

	// 3) 只有 random 策略 → 不补
	cfg3 := map[string]interface{}{
		"routing": map[string]interface{}{
			"balancers": []interface{}{
				map[string]interface{}{"selector": []interface{}{"a"}, "strategy": map[string]interface{}{"type": "random"}},
			},
		},
	}
	ensureObservatoryForLeastLoad(cfg3)
	if _, ok := cfg3["observatory"]; ok {
		t.Error("random 策略不该补 observatory")
	}
}

// selectorStrings 把 subjectSelector 归一成 []string（可能是 []string 也可能是 []interface{}）。
func selectorStrings(v interface{}) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, e := range t {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
