package warp

import (
	"context"
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// MASQUE 目前只有 xray 实现：注册必须带 core_type=xray，否则会得到一条
// 永远用不上的记录（sing-box 的 masque-client 还不能用于 WARP）。
func TestRegisterMasqueRequiresXray(t *testing.T) {
	svc, _ := newMasqueTestService(t)

	for _, coreType := range []string{"singbox", "", "XRAY-WRONG"} {
		if _, err := svc.RegisterMasque(1, coreType, ""); err == nil {
			t.Errorf("core_type=%q 应被拒绝", coreType)
		} else if !strings.Contains(err.Error(), "仅支持 xray") {
			t.Errorf("core_type=%q 错误信息不明确: %v", coreType, err)
		}
	}
}

// 即使历史数据里存在 core_type=singbox 的 MASQUE 记录，也不能把出站写进
// xray 的 outbound_configs（那是另一个核心的表）。
func TestCreateMasqueOutboundRejectsSingboxRecord(t *testing.T) {
	svc, outRepo := newMasqueTestService(t)
	ctx := context.Background()

	cfg := &entity.WarpConfig{
		ServerID: 6, CoreType: "singbox", Status: "active",
		PrivateKey: "ec-priv", PublicKey: "ec-pub", TunnelType: "masque",
		Endpoint: "162.159.198.2", EndpointPorts: "443",
	}
	if err := svc.repo.Create(ctx, cfg); err != nil {
		t.Fatalf("create masque cfg: %v", err)
	}

	if _, err := svc.CreateMasqueOutbound(6, cfg.ID, MasqueOutboundInput{Tag: "warp-masque"}); err == nil {
		t.Fatalf("sing-box 的 MASQUE 记录不应创建 xray 出站")
	}

	list, err := outRepo.List(ctx, 6, "xray")
	if err != nil {
		t.Fatalf("list outbounds: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("不应写入 xray 出站表，实际 %d 条", len(list))
	}
}
