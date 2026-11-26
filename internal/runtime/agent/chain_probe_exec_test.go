package agent

import (
	"strings"
	"testing"

	agentpb "corefusion/internal/runtime/agent/proto"
)

func TestAssignChainProbePorts(t *testing.T) {
	cases := []struct {
		dialer string
		conf   map[string]interface{}
		field  string
	}{
		{"xray", map[string]interface{}{"inbounds": []interface{}{
			map[string]interface{}{"port": float64(0)},
			map[string]interface{}{"port": float64(0)},
		}}, "port"},
		{"singbox", map[string]interface{}{"inbounds": []interface{}{
			map[string]interface{}{"listen_port": float64(0)},
		}}, "listen_port"},
		{"mihomo", map[string]interface{}{"listeners": []interface{}{
			map[string]interface{}{"port": float64(0)},
		}}, "port"},
	}
	for _, c := range cases {
		ports := make([]int, chainProbeListenerCount(c.conf, c.dialer))
		for i := range ports {
			ports[i] = 21000 + i
		}
		if err := assignChainProbePorts(c.conf, c.dialer, ports); err != nil {
			t.Fatalf("%s assign: %v", c.dialer, err)
		}
		key := "inbounds"
		if c.dialer == "mihomo" {
			key = "listeners"
		}
		list := c.conf[key].([]interface{})
		for i, item := range list {
			m := item.(map[string]interface{})
			got, _ := m[c.field].(int)
			if got != ports[i] {
				t.Fatalf("%s item %d = %v, want %d", c.dialer, i, m[c.field], ports[i])
			}
		}
	}
}

func TestAssignChainProbePortsMismatch(t *testing.T) {
	conf := map[string]interface{}{"inbounds": []interface{}{map[string]interface{}{"port": float64(0)}}}
	if err := assignChainProbePorts(conf, "xray", []int{1, 2}); err == nil {
		t.Fatal("端口数不一致应报错")
	}
}

func TestRunChainProbeRejectsBadInput(t *testing.T) {
	a := &LocalAgent{}
	if _, err := a.RunChainProbe(nil, &agentpb.ChainProbeRequest{Dialer: "nope", ConfigJson: "{}"}); err == nil {
		t.Fatal("未知 dialer 应报错")
	}
	if _, err := a.RunChainProbe(nil, &agentpb.ChainProbeRequest{Dialer: "xray", ConfigJson: " "}); err == nil {
		t.Fatal("空配置应报错")
	}
}

func TestChainProbeListenerCount(t *testing.T) {
	conf := map[string]interface{}{
		"inbounds":  []interface{}{map[string]interface{}{}, map[string]interface{}{}},
		"listeners": []interface{}{map[string]interface{}{}},
	}
	if n := chainProbeListenerCount(conf, "xray"); n != 2 {
		t.Fatalf("xray inbounds=%d, want 2", n)
	}
	if n := chainProbeListenerCount(conf, "mihomo"); n != 1 {
		t.Fatalf("mihomo listeners=%d, want 1", n)
	}
	if !strings.Contains("mihomo", "mihomo") {
		t.Fatal("sanity")
	}
}
