package migration

import (
	"encoding/json"
	"strings"
	"testing"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// 历史端点（core_configs.endpoints）必须搬到 endpoint_configs，并在全部落表后
// 清空文档段 —— 否则端点列表只显示新建的那条，生成的配置里却还有旧端点。
func TestMigrateSingboxEndpointsToTable(t *testing.T) {
	db := newMigrationTestDB(t, &entity.CoreConfig{}, &entity.EndpointConfig{})

	// 文档段：一条与端点表同名（以表为准），一条只在文档里（需要搬迁）
	docEndpoints := []map[string]interface{}{
		{"type": "wireguard", "tag": "wg-existing", "private_key": "doc-key"},
		{"type": "wireguard", "tag": "wg-legacy", "private_key": "legacy-key", "address": []string{"10.0.0.2/32"}},
	}
	docJSON, err := json.Marshal(docEndpoints)
	if err != nil {
		t.Fatal(err)
	}
	doc := &entity.CoreConfig{ServerID: 3, CoreType: "singbox", Section: "endpoints", ConfigJSON: string(docJSON)}
	if err := db.Create(doc).Error; err != nil {
		t.Fatal(err)
	}
	existing := &entity.EndpointConfig{
		Remark:       "wg-existing",
		CoreType:     "singbox",
		EndpointType: "wireguard",
		Tag:          "wg-existing",
		ConfigJSON:   `{"type":"wireguard","tag":"wg-existing","private_key":"table-key"}`,
		Enabled:      true,
		ServerID:     3,
	}
	if err := db.Create(existing).Error; err != nil {
		t.Fatal(err)
	}

	migrateSingboxEndpointsToTable(db)

	var rows []entity.EndpointConfig
	if err := db.Where("server_id = ? AND core_type = ?", 3, "singbox").Order("id ASC").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("期望端点表 2 条（已有 1 + 搬迁 1），实际 %d 条", len(rows))
	}
	// 同 tag 已存在 → 不覆盖端点表里的内容
	if got := repository.DecryptConfigJSON(rows[0].ConfigJSON); !strings.Contains(got, "table-key") {
		t.Fatalf("同 tag 记录应以端点表为准，实际 %s", got)
	}
	// 搬迁的记录：内容保留、凭据加密落库
	adopted := rows[1]
	if adopted.Tag != "wg-legacy" || adopted.EndpointType != "wireguard" {
		t.Fatalf("搬迁记录字段不符: %+v", adopted)
	}
	if !strings.HasPrefix(adopted.ConfigJSON, "enc:") {
		t.Fatalf("含私钥的端点应加密落库，实际 %s", adopted.ConfigJSON)
	}
	if got := repository.DecryptConfigJSON(adopted.ConfigJSON); !strings.Contains(got, "legacy-key") {
		t.Fatalf("搬迁记录内容不完整: %s", got)
	}

	// 全部覆盖 → 文档段清空
	var stored entity.CoreConfig
	if err := db.First(&stored, doc.ID).Error; err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(stored.ConfigJSON) != "[]" {
		t.Fatalf("文档段应清空，实际 %s", stored.ConfigJSON)
	}

	// 幂等：再跑一次不新增记录
	migrateSingboxEndpointsToTable(db)
	var count int64
	if err := db.Model(&entity.EndpointConfig{}).Where("server_id = ?", 3).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("迁移应幂等，实际 %d 条", count)
	}
}

// 文档段里存在无法落表的端点（无 tag）时必须保留文档段，避免运行时丢端点。
func TestMigrateSingboxEndpointsToTableKeepsUncoveredDoc(t *testing.T) {
	db := newMigrationTestDB(t, &entity.CoreConfig{}, &entity.EndpointConfig{})

	docEndpoints := []map[string]interface{}{
		{"type": "wireguard", "tag": "wg-legacy", "private_key": "legacy-key"},
		{"type": "wireguard", "private_key": "missing-tag"},
	}
	docJSON, _ := json.Marshal(docEndpoints)
	doc := &entity.CoreConfig{ServerID: 1, CoreType: "singbox", Section: "endpoints", ConfigJSON: string(docJSON)}
	if err := db.Create(doc).Error; err != nil {
		t.Fatal(err)
	}

	migrateSingboxEndpointsToTable(db)

	var count int64
	if err := db.Model(&entity.EndpointConfig{}).Where("server_id = ?", 1).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("合法端点应搬迁 1 条，实际 %d 条", count)
	}

	var stored entity.CoreConfig
	if err := db.First(&stored, doc.ID).Error; err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(stored.ConfigJSON) == "[]" {
		t.Fatal("存在未覆盖端点时不应清空文档段")
	}
}
