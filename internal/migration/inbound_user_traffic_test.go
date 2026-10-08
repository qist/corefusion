package migration

import (
	"encoding/json"
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

const inboundTrafficLegacyJSON = `{
	"tag": "inbound-vless-1",
	"settings": {
		"clients": [
			{"email": "a@xray.com", "id": "uuid-1", "_usedUp": 100, "_usedDown": 200, "_quota": "unlimited", "_expiry": null, "_paused": false},
			{"email": "b@xray.com", "id": "uuid-2"}
		]
	}
}`

// 存量 config_json 里的流量计数必须被搬到 inbound_user_traffic，
// 面板元数据被清掉、配置语义字段（_paused）保留，且整个迁移幂等。
func TestMigrateInboundUserTraffic(t *testing.T) {
	db := newMigrationTestDB(t, &entity.InboundConfig{}, &entity.InboundUserTraffic{})

	cfg := &entity.InboundConfig{
		ServerID:   3,
		CoreType:   "xray",
		Protocol:   "vless",
		Tag:        "inbound-vless-1",
		ConfigJSON: inboundTrafficLegacyJSON,
		Enabled:    true,
	}
	if err := db.Create(cfg).Error; err != nil {
		t.Fatal(err)
	}

	migrateInboundUserTraffic(db)

	// 1) 计数落到新表
	var rows []entity.InboundUserTraffic
	if err := db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("期望 1 行流量记录，实际 %d 行", len(rows))
	}
	row := rows[0]
	if row.ServerID != 3 || row.InboundID != cfg.ID || row.Email != "a@xray.com" || row.UsedUp != 100 || row.UsedDown != 200 {
		t.Fatalf("流量行内容不符: %+v", row)
	}

	// 2) config_json 已清理，_paused 保留
	var stored entity.InboundConfig
	if err := db.First(&stored, cfg.ID).Error; err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"_usedUp", "_usedDown", "_quota", "_expiry"} {
		if strings.Contains(stored.ConfigJSON, k) {
			t.Fatalf("config_json 仍包含 %s: %s", k, stored.ConfigJSON)
		}
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(stored.ConfigJSON), &parsed); err != nil {
		t.Fatalf("清理后不再是合法 JSON: %v", err)
	}
	clients := parsed["settings"].(map[string]interface{})["clients"].([]interface{})
	first := clients[0].(map[string]interface{})
	if _, ok := first["_paused"]; !ok {
		t.Fatal("_paused 属于配置语义，必须保留")
	}
	if first["id"] != "uuid-1" {
		t.Fatal("用户凭据不能被误删")
	}

	// 3) 幂等：再跑一次不会重复累加
	migrateInboundUserTraffic(db)
	var again []entity.InboundUserTraffic
	if err := db.Find(&again).Error; err != nil {
		t.Fatal(err)
	}
	if len(again) != 1 || again[0].UsedUp != 100 || again[0].UsedDown != 200 {
		t.Fatalf("迁移不幂等: %+v", again)
	}
}
