package nodeserver

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"
)

// 安全/正确性回归：删节点必须清子表，新建节点必须清掉同 ID 的孤儿行，
// 否则 ID 复用会继承上一个节点的配置（实测新节点拿到旧节点的空路由）。
func TestPurgeNodeScopedDataOnlyTouchesTargetServer(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "node.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	stmts := []string{
		`CREATE TABLE core_configs (id integer primary key autoincrement, server_id integer, core_type text, section text, config_json text)`,
		`CREATE TABLE inbound_configs (id integer primary key autoincrement, server_id integer, tag text)`,
		`CREATE TABLE plan_nodes (id integer primary key autoincrement, plan_id integer, inbound_config_id integer, enabled numeric)`,
		`CREATE TABLE server_metrics (id integer primary key autoincrement, server_id integer)`,
		`CREATE TABLE node_certificates (id integer primary key autoincrement, node_server_id integer, certificate_id integer, created_at datetime)`,
		`CREATE TABLE certificates (id integer primary key autoincrement, name text, issuer_node_id integer)`,
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	// 目标节点 20：旧节点残行。节点 19：必须原样保留。
	for _, q := range []struct {
		sql  string
		args []interface{}
	}{
		{`INSERT INTO core_configs (server_id, core_type, section, config_json) VALUES (?, 'xray', 'routing', '{"rules":[]}')`, []interface{}{20}},
		{`INSERT INTO core_configs (server_id, core_type, section, config_json) VALUES (?, 'xray', 'routing', '{"keep":true}')`, []interface{}{19}},
		{`INSERT INTO inbound_configs (server_id, tag) VALUES (?, 'stale-in')`, []interface{}{20}},
		{`INSERT INTO inbound_configs (server_id, tag) VALUES (?, 'keep-in')`, []interface{}{19}},
		{`INSERT INTO server_metrics (server_id) VALUES (?), (?), (?)`, []interface{}{20, 19, 20}},
		{`INSERT INTO node_certificates (node_server_id, certificate_id) VALUES (20, 2), (20, 3), (19, 2)`, nil},
		{`INSERT INTO certificates (name, issuer_node_id) VALUES ('from-20', 20), ('shared', 19)`, nil},
	} {
		if err := db.Exec(q.sql, q.args...).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	// 套餐引用了节点 20 的入站（id=1）与节点 19 的入站（id=2）。
	if err := db.Exec(`INSERT INTO plan_nodes (plan_id, inbound_config_id, enabled) VALUES (1, 1, 1), (1, 2, 1)`).Error; err != nil {
		t.Fatalf("seed plan_nodes: %v", err)
	}

	svc := &Service{db: db}
	if n := svc.purgeNodeScopedData(context.Background(), 20); n == 0 {
		t.Fatal("应当删除行，实际返回 0")
	}

	count := func(table, where string, args ...interface{}) int64 {
		var c int64
		if err := db.Table(table).Where(where, args...).Count(&c).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		return c
	}
	if c := count("core_configs", "server_id = ?", 20); c != 0 {
		t.Errorf("server 20 core_configs 残留 %d 行", c)
	}
	if c := count("inbound_configs", "server_id = ?", 20); c != 0 {
		t.Errorf("server 20 inbound_configs 残留 %d 行", c)
	}
	if c := count("server_metrics", "server_id = ?", 20); c != 0 {
		t.Errorf("server 20 server_metrics 残留 %d 行", c)
	}
	if c := count("node_certificates", "node_server_id = ?", 20); c != 0 {
		t.Errorf("server 20 证书绑定残留 %d 行", c)
	}
	if c := count("node_certificates", "node_server_id = ?", 19); c != 1 {
		t.Errorf("server 19 证书绑定被误删，剩 %d 行", c)
	}
	if c := count("certificates", "issuer_node_id = ?", 20); c != 0 {
		t.Errorf("server 20 签发的证书仍挂着 issuer，剩 %d 行", c)
	}
	if c := count("certificates", "issuer_node_id = ?", 19); c != 1 {
		t.Errorf("server 19 的 issuer 被误清，剩 %d 行", c)
	}
	// 节点 19 的数据不能被动到
	if c := count("core_configs", "server_id = ?", 19); c != 1 {
		t.Errorf("server 19 core_configs 被误删，剩 %d 行", c)
	}
	if c := count("inbound_configs", "server_id = ?", 19); c != 1 {
		t.Errorf("server 19 inbound_configs 被误删，剩 %d 行", c)
	}
	// 指向被删入站的套餐绑定要清掉，指向其他节点入站的保留
	if c := count("plan_nodes", "inbound_config_id = ?", 1); c != 0 {
		t.Errorf("plan_nodes 仍引用已删入站 %d 行", c)
	}
	if c := count("plan_nodes", "inbound_config_id = ?", 2); c != 1 {
		t.Errorf("plan_nodes 误删其它节点的绑定，剩 %d 行", c)
	}
}

// 表不存在（旧库/精简部署）时只跳过，不应报错中断。
func TestPurgeNodeScopedDataToleratesMissingTables(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "empty.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	svc := &Service{db: db}
	if n := svc.purgeNodeScopedData(context.Background(), 7); n != 0 {
		t.Fatalf("空库应返回 0，得到 %d", n)
	}
}
