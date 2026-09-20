package subscriptions

import "testing"

// 端口跳跃：入站只配了 _portHop（没有 finalmask）时，分享链接也要带上 mport，
// 否则客户端不会在端口段内跳跃。同时确认 manual 模式用 start-end。
func TestFinalMaskWithPortHop(t *testing.T) {
	cases := []struct {
		name      string
		config    map[string]interface{}
		wantPorts string
	}{
		{
			name:      "preset 模式",
			config:    map[string]interface{}{"_portHop": map[string]interface{}{"enabled": true, "mode": "preset", "range": "20000-50000"}},
			wantPorts: "20000-50000",
		},
		{
			name:      "manual 模式 → start-end",
			config:    map[string]interface{}{"_portHop": map[string]interface{}{"enabled": true, "mode": "manual", "start": float64(30000), "end": float64(40000)}},
			wantPorts: "30000-40000",
		},
		{
			name:      "未启用 → 不出 mport",
			config:    map[string]interface{}{"_portHop": map[string]interface{}{"enabled": false, "mode": "preset", "range": "20000-50000"}},
			wantPorts: "",
		},
		{
			name:      "没有 _portHop → 不出 mport",
			config:    map[string]interface{}{"type": "hysteria2"},
			wantPorts: "",
		},
		{
			name: "已有 finalmask.udpHop 时保持原样（不覆盖）",
			config: map[string]interface{}{
				"finalmask": map[string]interface{}{"quicParams": map[string]interface{}{"udpHop": map[string]interface{}{"ports": "10000-20000"}}},
				"_portHop":  map[string]interface{}{"enabled": true, "mode": "preset", "range": "40000-50000"},
			},
			wantPorts: "10000-20000",
		},
	}

	for _, tc := range cases {
		fm := finalMaskWithPortHop(tc.config)
		got := ""
		if fm != nil {
			if qp, ok := fm["quicParams"].(map[string]interface{}); ok {
				if hop, ok := qp["udpHop"].(map[string]interface{}); ok {
					got, _ = hop["ports"].(string)
				}
			}
		}
		if got != tc.wantPorts {
			t.Errorf("%s: mport = %q, want %q", tc.name, got, tc.wantPorts)
		}
	}
}
