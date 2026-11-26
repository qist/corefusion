package healthprobe

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

func newProbeTestService(t *testing.T) *Prober {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "probe.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.SystemSetting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return New(nil, nil, repository.NewSystemSettingRepository(db), nil, nil, t.TempDir())
}

// 探测默认关闭：没配置过（或键不存在）时不能自动开始定时探测。
func TestReadConfigProbeDisabledByDefault(t *testing.T) {
	p := newProbeTestService(t)
	cfg := p.ReadConfig(context.Background())
	if cfg.Enabled {
		t.Fatalf("默认应为关闭，实际 enabled=true")
	}
	if cfg.IntervalMinutes != DefaultIntervalMinutes {
		t.Fatalf("默认间隔 = %d, want %d", cfg.IntervalMinutes, DefaultIntervalMinutes)
	}
}

// 保存开关后立即读回新值（调度器会被 Wake 唤醒，不必等上一轮间隔）。
func TestSaveConfigTogglesProbe(t *testing.T) {
	p := newProbeTestService(t)
	ctx := context.Background()

	on := true
	if err := p.SaveConfig(ctx, ConfigUpdate{Enabled: &on}); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if cfg := p.ReadConfig(ctx); !cfg.Enabled {
		t.Fatalf("开启后应 enabled=true")
	}

	off := false
	if err := p.SaveConfig(ctx, ConfigUpdate{Enabled: &off}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if cfg := p.ReadConfig(ctx); cfg.Enabled {
		t.Fatalf("关闭后应 enabled=false")
	}
}

// 间隔/阈值等其它字段的保存不受开关影响。
func TestSaveConfigKeepsOtherFields(t *testing.T) {
	p := newProbeTestService(t)
	ctx := context.Background()

	on := true
	interval := 30
	maxLatency := 800
	entryNode := int64(7)
	dialer := "xray"
	if err := p.SaveConfig(ctx, ConfigUpdate{Enabled: &on, IntervalMinutes: &interval, MaxLatencyMs: &maxLatency, EntryNodeID: &entryNode, Dialer: &dialer}); err != nil {
		t.Fatalf("save: %v", err)
	}
	cfg := p.ReadConfig(ctx)
	if !cfg.Enabled || cfg.IntervalMinutes != 30 || cfg.MaxLatencyMs != 800 || cfg.EntryNodeID != 7 || cfg.Dialer != "xray" {
		t.Fatalf("配置读回不一致: %+v", cfg)
	}
	// 旧字段必须能推导成统一 spec，否则前端切到 entry 后会丢入口。
	if cfg.Entry != "ext:7" {
		t.Fatalf("entry = %q, want ext:7", cfg.Entry)
	}
}

func TestParseBoolSetting(t *testing.T) {
	cases := []struct {
		in   string
		def  bool
		want bool
	}{
		{"true", false, true},
		{"1", false, true},
		{"on", false, true},
		{"false", true, false},
		{"0", true, false},
		{"", true, true},
		{"", false, false},
		{"garbage", true, true},
	}
	for _, c := range cases {
		if got := parseBoolSetting(c.in, c.def); got != c.want {
			t.Errorf("parseBoolSetting(%q, %v) = %v, want %v", c.in, c.def, got, c.want)
		}
	}
}

// 未选择入口时禁止探测：本机直连只会做 TCP 判活，会把坏条目判成可用。
func TestCheckAllRequiresEntry(t *testing.T) {
	p := newProbeTestService(t)
	if _, err := p.CheckAll(context.Background()); err == nil {
		t.Fatal("未选择入口时 CheckAll 必须返回错误")
	}
}

// 执行节点与出口网卡要能持久化，并在非法网卡名时报错。
func TestSaveConfigProbeExecution(t *testing.T) {
	p := newProbeTestService(t)
	ctx := context.Background()

	agentID := int64(12)
	iface := "eth1"
	if err := p.SaveConfig(ctx, ConfigUpdate{AgentID: &agentID, Interface: &iface}); err != nil {
		t.Fatalf("save: %v", err)
	}
	cfg := p.ReadConfig(ctx)
	if cfg.AgentID != 12 || cfg.Interface != "eth1" {
		t.Fatalf("读回不一致: agent=%d iface=%q", cfg.AgentID, cfg.Interface)
	}

	zero := int64(0)
	empty := ""
	if err := p.SaveConfig(ctx, ConfigUpdate{AgentID: &zero, Interface: &empty}); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if cfg := p.ReadConfig(ctx); cfg.AgentID != 0 || cfg.Interface != "" {
		t.Fatalf("重置失败: %+v", cfg)
	}

	bad := "bad/nic"
	if err := p.SaveConfig(ctx, ConfigUpdate{Interface: &bad}); err == nil {
		t.Fatal("非法网卡名应报错")
	}
}

// 选中执行节点后，拨号核心必须按**该节点**的安装情况挑，不能用面板清单代替。
func TestPickDialerOnUsesAgentInstallList(t *testing.T) {
	p := newProbeTestService(t)
	ctx := context.Background()
	p.SetAgentDialerLister(func(_ context.Context, serverID int64) (AgentDialerInfo, error) {
		if serverID != 13 {
			t.Fatalf("unexpected server id %d", serverID)
		}
		return AgentDialerInfo{Dialers: []string{"singbox"}, Versions: map[string]string{"singbox": "1.15.0"}}, nil
	})

	// 未指定核心：按节点已安装的核心自动挑（面板没装任何核心也不影响）。
	if d, err := p.pickDialerOn(ctx, "", 13); err != nil || d != "singbox" {
		t.Fatalf("自动挑核心 = %q, err=%v，期望 singbox", d, err)
	}
	// 选了节点没装的核心：直接报错，不能静默生成一个跑不起来的配置。
	_, err := p.pickDialerOn(ctx, "xray", 13)
	if err == nil || !strings.Contains(err.Error(), "未安装") {
		t.Fatalf("缺核心应报错并说明，实际 %v", err)
	}
	// 别名归一后仍按节点判断。
	if _, err := p.pickDialerOn(ctx, "v2ray", 13); err == nil {
		t.Fatal("v2ray 归一成 xray 后应报节点缺核心")
	}
	// 面板本机（agent=0）不查节点清单：保持原有「按名字校验」的行为。
	if d, err := p.pickDialerOn(ctx, "xray", 0); err != nil || d != "xray" {
		t.Fatalf("面板本机应直接返回 xray，got %q err=%v", d, err)
	}
}

// 节点一个核心都没装时给出明确提示。
func TestPickDialerOnAgentWithoutCores(t *testing.T) {
	p := newProbeTestService(t)
	p.SetAgentDialerLister(func(context.Context, int64) (AgentDialerInfo, error) {
		return AgentDialerInfo{}, nil
	})
	if _, err := p.pickDialerOn(context.Background(), "", 9); err == nil {
		t.Fatal("节点无核心应报错")
	}
}

// AgentDialers(0) 返回面板本机清单（测试环境为空），且不调用节点查询。
func TestAgentDialersLocal(t *testing.T) {
	p := newProbeTestService(t)
	called := false
	p.SetAgentDialerLister(func(context.Context, int64) (AgentDialerInfo, error) {
		called = true
		return AgentDialerInfo{}, nil
	})
	info, err := p.AgentDialers(context.Background(), 0)
	if err != nil {
		t.Fatalf("local: %v", err)
	}
	if called {
		t.Fatal("agent=0 不应查询节点")
	}
	if len(info.Dialers) != 0 {
		t.Fatalf("测试环境面板无核心，实际 %v", info.Dialers)
	}
}

func TestNormalizeProbeKind(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"", ProbeKindAll, true},
		{"all", ProbeKindAll, true},
		{"IP", ProbeKindIP, true},
		{"cf_ip", ProbeKindIP, true},
		{"domain", ProbeKindDomain, true},
		{" CF_DOMAIN ", ProbeKindDomain, true},
		{"tcp", "", false},
	}
	for _, c := range cases {
		got, ok := NormalizeProbeKind(c.in)
		if got != c.want || ok != c.ok {
			t.Fatalf("NormalizeProbeKind(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestProbeKindWanted(t *testing.T) {
	cases := []struct {
		want, target string
		expect       bool
	}{
		{ProbeKindAll, "cf_ip", true},
		{ProbeKindAll, "cf_domain", true},
		{ProbeKindIP, "cf_ip", true},
		{ProbeKindIP, "cf_domain", false},
		{ProbeKindDomain, "cf_domain", true},
		{ProbeKindDomain, "cf_ip", false},
	}
	for _, c := range cases {
		if got := probeKindWanted(c.want, c.target); got != c.expect {
			t.Fatalf("probeKindWanted(%q, %q) = %v, want %v", c.want, c.target, got, c.expect)
		}
	}
}

// 检测连通性按 kind 分开：IP 页只收 IP 目标，域名页只收域名目标。
func TestCollectProbeTargetsByKind(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "targets.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.CFIPEntry{}, &entity.ShareDomainConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&entity.CFIPEntry{IP: "192.0.2.10", Enabled: true}).Error; err != nil {
		t.Fatalf("seed ip: %v", err)
	}
	if err := db.Create(&entity.ShareDomainConfig{Type: "cf_domain", Value: "example.com", Port: 443, Enabled: true}).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}

	p := New(repository.NewCFIPEntryRepository(db), repository.NewShareDomainConfigRepository(db), nil, nil, nil, t.TempDir())
	ctx := context.Background()

	kinds := func(items []probeTarget) map[string]int {
		out := map[string]int{}
		for _, it := range items {
			out[it.kind]++
		}
		return out
	}

	all, _, err := p.collectProbeTargets(ctx, ProbeKindAll)
	if err != nil {
		t.Fatalf("all: %v", err)
	}
	if k := kinds(all); k["cf_ip"] != 1 || k["cf_domain"] != 1 {
		t.Fatalf("全部应各 1 条，实际 %v", k)
	}

	ips, _, err := p.collectProbeTargets(ctx, ProbeKindIP)
	if err != nil {
		t.Fatalf("ip: %v", err)
	}
	if k := kinds(ips); k["cf_ip"] != 1 || k["cf_domain"] != 0 {
		t.Fatalf("ip 模式只应有 cf_ip，实际 %v", k)
	}

	domains, _, err := p.collectProbeTargets(ctx, ProbeKindDomain)
	if err != nil {
		t.Fatalf("domain: %v", err)
	}
	if k := kinds(domains); k["cf_domain"] != 1 || k["cf_ip"] != 0 {
		t.Fatalf("domain 模式只应有 cf_domain，实际 %v", k)
	}
}
