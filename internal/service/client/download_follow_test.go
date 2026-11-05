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

func (f *fakeUserManager) UpsertUserInbound(ctx context.Context, inboundTag, protocol, email, uuid, password, flow, method string) error {
	return f.AddUserToInboundWithProtocol(ctx, inboundTag, protocol, email, uuid, password, flow, method)
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
	if err := db.AutoMigrate(&entity.InboundConfig{}, &entity.Client{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := repository.NewInboundConfigRepository(db)
	mgr := &fakeUserManager{}
	svc := &Service{
		inboundRepo: repo,
		clientRepo:  repository.NewClientRepository(db),
		userManager: mgr,
	}
	return svc, repo, mgr
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

func mustClient(t *testing.T, svc *Service, email string) {
	t.Helper()
	if err := svc.clientRepo.Create(context.Background(), &entity.Client{Email: email}); err != nil {
		t.Fatalf("create client %s: %v", email, err)
	}
}

// makeUplink 造一个上行入站（xhttp vless），可选写入下行引用。
func makeUplink(t *testing.T, repo *repository.InboundConfigRepository, tag string, port int, email, uuid string, targetID int64) *entity.InboundConfig {
	t.Helper()
	ctx := context.Background()
	cfgJSON := xhttpVlessConfig(t, "/up", map[string]interface{}{"email": email, "id": uuid})
	if targetID > 0 {
		var parsed map[string]interface{}
		_ = json.Unmarshal([]byte(cfgJSON), &parsed)
		ss := parsed["streamSettings"].(map[string]interface{})
		xh := ss["xhttpSettings"].(map[string]interface{})
		xh["extra"] = map[string]interface{}{"_downloadInboundId": float64(targetID)}
		raw, _ := json.Marshal(parsed)
		cfgJSON = string(raw)
	}
	up := &entity.InboundConfig{
		ServerID: 1, CoreType: "xray", Protocol: "vless", Tag: tag,
		Listen: "127.0.0.1", Port: port, Enabled: true, ConfigJSON: cfgJSON,
	}
	if err := repo.Create(ctx, up); err != nil {
		t.Fatalf("create uplink %s: %v", tag, err)
	}
	return up
}

func makeTarget(t *testing.T, repo *repository.InboundConfigRepository, tag string, port int) *entity.InboundConfig {
	t.Helper()
	target := &entity.InboundConfig{
		ServerID: 1, CoreType: "xray", Protocol: "vless", Tag: tag,
		Listen: "127.0.0.1", Port: port, Enabled: true,
		ConfigJSON: xhttpVlessConfig(t, "/up"),
	}
	if err := repo.Create(context.Background(), target); err != nil {
		t.Fatalf("create target %s: %v", tag, err)
	}
	return target
}

// TestSyncDownloadAccountsCopiesCredentials 上行客户账号要被复制到下行入站，
// 凭据完全一致、email 换成下行入站后缀。
func TestSyncDownloadAccountsCopiesCredentials(t *testing.T) {
	svc, repo, mgr := newClientTestService(t)
	ctx := context.Background()

	const uuid = "11111111-1111-1111-1111-111111111111"
	target := makeTarget(t, repo, "inbound-vless-8443", 8443)
	up := makeUplink(t, repo, "inbound-vless-20296", 20296, "a@x.com_vless20296", uuid, target.ID)
	mustClient(t, svc, "a@x.com")

	if err := svc.SyncDownloadAccounts(ctx, up.ID); err != nil {
		t.Fatalf("SyncDownloadAccounts: %v", err)
	}
	if len(mgr.calls) != 1 {
		t.Fatalf("expected 1 account, got %d: %#v", len(mgr.calls), mgr.calls)
	}
	call := mgr.calls[0]
	if call.Tag != target.Tag || call.Protocol != "vless" {
		t.Errorf("target/protocol mismatch: %#v", call)
	}
	if call.Email != "a@x.com_vless8443" {
		t.Errorf("email = %q, want a@x.com_vless8443", call.Email)
	}
	if call.UUID != uuid {
		t.Errorf("uuid = %q, want %q（必须与上行一致）", call.UUID, uuid)
	}
}

// TestSyncDownloadAccountsSkipsNonClient 手工账号（没有 client 记录）不能复制到下行，
// 否则会出现统计归不到任何客户的账号。
func TestSyncDownloadAccountsSkipsNonClient(t *testing.T) {
	svc, repo, mgr := newClientTestService(t)
	target := makeTarget(t, repo, "inbound-vless-8443", 8443)
	up := makeUplink(t, repo, "inbound-vless-20296", 20296, "manual@x.com_vless20296", "22222222-2222-2222-2222-222222222222", target.ID)

	if err := svc.SyncDownloadAccounts(context.Background(), up.ID); err != nil {
		t.Fatalf("SyncDownloadAccounts: %v", err)
	}
	if len(mgr.calls) != 0 {
		t.Errorf("非订阅客户不应复制，实际: %#v", mgr.calls)
	}
}

// TestSyncDownloadAccountsDetectsCredentialConflict 两个上行引用同一 target、同一客户
// 却是不同凭据时，必须拒绝写入（否则无论写谁都会让另一条上行认证失败/串号）。
func TestSyncDownloadAccountsDetectsCredentialConflict(t *testing.T) {
	svc, repo, mgr := newClientTestService(t)
	target := makeTarget(t, repo, "inbound-vless-8443", 8443)
	up1 := makeUplink(t, repo, "inbound-vless-20296", 20296, "a@x.com_vless20296", "11111111-1111-1111-1111-111111111111", target.ID)
	makeUplink(t, repo, "inbound-vless-20297", 20297, "a@x.com_vless20297", "33333333-3333-3333-3333-333333333333", target.ID)
	mustClient(t, svc, "a@x.com")

	if err := svc.SyncDownloadAccounts(context.Background(), up1.ID); err != nil {
		t.Fatalf("SyncDownloadAccounts: %v", err)
	}
	if len(mgr.calls) != 0 {
		t.Errorf("凭据冲突时不应写入，实际: %#v", mgr.calls)
	}
}
