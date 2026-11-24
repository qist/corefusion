package sharedomainconfig

import (
	"context"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

type fakeDNSUpdater struct {
	calls []struct {
		channelID int64
		fqdn      string
		target    string
		proxied   bool
	}
	err error
}

func (f *fakeDNSUpdater) UpsertA(_ context.Context, channelID int64, fqdn, target string, proxied bool) error {
	f.calls = append(f.calls, struct {
		channelID int64
		fqdn      string
		target    string
		proxied   bool
	}{channelID, fqdn, target, proxied})
	return f.err
}

func newShareDomainTest(t *testing.T) (*Service, *repository.ShareDomainConfigRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(&entity.ShareDomainConfig{}); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	repo := repository.NewShareDomainConfigRepository(db)
	return NewService(repo), repo
}

// 只更新「direct + 配了 DNS 通道 + 绑定到当前 primary」的域名；
// 未配通道、值是 IP、绑定到别的节点的一律跳过。
func TestDriftForPrimaryFiltersAndUpdates(t *testing.T) {
	svc, repo := newShareDomainTest(t)
	ctx := context.Background()

	rows := []*entity.ShareDomainConfig{
		{Type: "direct", Value: "entry.tycng.com", DNSChannelID: 2, ServerIDs: "[12,13]", Enabled: true},
		{Type: "direct", Value: "old.tycng.com", DNSChannelID: 2, ServerIDs: "[99]", Enabled: true},
		{Type: "direct", Value: "nochannel.tycng.com", Enabled: true},
		{Type: "direct", Value: "192.168.2.180", DNSChannelID: 2, ServerIDs: "[12]", Enabled: true},
		{Type: "cf_domain", Value: "cf.example.com", DNSChannelID: 2, ServerIDs: "[12]", Enabled: true},
		{Type: "direct", Value: "ignored.example.com", DNSRecordName: "real.tycng.com", DNSChannelID: 2, ServerIDs: "[12]", Enabled: true},
	}
	for _, r := range rows {
		if err := repo.Create(ctx, r); err != nil {
			t.Fatalf("插入失败：%v", err)
		}
	}

	fake := &fakeDNSUpdater{}
	svc.WithDNSUpdater(fake)
	n, err := svc.DriftForPrimary(ctx, 12, "192.168.2.176")
	if err != nil {
		t.Fatalf("drift: %v", err)
	}
	if n != 2 {
		t.Fatalf("应更新 2 条，实际 %d（calls=%+v）", n, fake.calls)
	}
	got := map[string]string{}
	for _, c := range fake.calls {
		got[c.fqdn] = c.target
	}
	if got["entry.tycng.com"] != "192.168.2.176" {
		t.Errorf("entry.tycng.com 未更新：%+v", fake.calls)
	}
	if got["real.tycng.com"] != "192.168.2.176" {
		t.Errorf("自定义记录名未生效：%+v", fake.calls)
	}
	if _, ok := got["old.tycng.com"]; ok {
		t.Error("绑定到其它节点的域名不应更新")
	}
	if _, ok := got["nochannel.tycng.com"]; ok {
		t.Error("未配置 DNS 通道的域名不应更新")
	}
}

// 非法目标 IP 直接报错，且不触发任何更新。
func TestDriftForPrimaryRejectsBadIP(t *testing.T) {
	svc, _ := newShareDomainTest(t)
	fake := &fakeDNSUpdater{}
	svc.WithDNSUpdater(fake)
	if _, err := svc.DriftForPrimary(context.Background(), 12, "not-an-ip"); err == nil {
		t.Fatal("非法 IP 应报错")
	}
	if len(fake.calls) != 0 {
		t.Fatalf("不应有更新调用：%+v", fake.calls)
	}
}
