package ratelimitsvc

import (
	"context"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/dto"
	"corefusion/internal/model/entity"
	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/store/repository"
)

type fakeNodeOp struct {
	assoc []*agentpb.AssociateIPRequest
}

func (f *fakeNodeOp) AssociateIP(_ context.Context, _ int64, req *agentpb.AssociateIPRequest) error {
	if req != nil {
		f.assoc = append(f.assoc, req)
	}
	return nil
}
func (f *fakeNodeOp) ApplyLimits(context.Context, int64, *agentpb.ApplyLimitsRequest) error {
	return nil
}
func (f *fakeNodeOp) IsOnline(int64) bool { return true }
func (f *fakeNodeOp) GetRateLimitStatus(context.Context, int64, int64) (*dto.AgentRateLimitStatus, error) {
	return &dto.AgentRateLimitStatus{}, nil
}
func (f *fakeNodeOp) SetIPBlacklist(context.Context, int64, []string) error { return nil }

// 核心观测 IP 必须按节点归户，并跳过回环/私网；限速参数按「客户覆盖 > 套餐模板」解析。
func TestHandleObservedUsersAssociatesCoreIPs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("内存 SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(&entity.RateLimitConfig{}, &entity.Client{}, &entity.PlanTemplate{}); err != nil {
		t.Fatal(err)
	}

	// 节点 2 开启限速
	if err := db.Create(&entity.RateLimitConfig{ServerID: 2, Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	tmpl := &entity.PlanTemplate{
		Name: "p1", Category: entity.PlanCategoryBasic,
		SpeedLimitMbps: 100, SpeedLimitUpMbps: 50, ConcurrentLimit: 3, DeviceLimit: 2,
	}
	if err := db.Create(tmpl).Error; err != nil {
		t.Fatal(err)
	}
	subID := int64(7)
	cl := &entity.Client{Email: "a@example.com", PlanID: tmpl.ID, SubscriptionID: &subID, Status: "active"}
	if err := db.Create(cl).Error; err != nil {
		t.Fatal(err)
	}

	op := &fakeNodeOp{}
	svc := NewService(repository.NewRateLimitConfigRepository(db))
	svc.SetNodeOperator(op)
	svc.SetObservedDeps(repository.NewClientRepository(db), repository.NewPlanTemplateRepository(db))

	svc.HandleObservedUsers(context.Background(), 2, []ObservedUser{{
		// xray 账号 email 带 _tag 后缀，客户表里是 base email
		Email: "a@example.com_vless12345",
		IPs:   []string{"1.2.3.4", "127.0.0.1", "192.168.1.9", "2001:4860:4860::8888"},
	}})

	if len(op.assoc) != 2 {
		t.Fatalf("应只为 2 个公网 IP 下发关联，实际 %d: %+v", len(op.assoc), op.assoc)
	}
	seen := map[string]bool{}
	for _, req := range op.assoc {
		if req.SubscriptionId != subID {
			t.Fatalf("订阅 ID 应为 %d，实际 %d", subID, req.SubscriptionId)
		}
		if req.MaxDownMbps != 100 || req.MaxUpMbps != 50 || req.ConnLimit != 3 || req.DeviceLimit != 2 {
			t.Fatalf("限速参数应继承套餐，实际 %+v", req)
		}
		seen[req.Ip] = true
	}
	if !seen["1.2.3.4"] || !seen["2001:4860:4860::8888"] {
		t.Fatalf("公网 IP 关联缺失: %+v", seen)
	}
}

// 节点未启用限速时不应下发；邮箱查不到客户时也不应下发。
func TestHandleObservedUsersSkipsDisabledNodeAndUnknownEmail(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("内存 SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(&entity.RateLimitConfig{}, &entity.Client{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&entity.RateLimitConfig{ServerID: 3, Enabled: false}).Error; err != nil {
		t.Fatal(err)
	}

	op := &fakeNodeOp{}
	svc := NewService(repository.NewRateLimitConfigRepository(db))
	svc.SetNodeOperator(op)
	svc.SetObservedDeps(repository.NewClientRepository(db), nil)

	svc.HandleObservedUsers(context.Background(), 3, []ObservedUser{
		{Email: "unknown@example.com", IPs: []string{"1.2.3.4"}},
	})
	if len(op.assoc) != 0 {
		t.Fatalf("不应下发任何关联，实际 %+v", op.assoc)
	}
}

// proto ONLINE_USERS → 内部观测结构：每个 session 一个 IP，空邮箱/空 IP 丢弃。
func TestObservedUsersFromProto(t *testing.T) {
	users := ObservedUsersFromProto([]*agentpb.OnlineUserEntry{
		{
			SubscriptionId: 3,
			Email:          "a@example.com",
			Sessions: []*agentpb.OnlineSession{
				{Ip: "1.2.3.4"},
				{Ip: ""},
				{Ip: "2001:4860:4860::8888"},
			},
		},
		{Email: ""},
		{Email: "b@example.com"},
	})
	if len(users) != 1 {
		t.Fatalf("应只保留 1 个有效用户，实际 %d", len(users))
	}
	if users[0].Email != "a@example.com" || len(users[0].IPs) != 2 {
		t.Fatalf("转换结果不正确: %+v", users[0])
	}
}
