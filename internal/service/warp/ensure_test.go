package warp

import (
	"context"
	"testing"

	"corefusion/internal/model/entity"
)

// xray 与 sing-box 不能共用同一条 WARP 记录：即使「所有核心」（core_type 为空）
// 的历史记录 id 更大，也必须选类型精确匹配的那条。
func TestActiveWarpConfigMatchesCoreType(t *testing.T) {
	svc, _ := newMasqueTestService(t)
	ctx := context.Background()

	xrayCfg := &entity.WarpConfig{
		ServerID: 1, CoreType: "xray", Status: "registered",
		PrivateKey: "xray-priv", PublicKey: "xray-pub", TunnelType: "wireguard",
	}
	if err := svc.repo.Create(ctx, xrayCfg); err != nil {
		t.Fatalf("create xray cfg: %v", err)
	}
	// 后插入的空类型记录（历史「所有核心」），id 更大
	anyCfg := &entity.WarpConfig{
		ServerID: 1, CoreType: "", Status: "active",
		PrivateKey: "any-priv", PublicKey: "any-pub", TunnelType: "wireguard",
	}
	if err := svc.repo.Create(ctx, anyCfg); err != nil {
		t.Fatalf("create any cfg: %v", err)
	}
	// MASQUE 记录也不能被 WireGuard 路径选中
	masqueCfg := &entity.WarpConfig{
		ServerID: 1, CoreType: "xray", Status: "active",
		PrivateKey: "ec-priv", PublicKey: "ec-pub", TunnelType: "masque",
	}
	if err := svc.repo.Create(ctx, masqueCfg); err != nil {
		t.Fatalf("create masque cfg: %v", err)
	}

	got, err := svc.activeWarpConfig(ctx, 1, "xray")
	if err != nil {
		t.Fatalf("activeWarpConfig: %v", err)
	}
	if got.ID != xrayCfg.ID {
		t.Fatalf("应选中 xray 的 WireGuard 记录 id=%d，实际 id=%d (core=%q tunnel=%q)",
			xrayCfg.ID, got.ID, got.CoreType, got.TunnelType)
	}
}

func TestActiveMasqueConfigMatchesCoreType(t *testing.T) {
	svc, _ := newMasqueTestService(t)
	ctx := context.Background()

	masqueCfg := &entity.WarpConfig{
		ServerID: 2, CoreType: "xray", Status: "active",
		PrivateKey: "ec-priv", PublicKey: "ec-pub", TunnelType: "masque",
	}
	if err := svc.repo.Create(ctx, masqueCfg); err != nil {
		t.Fatalf("create masque cfg: %v", err)
	}
	// 空类型的 MASQUE 记录不应被 xray 自动附加复用
	anyMasque := &entity.WarpConfig{
		ServerID: 2, CoreType: "", Status: "active",
		PrivateKey: "ec-priv2", PublicKey: "ec-pub2", TunnelType: "masque",
	}
	if err := svc.repo.Create(ctx, anyMasque); err != nil {
		t.Fatalf("create any masque cfg: %v", err)
	}

	got, err := svc.activeMasqueConfig(ctx, 2, "xray")
	if err != nil {
		t.Fatalf("activeMasqueConfig: %v", err)
	}
	if got.ID != masqueCfg.ID {
		t.Fatalf("应选中 xray 的 MASQUE 记录 id=%d，实际 id=%d", masqueCfg.ID, got.ID)
	}
}

// GetActiveWireGuard 必须排除 MASQUE（EC 私钥），否则生成的 WG 配置不可用。
func TestGetActiveWireGuardSkipsMasque(t *testing.T) {
	svc, _ := newMasqueTestService(t)
	ctx := context.Background()

	wg := &entity.WarpConfig{
		ServerID: 3, CoreType: "xray", Status: "registered",
		PrivateKey: "wg-priv", PublicKey: "wg-pub", TunnelType: "wireguard",
	}
	if err := svc.repo.Create(ctx, wg); err != nil {
		t.Fatalf("create wg: %v", err)
	}
	masque := &entity.WarpConfig{
		ServerID: 3, CoreType: "xray", Status: "active",
		PrivateKey: "ec-priv", PublicKey: "ec-pub", TunnelType: "masque",
	}
	if err := svc.repo.Create(ctx, masque); err != nil {
		t.Fatalf("create masque: %v", err)
	}

	got, err := svc.repo.GetActiveWireGuard(ctx, 3, "xray")
	if err != nil {
		t.Fatalf("GetActiveWireGuard: %v", err)
	}
	if got.ID != wg.ID {
		t.Fatalf("应返回 WireGuard 记录 id=%d，实际 id=%d", wg.ID, got.ID)
	}
	if got.PrivateKey != "wg-priv" {
		t.Errorf("私钥未正确解密: %q", got.PrivateKey)
	}
}

// /warp/status 被「出站/端点一键导入 WARP」使用，必须返回 WireGuard 记录：
// 同节点上 id 更大的 MASQUE 记录不能顶掉它（否则前端把 EC 私钥填进 wireguard 出站）。
func TestGetStatusReturnsWireGuardNotMasque(t *testing.T) {
	svc, _ := newMasqueTestService(t)
	ctx := context.Background()

	wg := &entity.WarpConfig{
		ServerID: 4, CoreType: "xray", Status: "registered",
		PrivateKey: "wg-priv", PublicKey: "wg-pub", TunnelType: "wireguard",
	}
	if err := svc.repo.Create(ctx, wg); err != nil {
		t.Fatalf("create wg: %v", err)
	}
	masque := &entity.WarpConfig{
		ServerID: 4, CoreType: "xray", Status: "active",
		PrivateKey: "ec-priv", PublicKey: "ec-pub", TunnelType: "masque",
	}
	if err := svc.repo.Create(ctx, masque); err != nil {
		t.Fatalf("create masque: %v", err)
	}

	status, err := svc.GetStatus(4, "xray")
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if !status.Registered {
		t.Fatalf("应返回已注册状态: %+v", status)
	}
	if status.PrivateKey != "wg-priv" {
		t.Fatalf("应返回 WireGuard 私钥，实际 %q", status.PrivateKey)
	}
	if status.TunnelType == "masque" {
		t.Errorf("不应返回 MASQUE 记录")
	}
}

// 只有 MASQUE 记录时 /warp/status 必须返回 not_registered，
// 前端据此按核心类型重新注册 WireGuard，而不是复用 MASQUE。
func TestGetStatusWithoutWireGuardIsNotRegistered(t *testing.T) {
	svc, _ := newMasqueTestService(t)
	ctx := context.Background()

	masque := &entity.WarpConfig{
		ServerID: 5, CoreType: "xray", Status: "active",
		PrivateKey: "ec-priv", PublicKey: "ec-pub", TunnelType: "masque",
	}
	if err := svc.repo.Create(ctx, masque); err != nil {
		t.Fatalf("create masque: %v", err)
	}

	status, err := svc.GetStatus(5, "xray")
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if status.Registered {
		t.Fatalf("只有 MASQUE 时应为 not_registered，实际 %+v", status)
	}
}
