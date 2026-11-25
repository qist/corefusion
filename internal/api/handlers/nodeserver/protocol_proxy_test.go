package nodeserver

import (
	"encoding/json"
	"strings"
	"testing"

	agentpb "corefusion/internal/runtime/agent/proto"
)

// 远程节点的 proxy/status 必须把 Agent 上报的 capabilities 透传给前端：
// sing-box 的 SNI 提示靠它区分「带 stream_pp_var 补丁的 nginx」和旧版。
func TestTransformProxyStatusIncludesCapabilities(t *testing.T) {
	raw := &agentpb.GetProxyStatusResponse{
		Proxies: []*agentpb.ProxyInfo{
			{
				Type:         "nginx",
				Installed:    true,
				Status:       "running",
				Capabilities: []string{"stream_pp_var", "stream_ssl_preread"},
			},
		},
	}
	resp := transformProxyStatus(raw, nil, 2)
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if !strings.Contains(string(data), `"capabilities":["stream_pp_var","stream_ssl_preread"]`) {
		t.Fatalf("capabilities 未透传：%s", data)
	}
}
