package warp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

func masqueWarpConfig() *entity.WarpConfig {
	return &entity.WarpConfig{
		ID:            7,
		ServerID:      1,
		CoreType:      "xray",
		Remark:        "warp masque",
		DeviceID:      "dev-1",
		Token:         "token-1",
		PrivateKey:    "cHJpdmF0ZQ==",
		PublicKey:     "cHVibGlj",
		AddressIPv4:   "172.16.0.2",
		AddressIPv6:   "2606:4700:110::2",
		Endpoint:      "162.159.198.2",
		KeyType:       "secp256r1",
		TunnelType:    "masque",
		EndpointPorts: "443,500",
		Status:        "registered",
	}
}

// TestBuildMasqueOutboundConfig 校验生成的出站结构：warp 三件套、masque 传输、TLS。
func TestBuildMasqueOutboundConfig(t *testing.T) {
	raw, tag, err := BuildMasqueOutboundConfig(masqueWarpConfig(), MasqueOutboundInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag != "warp-masque-7" {
		t.Errorf("tag = %q, want warp-masque-7", tag)
	}

	var ob map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &ob); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ob["protocol"] != "masque" {
		t.Errorf("protocol = %v", ob["protocol"])
	}
	settings := ob["settings"].(map[string]interface{})
	if settings["address"] != "162.159.198.2" {
		t.Errorf("address = %v", settings["address"])
	}
	if settings["port"].(float64) != 443 {
		t.Errorf("port = %v, want 443（注册返回的首个端口）", settings["port"])
	}
	ss := ob["streamSettings"].(map[string]interface{})
	if ss["network"] != "masque" || ss["security"] != "tls" {
		t.Errorf("streamSettings network/security = %v/%v", ss["network"], ss["security"])
	}
	mq := ss["masqueSettings"].(map[string]interface{})
	warp := mq["warp"].(map[string]interface{})
	if warp["privateKey"] != "cHJpdmF0ZQ==" || warp["publicKey"] != "cHVibGlj" {
		t.Errorf("warp keys = %#v", warp)
	}
	addrs := warp["address"].([]interface{})
	if len(addrs) != 2 || addrs[0] != "172.16.0.2/32" || addrs[1] != "2606:4700:110::2/128" {
		t.Errorf("warp address = %#v, want /32 与 /128", addrs)
	}
	if _, hasUser := mq["user"]; hasUser {
		t.Error("warp 模式下不应带 user")
	}
	tls := ss["tlsSettings"].(map[string]interface{})
	if tls["serverName"] != masqueConnectSNI {
		t.Errorf("serverName = %v", tls["serverName"])
	}
	if alpn := tls["alpn"].([]interface{}); len(alpn) != 1 || alpn[0] != "h3" {
		t.Errorf("alpn = %#v", alpn)
	}
}

// TestBuildMasqueOutboundConfigRejectsWireGuard 旧 WireGuard 记录不能直接生成 masque 出站。
func TestBuildMasqueOutboundConfigRejectsWireGuard(t *testing.T) {
	cfg := masqueWarpConfig()
	cfg.TunnelType = "" // 历史数据
	if _, _, err := BuildMasqueOutboundConfig(cfg, MasqueOutboundInput{}); err == nil {
		t.Fatal("expected error for wireguard record")
	}
	cfg.TunnelType = "wireguard"
	if _, _, err := BuildMasqueOutboundConfig(cfg, MasqueOutboundInput{}); err == nil {
		t.Fatal("expected error for wireguard record")
	}
}

func TestNormalizeEndpointAndPorts(t *testing.T) {
	cases := map[string]string{
		"162.159.198.2:443":      "162.159.198.2",
		"[2606:4700:103::2]:443": "2606:4700:103::2",
		"162.159.198.2":          "162.159.198.2",
		"2606:4700:103::2":       "2606:4700:103::2",
	}
	for in, want := range cases {
		if got := normalizeEndpoint(in); got != want {
			t.Errorf("normalizeEndpoint(%q) = %q, want %q", in, got, want)
		}
	}
	ports := splitPorts("443, 500,abc,70000,8443")
	if len(ports) != 3 || ports[0] != 443 || ports[1] != 500 || ports[2] != 8443 {
		t.Errorf("splitPorts = %#v", ports)
	}
}

func newMasqueTestService(t *testing.T) (*Service, *repository.OutboundConfigRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&entity.WarpConfig{}, &entity.OutboundConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := &Service{
		repo:         repository.NewWarpRepository(db),
		outboundRepo: repository.NewOutboundConfigRepository(db),
	}
	return svc, svc.outboundRepo
}

// TestCreateMasqueOutbound 校验落库与 tag 去重（同 tag 连续创建会追加序号）。
func TestCreateMasqueOutbound(t *testing.T) {
	svc, outRepo := newMasqueTestService(t)

	cfg := masqueWarpConfig()
	cfg.ID = 0
	cfg.Status = "registered"
	if err := svc.repo.Create(nil, cfg); err != nil {
		t.Fatalf("create warp cfg: %v", err)
	}

	first, err := svc.CreateMasqueOutbound(1, cfg.ID, MasqueOutboundInput{Tag: "warp-masque"})
	if err != nil {
		t.Fatalf("create outbound: %v", err)
	}
	if first.Tag != "warp-masque" {
		t.Errorf("tag = %q", first.Tag)
	}
	saved, err := outRepo.GetByID(nil, 1, first.ID)
	if err != nil {
		t.Fatalf("load outbound: %v", err)
	}
	if saved.Protocol != "masque" || !saved.Enabled || saved.CoreType != "xray" {
		t.Errorf("saved outbound = %+v", saved)
	}
	if !strings.Contains(saved.ConfigJSON, `"masqueSettings"`) {
		t.Errorf("config json missing masqueSettings: %s", saved.ConfigJSON)
	}

	second, err := svc.CreateMasqueOutbound(1, cfg.ID, MasqueOutboundInput{Tag: "warp-masque"})
	if err != nil {
		t.Fatalf("create second outbound: %v", err)
	}
	if second.Tag != "warp-masque-2" {
		t.Errorf("duplicate tag not resolved: %q", second.Tag)
	}
}

// TestCreateMasqueOutboundRejectsOtherServer 防止跨节点误用。
func TestCreateMasqueOutboundRejectsOtherServer(t *testing.T) {
	svc, _ := newMasqueTestService(t)
	cfg := masqueWarpConfig()
	cfg.ID = 0
	if err := svc.repo.Create(nil, cfg); err != nil {
		t.Fatalf("create warp cfg: %v", err)
	}
	if _, err := svc.CreateMasqueOutbound(2, cfg.ID, MasqueOutboundInput{}); err == nil {
		t.Fatal("expected server mismatch error")
	}
}
