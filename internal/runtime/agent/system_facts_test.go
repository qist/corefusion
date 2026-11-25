package agent

import (
	"encoding/json"
	"testing"

	"corefusion/internal/model/nodecheckup"
	agentpb "corefusion/internal/runtime/agent/proto"
)

func TestCollectSystemFactsReadOnly(t *testing.T) {
	resp := CollectSystemFacts(&agentpb.GetSystemFactsRequest{IncludeFirewallRules: true, IncludeSsh: true})
	if resp == nil || !resp.GetSuccess() {
		t.Fatalf("采集失败：%+v", resp)
	}
	if len(resp.GetFacts()) > 256*1024 {
		t.Fatalf("facts 超过 256KB：%d", len(resp.GetFacts()))
	}
	var facts nodecheckup.Facts
	if err := json.Unmarshal([]byte(resp.GetFacts()), &facts); err != nil {
		t.Fatalf("facts JSON 无法解析：%v", err)
	}
	if facts.Env.Kind == "" || facts.Env.OS == "" {
		t.Fatalf("环境信息缺失：%+v", facts.Env)
	}
	if len(facts.Sysctl) == 0 {
		t.Fatalf("未采集到 sysctl 白名单")
	}
	// 容器/宿主都必须能判断「可写性」，不能因为 /proc/sys 只读而整体失败。
	for key, f := range facts.Sysctl {
		if f.Scope == "" {
			t.Fatalf("sysctl %s 缺少 scope", key)
		}
	}
}
