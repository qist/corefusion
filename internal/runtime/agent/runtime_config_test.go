package agent

import (
	"context"
	"strings"
	"testing"

	agentpb "corefusion/internal/runtime/agent/proto"
)

// 老节点升级后靠这个 RPC 补齐 update.public_key；返回的必须是可解析的 ed25519 PEM。
func TestHeartbeatGetRuntimeConfigReturnsSigningKey(t *testing.T) {
	signer, err := LoadOrCreateAgentUpdateSigner(context.Background(), t.TempDir(), nil)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	mgr := &NodeServerManager{updateSigner: signer}
	h := &heartbeatServiceImpl{mgr: mgr}

	resp, err := h.GetRuntimeConfig(context.Background(), &agentpb.GetRuntimeConfigRequest{})
	if err != nil {
		t.Fatalf("rpc: %v", err)
	}
	pem := resp.GetUpdatePublicKey()
	if !strings.Contains(pem, "BEGIN PUBLIC KEY") {
		t.Fatalf("unexpected pem: %q", pem)
	}
	if _, err := ParseUpdatePublicKey([]byte(pem)); err != nil {
		t.Fatalf("pem 无法解析：%v", err)
	}

	// 面板还没注入签名器时返回空（Agent 不会写空配置）
	empty := &heartbeatServiceImpl{mgr: &NodeServerManager{}}
	r2, err := empty.GetRuntimeConfig(context.Background(), &agentpb.GetRuntimeConfigRequest{})
	if err != nil || r2.GetUpdatePublicKey() != "" {
		t.Fatalf("无签名器应返回空：resp=%v err=%v", r2, err)
	}
}
