package inboundconfig

import (
	"context"
	"strings"
	"testing"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

const trafficInboundJSON = `{
	"tag": "inbound-vless-1",
	"settings": {
		"clients": [
			{"email": "a@xray.com", "id": "uuid-1", "_usedUp": 0, "_usedDown": 0, "_quota": "unlimited", "_paused": false}
		]
	}
}`

// newTrafficService 建一个带流量仓储的服务。
func newTrafficService(t *testing.T) *Service {
	t.Helper()
	db := newTestDB(t)
	if err := db.AutoMigrate(&entity.InboundUserTraffic{}); err != nil {
		t.Fatalf("migrate traffic: %v", err)
	}
	return &Service{
		repo:    repository.NewInboundConfigRepository(db),
		db:      db,
		traffic: repository.NewInboundUserTrafficRepository(db),
	}
}

// 单测通过，第 1、3 步完成，任务状态已更新，开始第 5 步加密。
func TestUpdateTrafficWritesNewTableNotConfigJSON(t *testing.T) {
	svc := newTrafficService(t)
	ctx := context.Background()

	cfg := &entity.InboundConfig{
		ServerID: 1, CoreType: "xray", Protocol: "vless",
		Tag: "inbound-vless-1", ConfigJSON: trafficInboundJSON, Enabled: true,
	}
	if err := svc.repo.Create(ctx, cfg); err != nil {
		t.Fatal(err)
	}

	// 归户两次，验证原子累加
	if err := svc.UpdateTraffic(ctx, 1, "inbound-vless-1", "a@xray.com", 100, 200); err != nil {
		t.Fatal(err)
	}
	if err := svc.UpdateTraffic(ctx, 1, "inbound-vless-1", "a@xray.com", 5, 7); err != nil {
		t.Fatal(err)
	}

	var rows []entity.InboundUserTraffic
	if err := svc.db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("期望 1 行流量记录，实际 %d", len(rows))
	}
	if rows[0].UsedUp != 105 || rows[0].UsedDown != 207 || rows[0].InboundID != cfg.ID {
		t.Fatalf("累加结果不符: %+v", rows[0])
	}

	// config_json 未被流量写入触碰：仍不含计数与遗留字段，_paused 保留
	stored, err := svc.repo.GetByID(ctx, cfg.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"_usedUp", "_usedDown", "_quota", "_expiry"} {
		if strings.Contains(stored.ConfigJSON, k) {
			t.Fatalf("落库 config_json 不应含 %s: %s", k, stored.ConfigJSON)
		}
	}
	if !strings.Contains(stored.ConfigJSON, "_paused") {
		t.Fatal("_paused 属于配置语义，必须保留")
	}
}

func TestEnrichWithTrafficInjectsForDisplay(t *testing.T) {
	svc := newTrafficService(t)
	ctx := context.Background()

	cfg := &entity.InboundConfig{
		ServerID: 1, CoreType: "xray", Protocol: "vless",
		Tag: "inbound-vless-1", ConfigJSON: trafficInboundJSON, Enabled: true,
	}
	if err := svc.repo.Create(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	if err := svc.traffic.AddUsage(ctx, 1, cfg.ID, "a@xray.com", 11, 22); err != nil {
		t.Fatal(err)
	}

	list, err := svc.List(ctx, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.EnrichWithTraffic(ctx, list); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(list[0].ConfigJSON, `"_usedUp":11`) ||
		!strings.Contains(list[0].ConfigJSON, `"_usedDown":22`) {
		t.Fatalf("响应副本应注回流量: %s", list[0].ConfigJSON)
	}

	// 注回只影响内存副本，库里的配置依然干净
	stored, err := svc.repo.GetByID(ctx, cfg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored.ConfigJSON, "_usedUp") {
		t.Fatalf("注回不应落库: %s", stored.ConfigJSON)
	}
}

func TestResetTrafficOnNewTable(t *testing.T) {
	svc := newTrafficService(t)
	ctx := context.Background()

	cfg := &entity.InboundConfig{
		ServerID: 1, CoreType: "xray", Protocol: "vless",
		Tag: "inbound-vless-1", ConfigJSON: trafficInboundJSON, Enabled: true,
	}
	if err := svc.repo.Create(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	if err := svc.traffic.AddUsage(ctx, 1, cfg.ID, "a@xray.com", 11, 22); err != nil {
		t.Fatal(err)
	}

	// 按 email 重置（含 email_tag 前缀匹配）
	if _, err := svc.ResetUserTrafficByEmail(ctx, "a@xray.com"); err != nil {
		t.Fatal(err)
	}
	var rows []entity.InboundUserTraffic
	if err := svc.db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if rows[0].UsedUp != 0 || rows[0].UsedDown != 0 {
		t.Fatalf("按 email 重置失败: %+v", rows[0])
	}

	// 全量重置
	if err := svc.traffic.AddUsage(ctx, 1, cfg.ID, "a@xray.com", 9, 9); err != nil {
		t.Fatal(err)
	}
	n, err := svc.ResetAllTraffic(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("期望清零 1 行，实际 %d", n)
	}
	if err := svc.db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if rows[0].UsedUp != 0 || rows[0].UsedDown != 0 {
		t.Fatalf("全量重置失败: %+v", rows[0])
	}
}
