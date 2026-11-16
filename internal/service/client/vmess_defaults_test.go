package client

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// 面板自动建号（新建客户 / 上下行对齐 / 镜像扩散）以前把 vmess 的 alterId 写死 0
// 且完全不写 security，同一个入站里手工建的主账号却有 `security: auto`。
// 订阅里的 scy/aid 直接读 clients[].security / alterId，两套值会让新客户
// 的客户端加密参数与主账号不一致。
func TestAddUserInboundSyncsVmessDefaults(t *testing.T) {
	mgr, repo := newInboundManagerForTest(t)

	cfg, err := json.Marshal(map[string]interface{}{
		"protocol": "vmess",
		"settings": map[string]interface{}{
			"clients": []interface{}{
				map[string]interface{}{
					"id":       "11111111-1111-1111-1111-111111111111",
					"email":    "main@example.com_vmess1",
					"alterId":  float64(64),
					"security": "aes-128-gcm",
					"level":    float64(0),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	inbound := &entity.InboundConfig{
		ServerID: 1, CoreType: "xray", Protocol: "vmess", Tag: "inbound-vmess-test",
		Listen: "127.0.0.1", Port: 39001, Enabled: true, ConfigJSON: string(cfg),
	}
	if err := repo.Create(context.Background(), inbound); err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	// 与 usersync 的实际调用一致：协议显式传入，凭据只有 uuid。
	if err := mgr.AddUserToInboundWithProtocol(context.Background(), inbound.Tag, "vmess",
		"new@example.com_vmess1", "22222222-2222-2222-2222-222222222222", "", "", ""); err != nil {
		t.Fatalf("add user: %v", err)
	}

	got := readInboundUsers(t, repo, inbound.ID)
	nu := findUserByEmail(got, "new@example.com_vmess1")
	if nu == nil {
		t.Fatalf("新用户未写入: %#v", got)
	}
	if aid := intField(nu["alterId"]); aid != 64 {
		t.Errorf("新用户 alterId = %d, want 64（应跟主账号同步）", aid)
	}
	if sec := getStrField(nu, "security"); sec != "aes-128-gcm" {
		t.Errorf("新用户 security = %q, want aes-128-gcm（应跟主账号同步）", sec)
	}
}

// 主账号没有 security、alterId 为 0 时，新用户也不该凭空写 security（保持旧行为）。
func TestAddUserInboundVmessDefaultsEmptyTemplate(t *testing.T) {
	mgr, repo := newInboundManagerForTest(t)

	cfg, err := json.Marshal(map[string]interface{}{
		"protocol": "vmess",
		"settings": map[string]interface{}{
			"clients": []interface{}{
				map[string]interface{}{
					"id":      "11111111-1111-1111-1111-111111111111",
					"email":   "main@example.com_vmess1",
					"alterId": float64(0),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	inbound := &entity.InboundConfig{
		ServerID: 1, CoreType: "xray", Protocol: "vmess", Tag: "inbound-vmess-empty",
		Listen: "127.0.0.1", Port: 39002, Enabled: true, ConfigJSON: string(cfg),
	}
	if err := repo.Create(context.Background(), inbound); err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	if err := mgr.AddUserToInboundWithProtocol(context.Background(), inbound.Tag, "vmess",
		"new@example.com_vmess1", "22222222-2222-2222-2222-222222222222", "", "", ""); err != nil {
		t.Fatalf("add user: %v", err)
	}

	nu := findUserByEmail(readInboundUsers(t, repo, inbound.ID), "new@example.com_vmess1")
	if nu == nil {
		t.Fatal("新用户未写入")
	}
	if _, ok := nu["security"]; ok {
		t.Errorf("主账号没有 security 时不应给新用户写该字段: %#v", nu)
	}
	if aid := intField(nu["alterId"]); aid != 0 {
		t.Errorf("新用户 alterId = %d, want 0", aid)
	}
}

// Upsert 已有用户时也要把缺失的 security/alterId 补齐到主账号口径
// （历史自动建号留下的数据就是这样：字段缺一半）。
func TestUpsertUserInboundAlignsVmessDefaults(t *testing.T) {
	mgr, repo := newInboundManagerForTest(t)

	cfg, err := json.Marshal(map[string]interface{}{
		"protocol": "vmess",
		"settings": map[string]interface{}{
			"clients": []interface{}{
				map[string]interface{}{
					"id": "11111111-1111-1111-1111-111111111111", "email": "main@example.com_vmess1",
					"alterId": float64(64), "security": "auto",
				},
				map[string]interface{}{
					"id": "33333333-3333-3333-3333-333333333333", "email": "legacy@example.com_vmess1",
					"alterId": float64(0),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	inbound := &entity.InboundConfig{
		ServerID: 1, CoreType: "xray", Protocol: "vmess", Tag: "inbound-vmess-upsert",
		Listen: "127.0.0.1", Port: 39003, Enabled: true, ConfigJSON: string(cfg),
	}
	if err := repo.Create(context.Background(), inbound); err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	if err := mgr.UpsertUserInbound(context.Background(), inbound.Tag, "vmess",
		"legacy@example.com_vmess1", "33333333-3333-3333-3333-333333333333", "", "", ""); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	lu := findUserByEmail(readInboundUsers(t, repo, inbound.ID), "legacy@example.com_vmess1")
	if lu == nil {
		t.Fatal("用户丢失")
	}
	if aid := intField(lu["alterId"]); aid != 64 {
		t.Errorf("upsert 后 alterId = %d, want 64", aid)
	}
	if sec := getStrField(lu, "security"); sec != "auto" {
		t.Errorf("upsert 后 security = %q, want auto", sec)
	}
}

// sing-box 的 VMessUser 也有 alterId 字段：自动建号要跟主账号一致。
func TestAddUserInboundSingboxVmessAlterID(t *testing.T) {
	mgr, repo := newInboundManagerForTest(t)

	cfg, err := json.Marshal(map[string]interface{}{
		"type": "vmess",
		"tag":  "sb-vmess-test",
		"users": []interface{}{
			map[string]interface{}{"name": "main@example.com_vmess1", "uuid": "11111111-1111-1111-1111-111111111111", "alterId": float64(64)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	inbound := &entity.InboundConfig{
		ServerID: 1, CoreType: "singbox", Protocol: "vmess", Tag: "sb-vmess-test",
		Listen: "127.0.0.1", Port: 39004, Enabled: true, ConfigJSON: string(cfg),
	}
	if err := repo.Create(context.Background(), inbound); err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	if err := mgr.AddUserToInboundWithProtocol(context.Background(), inbound.Tag, "vmess",
		"new@example.com_vmess1", "22222222-2222-2222-2222-222222222222", "", "", ""); err != nil {
		t.Fatalf("add user: %v", err)
	}

	nu := findUserByEmail(readInboundUsers(t, repo, inbound.ID), "new@example.com_vmess1")
	if nu == nil {
		t.Fatal("新用户未写入")
	}
	if aid := intField(nu["alterId"]); aid != 64 {
		t.Errorf("sing-box 新用户 alterId = %d, want 64", aid)
	}
}

func newInboundManagerForTest(t *testing.T) (*InboundManager, *repository.InboundConfigRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	if err := db.AutoMigrate(&entity.InboundConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := repository.NewInboundConfigRepository(db)
	return &InboundManager{inboundRepo: repo}, repo
}

func readInboundUsers(t *testing.T, repo *repository.InboundConfigRepository, id int64) []interface{} {
	t.Helper()
	got, err := repo.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("reload inbound: %v", err)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(got.ConfigJSON), &cfg); err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if settings, ok := cfg["settings"].(map[string]interface{}); ok {
		if arr, ok := settings["clients"].([]interface{}); ok {
			return arr
		}
	}
	arr, _ := cfg["users"].([]interface{})
	return arr
}

func findUserByEmail(users []interface{}, email string) map[string]interface{} {
	for _, item := range users {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if inboundUserIdentityOf(m) == email {
			return m
		}
	}
	return nil
}

// vless 的 flow 是入站级参数：xray 在 Build 时会把 settings.flow 填给 flow 为空的
// 用户。若新用户条目 flow 留空，订阅也不会输出 flow，客户端按空 flow 连接会被
// 服务端拒绝（"client flow is empty"）——主账号能拨号、新用户连不上。
//
// 因此自动建号必须继承主账号 flow，并且**热加**与落盘同值（否则运行中的账号
// flow 空、订阅却发 vision，同样被拒）。
func TestAddUserInboundInheritsVlessFlow(t *testing.T) {
	cases := []struct {
		name         string
		mainFlow     string
		settingsFlow string
		want         string
	}{
		{"main-user", "xtls-rprx-vision", "", "xtls-rprx-vision"},
		{"settings-level", "", "xtls-rprx-vision", "xtls-rprx-vision"},
		{"none", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mgr, repo := newInboundManagerForTest(t)
			api := &recordingUserAPI{calls: make(chan string, 4)}
			mgr.userAPI = api

			settings := map[string]interface{}{
				"clients": []interface{}{
					map[string]interface{}{
						"id": "11111111-1111-1111-1111-111111111111", "email": "main@example.com_vless1",
						"flow": tc.mainFlow, "level": float64(0),
					},
				},
				"decryption": "none",
			}
			if tc.settingsFlow != "" {
				settings["flow"] = tc.settingsFlow
			}
			cfg, err := json.Marshal(map[string]interface{}{"protocol": "vless", "settings": settings})
			if err != nil {
				t.Fatal(err)
			}
			inbound := &entity.InboundConfig{
				ServerID: 1, CoreType: "xray", Protocol: "vless", Tag: "inbound-vless-flow",
				Listen: "127.0.0.1", Port: 39101, Enabled: true, ConfigJSON: string(cfg),
			}
			if err := repo.Create(context.Background(), inbound); err != nil {
				t.Fatalf("create inbound: %v", err)
			}

			if err := mgr.AddUserToInboundWithProtocol(context.Background(), inbound.Tag, "vless",
				"new@example.com_vless1", "22222222-2222-2222-2222-222222222222", "", "", ""); err != nil {
				t.Fatalf("add user: %v", err)
			}

			nu := findUserByEmail(readInboundUsers(t, repo, inbound.ID), "new@example.com_vless1")
			if nu == nil {
				t.Fatal("新用户未写入")
			}
			if got := getStrField(nu, "flow"); got != tc.want {
				t.Errorf("新用户 flow = %q, want %q", got, tc.want)
			}

			// 热加必须用同一个 flow（goroutine 内执行，等一小会儿）
			select {
			case got := <-api.calls:
				if got != tc.want {
					t.Errorf("热加 flow = %q, want %q", got, tc.want)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("没有收到热加调用")
			}
		})
	}
}

// Upsert 已有用户时补齐 flow（历史自动建号留下的空 flow）。
func TestUpsertUserInboundAlignsVlessFlow(t *testing.T) {
	mgr, repo := newInboundManagerForTest(t)

	cfg, err := json.Marshal(map[string]interface{}{
		"protocol": "vless",
		"settings": map[string]interface{}{
			"clients": []interface{}{
				map[string]interface{}{"id": "11111111-1111-1111-1111-111111111111", "email": "main@example.com_vless1", "flow": "xtls-rprx-vision"},
				map[string]interface{}{"id": "33333333-3333-3333-3333-333333333333", "email": "legacy@example.com_vless1"},
			},
			"decryption": "none",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	inbound := &entity.InboundConfig{
		ServerID: 1, CoreType: "xray", Protocol: "vless", Tag: "inbound-vless-upsert",
		Listen: "127.0.0.1", Port: 39102, Enabled: true, ConfigJSON: string(cfg),
	}
	if err := repo.Create(context.Background(), inbound); err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	if err := mgr.UpsertUserInbound(context.Background(), inbound.Tag, "vless",
		"legacy@example.com_vless1", "33333333-3333-3333-3333-333333333333", "", "", ""); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	lu := findUserByEmail(readInboundUsers(t, repo, inbound.ID), "legacy@example.com_vless1")
	if lu == nil {
		t.Fatal("用户丢失")
	}
	if got := getStrField(lu, "flow"); got != "xtls-rprx-vision" {
		t.Errorf("upsert 后 flow = %q, want xtls-rprx-vision", got)
	}
}

// sing-box 的 vless 用户也有 flow，同样继承主账号。
func TestAddUserInboundSingboxVlessFlow(t *testing.T) {
	mgr, repo := newInboundManagerForTest(t)

	cfg, err := json.Marshal(map[string]interface{}{
		"type": "vless",
		"tag":  "sb-vless-flow",
		"users": []interface{}{
			map[string]interface{}{"name": "main@example.com_vless1", "uuid": "11111111-1111-1111-1111-111111111111", "flow": "xtls-rprx-vision"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	inbound := &entity.InboundConfig{
		ServerID: 1, CoreType: "singbox", Protocol: "vless", Tag: "sb-vless-flow",
		Listen: "127.0.0.1", Port: 39103, Enabled: true, ConfigJSON: string(cfg),
	}
	if err := repo.Create(context.Background(), inbound); err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	if err := mgr.AddUserToInboundWithProtocol(context.Background(), inbound.Tag, "vless",
		"new@example.com_vless1", "22222222-2222-2222-2222-222222222222", "", "", ""); err != nil {
		t.Fatalf("add user: %v", err)
	}
	nu := findUserByEmail(readInboundUsers(t, repo, inbound.ID), "new@example.com_vless1")
	if nu == nil {
		t.Fatal("新用户未写入")
	}
	if got := getStrField(nu, "flow"); got != "xtls-rprx-vision" {
		t.Errorf("sing-box 新用户 flow = %q, want xtls-rprx-vision", got)
	}
}

// recordingUserAPI 记录热加时真正下发给核心的 flow。
type recordingUserAPI struct {
	calls chan string
}

func (r *recordingUserAPI) AddUser(ctx context.Context, serverID int64, tag, protocol, email, id, password, flow, method string) error {
	select {
	case r.calls <- flow:
	default:
	}
	return nil
}

func (r *recordingUserAPI) RemoveUser(ctx context.Context, serverID int64, tag, email string) error {
	return nil
}
