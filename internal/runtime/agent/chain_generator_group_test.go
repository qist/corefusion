package agent

import (
	"context"
	"testing"

	sqlite "github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

func setupChainGeneratorDB(t *testing.T) (*ChainLinkGenerator, *repository.InboundConfigRepository, *repository.NodeServerRepository, *repository.ChainLinkRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&entity.NodeServer{}, &entity.ChainLink{}, &entity.InboundConfig{}); err != nil {
		t.Fatal(err)
	}
	inboundRepo := repository.NewInboundConfigRepository(db)
	serverRepo := repository.NewNodeServerRepository(db)
	linkRepo := repository.NewChainLinkRepository(db)
	gen := NewChainLinkGenerator(linkRepo, serverRepo, nil)
	gen.SetInboundRepo(inboundRepo)
	return gen, inboundRepo, serverRepo, linkRepo
}

// 多出口必须生成**一个** balancer（leastPing 自动故障转移），而不是多条同 inboundTag
// 的 field 规则 —— 后者只有第一条生效，主出口挂掉不会切备。
func TestChainGeneratorMultiExitUsesBalancer(t *testing.T) {
	ctx := context.Background()
	gen, inboundRepo, serverRepo, linkRepo := setupChainGeneratorDB(t)

	entry := &entity.NodeServer{Name: "entry", PublicIP: "1.1.1.1", Enabled: true}
	ex1 := &entity.NodeServer{Name: "exit1", PublicIP: "2.2.2.2", Enabled: true}
	ex2 := &entity.NodeServer{Name: "exit2", PublicIP: "3.3.3.3", Enabled: true}
	for _, s := range []*entity.NodeServer{entry, ex1, ex2} {
		if err := serverRepo.Create(context.Background(), s); err != nil {
			t.Fatal(err)
		}
	}
	entryIb := &entity.InboundConfig{
		ServerID: entry.ID, CoreType: "xray", Protocol: "vless", Port: 443, Tag: "inbound-entry", Enabled: true,
		ConfigJSON: `{"protocol":"vless","port":443,"tag":"inbound-entry","settings":{"clients":[{"id":"11111111-1111-1111-1111-111111111111"}]},"streamSettings":{"network":"tcp"}}`,
	}
	if err := inboundRepo.Create(context.Background(), entryIb); err != nil {
		t.Fatal(err)
	}
	mkExitInbound := func(serverID int64, port int) *entity.InboundConfig {
		return &entity.InboundConfig{
			ServerID: serverID, CoreType: "xray", Protocol: "vless", Port: port, Tag: "exit-in", Enabled: true,
			ConfigJSON: `{"protocol":"vless","port":8443,"settings":{"clients":[{"id":"22222222-2222-2222-2222-222222222222"}]},"streamSettings":{"network":"tcp"}}`,
		}
	}
	ei1 := mkExitInbound(ex1.ID, 8443)
	ei2 := mkExitInbound(ex2.ID, 8444)
	for _, ib := range []*entity.InboundConfig{ei1, ei2} {
		if err := inboundRepo.Create(context.Background(), ib); err != nil {
			t.Fatal(err)
		}
	}
	for i, pair := range []struct {
		serverID, inboundID int64
	}{{ex1.ID, ei1.ID}, {ex2.ID, ei2.ID}} {
		link := &entity.ChainLink{
			EntryServerID: entry.ID, EntryInboundID: entryIb.ID,
			ExitServerID: pair.serverID, ExitInboundID: pair.inboundID,
			Priority: i + 1, SortOrder: i, Weight: 100, Enabled: true,
		}
		if err := linkRepo.Create(context.Background(), link); err != nil {
			t.Fatal(err)
		}
	}

	outbounds, rules, balancers, obs, err := gen.generateEntryOutboundsTyped(ctx, entry.ID, "xray")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(outbounds) != 2 {
		t.Fatalf("want 2 outbounds, got %d", len(outbounds))
	}
	if outbounds[0].Tag == outbounds[1].Tag {
		t.Fatalf("duplicate outbound tag: %s", outbounds[0].Tag)
	}
	if len(balancers) != 1 {
		t.Fatalf("want 1 balancer, got %d", len(balancers))
	}
	bal := balancers[0]
	if len(bal.Selector) != 2 {
		t.Fatalf("balancer selector = %v", bal.Selector)
	}
	if bal.FallbackTag == "direct" || bal.FallbackTag == "" {
		t.Fatalf("fallback must be a chain exit, got %q", bal.FallbackTag)
	}
	if len(rules) != 1 || rules[0].BalancerTag != bal.Tag {
		t.Fatalf("rules = %#v, balancer = %q", rules, bal.Tag)
	}
	if obs == nil || len(obs.SubjectSelector) != 2 {
		t.Fatalf("observatory = %#v", obs)
	}
}

// 链路配置了但出口生成失败时必须 fail-closed（blocked），不能没有规则落到 direct。
func TestChainGeneratorInvalidExitBlocksTraffic(t *testing.T) {
	ctx := context.Background()
	gen, inboundRepo, serverRepo, linkRepo := setupChainGeneratorDB(t)

	entry := &entity.NodeServer{Name: "entry", PublicIP: "1.1.1.1", Enabled: true}
	if err := serverRepo.Create(ctx, entry); err != nil {
		t.Fatal(err)
	}
	entryIb := &entity.InboundConfig{
		ServerID: entry.ID, CoreType: "xray", Protocol: "vless", Port: 443, Tag: "inbound-entry", Enabled: true,
		ConfigJSON: `{"protocol":"vless","port":443,"tag":"inbound-entry","settings":{"clients":[{"id":"11111111-1111-1111-1111-111111111111"}]},"streamSettings":{"network":"tcp"}}`,
	}
	if err := inboundRepo.Create(ctx, entryIb); err != nil {
		t.Fatal(err)
	}
	// exit_inbound_id=0：生成器无法构建出口（旧数据/悬空引用）
	link := &entity.ChainLink{
		EntryServerID: entry.ID, EntryInboundID: entryIb.ID,
		ExitServerID: 999, Priority: 1, Enabled: true,
	}
	if err := linkRepo.Create(ctx, link); err != nil {
		t.Fatal(err)
	}

	outbounds, rules, _, _, err := gen.generateEntryOutboundsTyped(ctx, entry.ID, "xray")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(outbounds) != 0 {
		t.Fatalf("unexpected outbounds: %#v", outbounds)
	}
	if len(rules) != 1 || rules[0].OutboundTag != "blocked" {
		t.Fatalf("must fail closed to blocked, got rules=%#v", rules)
	}
}
