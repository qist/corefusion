package client

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

type fakeUserManager struct {
	calls []fakeAddUserCall
}

type fakeAddUserCall struct {
	Tag      string
	Protocol string
	Email    string
	UUID     string
	Password string
	Flow     string
	Method   string
}

func (f *fakeUserManager) AddUserToInbound(ctx context.Context, inboundTag, email, uuid, password, flow, method string) error {
	return f.AddUserToInboundWithProtocol(ctx, inboundTag, "", email, uuid, password, flow, method)
}

func (f *fakeUserManager) AddUserToInboundWithProtocol(ctx context.Context, inboundTag, protocol, email, uuid, password, flow, method string) error {
	f.calls = append(f.calls, fakeAddUserCall{
		Tag: inboundTag, Protocol: protocol, Email: email,
		UUID: uuid, Password: password, Flow: flow, Method: method,
	})
	return nil
}

func (f *fakeUserManager) RemoveUserFromInbound(ctx context.Context, inboundTag, email string) error {
	return nil
}
func (f *fakeUserManager) PauseUser(ctx context.Context, inboundID int64, email string) error {
	return nil
}
func (f *fakeUserManager) ResumeUser(ctx context.Context, inboundID int64, email string) error {
	return nil
}

func newClientTestService(t *testing.T) (*Service, *repository.InboundConfigRepository, *fakeUserManager) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	if err := db.AutoMigrate(&entity.InboundConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := repository.NewInboundConfigRepository(db)
	mgr := &fakeUserManager{}
	return &Service{inboundRepo: repo, userManager: mgr}, repo, mgr
}

func xhttpVlessConfig(t *testing.T, path string, users ...map[string]interface{}) string {
	t.Helper()
	clients := make([]interface{}, 0, len(users))
	for _, u := range users {
		clients = append(clients, u)
	}
	cfg := map[string]interface{}{
		"protocol": "vless",
		"settings": map[string]interface{}{"clients": clients, "decryption": "none"},
		"streamSettings": map[string]interface{}{
			"network":       "xhttp",
			"xhttpSettings": map[string]interface{}{"path": path, "mode": "auto"},
		},
	}
	raw, _ := json.Marshal(cfg)
	return string(raw)
}

// TestSyncDownloadAccountsCopiesCredentials 校验：入站关联下行入口后，
// 上行用户会被复制到下行入站，凭据相同、email 换成下行入站后缀。
func TestSyncDownloadAccountsCopiesCredentials(t *testing.T) {
	svc, repo, mgr := newClientTestService(t)
	ctx := context.Background()

	const uuid = "11111111-1111-1111-1111-111111111111"
	uploadCfg := xhttpVlessConfig(t, "/up", map[string]interface{}{
		"email": "a@x.com_vless20296",
		"id":    uuid,
	})
	upload := &entity.InboundConfig{
		ServerID: 1, CoreType: "xray", Protocol: "vless", Tag: "inbound-vless-20296",
		Listen: "127.0.0.1", Port: 20296, Enabled: true, ConfigJSON: uploadCfg,
	}
	if err := repo.Create(ctx, upload); err != nil {
		t.Fatalf("create upload inbound: %v", err)
	}

	targetCfg := xhttpVlessConfig(t, "/up")
	target := &entity.InboundConfig{
		ServerID: 1, CoreType: "xray", Protocol: "vless", Tag: "inbound-vless-8443",
		Listen: "127.0.0.1", Port: 8443, Enabled: true, ConfigJSON: targetCfg,
	}
	if err := repo.Create(ctx, target); err != nil {
		t.Fatalf("create target inbound: %v", err)
	}

	// 写入下行引用
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(uploadCfg), &parsed); err != nil {
		t.Fatalf("unmarshal upload cfg: %v", err)
	}
	ss := parsed["streamSettings"].(map[string]interface{})
	xh := ss["xhttpSettings"].(map[string]interface{})
	xh["extra"] = map[string]interface{}{"_downloadInboundId": float64(target.ID)}
	raw, _ := json.Marshal(parsed)
	upload.ConfigJSON = string(raw)
	if err := repo.Update(ctx, upload); err != nil {
		t.Fatalf("update upload inbound: %v", err)
	}

	if err := svc.SyncDownloadAccounts(ctx, upload.ID); err != nil {
		t.Fatalf("SyncDownloadAccounts: %v", err)
	}

	if len(mgr.calls) != 1 {
		t.Fatalf("expected 1 account copy, got %d: %#v", len(mgr.calls), mgr.calls)
	}
	call := mgr.calls[0]
	if call.Tag != target.Tag || call.Protocol != "vless" {
		t.Errorf("target/tag mismatch: %#v", call)
	}
	if call.Email != "a@x.com_vless8443" {
		t.Errorf("email = %q, want a@x.com_vless8443", call.Email)
	}
	if call.UUID != uuid {
		t.Errorf("uuid = %q, want %q（必须与上行一致，否则下行认证失败）", call.UUID, uuid)
	}
}

// TestSyncDownloadAccountsSkipsProtocolMismatch 协议不同时不能复制凭据。
func TestSyncDownloadAccountsSkipsProtocolMismatch(t *testing.T) {
	svc, repo, mgr := newClientTestService(t)
	ctx := context.Background()

	uploadCfg := xhttpVlessConfig(t, "/up", map[string]interface{}{
		"email": "a@x.com_vless20296",
		"id":    "11111111-1111-1111-1111-111111111111",
	})
	targetCfg := xhttpVlessConfig(t, "/up")
	target := &entity.InboundConfig{
		ServerID: 1, CoreType: "xray", Protocol: "trojan", Tag: "inbound-trojan-8443",
		Listen: "127.0.0.1", Port: 8443, Enabled: true, ConfigJSON: targetCfg,
	}
	if err := repo.Create(ctx, target); err != nil {
		t.Fatalf("create target: %v", err)
	}
	var parsed map[string]interface{}
	_ = json.Unmarshal([]byte(uploadCfg), &parsed)
	ss := parsed["streamSettings"].(map[string]interface{})
	xh := ss["xhttpSettings"].(map[string]interface{})
	xh["extra"] = map[string]interface{}{"_downloadInboundId": float64(target.ID)}
	raw, _ := json.Marshal(parsed)
	upload := &entity.InboundConfig{
		ServerID: 1, CoreType: "xray", Protocol: "vless", Tag: "inbound-vless-20296",
		Listen: "127.0.0.1", Port: 20296, Enabled: true, ConfigJSON: string(raw),
	}
	if err := repo.Create(ctx, upload); err != nil {
		t.Fatalf("create upload: %v", err)
	}

	if err := svc.SyncDownloadAccounts(ctx, upload.ID); err != nil {
		t.Fatalf("SyncDownloadAccounts: %v", err)
	}
	if len(mgr.calls) != 0 {
		t.Errorf("协议不一致时不应建号，实际: %#v", mgr.calls)
	}
}
