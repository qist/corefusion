package repository

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"
)

// 删除客户必须带走全部从属数据，否则 SQLite 复用 ID 后会把旧数据串到新客户。
func TestPurgeClientScopedData(t *testing.T) {
	ctx := context.Background()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "purge.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	schema := []string{
		`CREATE TABLE clients (id INTEGER PRIMARY KEY, email TEXT)`,
		`CREATE TABLE tickets (id INTEGER PRIMARY KEY, client_id INTEGER)`,
		`CREATE TABLE ticket_messages (id INTEGER PRIMARY KEY, ticket_id INTEGER)`,
		`CREATE TABLE subscriptions (id INTEGER PRIMARY KEY, client_id INTEGER)`,
		`CREATE TABLE subscription_access_logs (id INTEGER PRIMARY KEY, subscription_id INTEGER)`,
		`CREATE TABLE subscription_rotations (id INTEGER PRIMARY KEY, subscription_id INTEGER)`,
		`CREATE TABLE traffic_sessions (id INTEGER PRIMARY KEY, subscription_id INTEGER)`,
		`CREATE TABLE traffic_usage_ledger (id INTEGER PRIMARY KEY, subscription_id INTEGER)`,
		`CREATE TABLE rate_limit_ip_entries (id INTEGER PRIMARY KEY, subscription_id INTEGER)`,
		`CREATE TABLE service_access_policies (id INTEGER PRIMARY KEY, subscription_id INTEGER)`,
		`CREATE TABLE charge_orders (id INTEGER PRIMARY KEY, client_id INTEGER)`,
		`CREATE TABLE plan_orders (id INTEGER PRIMARY KEY, client_id INTEGER)`,
		`CREATE TABLE balance_transactions (id INTEGER PRIMARY KEY, client_id INTEGER)`,
		`CREATE TABLE client_balances (id INTEGER PRIMARY KEY, client_id INTEGER)`,
		`CREATE TABLE client_gifts (id INTEGER PRIMARY KEY, client_id INTEGER)`,
		`CREATE TABLE client_log_packages (id INTEGER PRIMARY KEY, client_id INTEGER, subscription_id INTEGER)`,
		`CREATE TABLE client_oauth_bindings (id INTEGER PRIMARY KEY, client_id INTEGER)`,
		`CREATE TABLE client_self_unban_logs (id INTEGER PRIMARY KEY, client_id INTEGER)`,
		`CREATE TABLE ip_whitelist_entries (id INTEGER PRIMARY KEY, client_id INTEGER)`,
		`CREATE TABLE shared_account_repair_logs (id INTEGER PRIMARY KEY, client_id INTEGER)`,
		`CREATE TABLE warp_configs (id INTEGER PRIMARY KEY, client_id TEXT)`,
		`CREATE TABLE referral_records (id INTEGER PRIMARY KEY, referrer_id INTEGER, referee_id INTEGER)`,
		`CREATE TABLE referral_withdrawals (id INTEGER PRIMARY KEY, client_id INTEGER)`,
		`CREATE TABLE referral_codes (id INTEGER PRIMARY KEY, client_id INTEGER)`,
	}
	for _, q := range schema {
		if err := db.Exec(q).Error; err != nil {
			t.Fatalf("create table: %v (%s)", err, q)
		}
	}

	const victim, bystander = int64(7), int64(8)
	seed := []string{
		`INSERT INTO clients (id, email) VALUES (7, 'victim@test.local')`,
		`INSERT INTO clients (id, email) VALUES (8, 'other@test.local')`,
		`INSERT INTO tickets (id, client_id) VALUES (100, 7)`,
		`INSERT INTO ticket_messages (id, ticket_id) VALUES (200, 100)`,
		`INSERT INTO subscriptions (id, client_id) VALUES (300, 7)`,
		`INSERT INTO subscription_access_logs (id, subscription_id) VALUES (400, 300)`,
		`INSERT INTO subscription_rotations (id, subscription_id) VALUES (401, 300)`,
		`INSERT INTO traffic_sessions (id, subscription_id) VALUES (402, 300)`,
		`INSERT INTO traffic_usage_ledger (id, subscription_id) VALUES (403, 300)`,
		`INSERT INTO rate_limit_ip_entries (id, subscription_id) VALUES (404, 300)`,
		`INSERT INTO service_access_policies (id, subscription_id) VALUES (405, 300)`,
		`INSERT INTO charge_orders (id, client_id) VALUES (500, 7)`,
		`INSERT INTO plan_orders (id, client_id) VALUES (600, 7)`,
		`INSERT INTO balance_transactions (id, client_id) VALUES (700, 7)`,
		`INSERT INTO client_balances (id, client_id) VALUES (800, 7)`,
		`INSERT INTO client_gifts (id, client_id) VALUES (900, 7)`,
		`INSERT INTO client_log_packages (id, client_id, subscription_id) VALUES (901, 7, 300)`,
		`INSERT INTO client_oauth_bindings (id, client_id) VALUES (902, 7)`,
		`INSERT INTO client_self_unban_logs (id, client_id) VALUES (903, 7)`,
		`INSERT INTO ip_whitelist_entries (id, client_id) VALUES (904, 7)`,
		`INSERT INTO shared_account_repair_logs (id, client_id) VALUES (905, 7)`,
		`INSERT INTO warp_configs (id, client_id) VALUES (906, 'warp-uuid-7')`,
		`INSERT INTO referral_records (id, referrer_id, referee_id) VALUES (907, 7, 8)`,
		`INSERT INTO referral_withdrawals (id, client_id) VALUES (908, 7)`,
		`INSERT INTO referral_codes (id, client_id) VALUES (909, 7)`,
		// 旁证：另一个客户的数据不能被误删
		`INSERT INTO charge_orders (id, client_id) VALUES (501, 8)`,
		`INSERT INTO subscriptions (id, client_id) VALUES (301, 8)`,
		`INSERT INTO referral_records (id, referrer_id, referee_id) VALUES (910, 8, 9)`,
	}
	for _, q := range seed {
		if err := db.Exec(q).Error; err != nil {
			t.Fatalf("seed: %v (%s)", err, q)
		}
	}

	repo := NewClientRepository(db)
	if err := repo.PurgeClientScopedData(ctx, victim); err != nil {
		t.Fatalf("PurgeClientScopedData: %v", err)
	}
	if err := repo.Delete(ctx, victim); err != nil {
		t.Fatalf("Delete client: %v", err)
	}

	checks := map[string]string{
		"tickets":                    "client_id = 7",
		"ticket_messages":            "ticket_id = 100",
		"subscriptions":              "client_id = 7",
		"subscription_access_logs":   "subscription_id = 300",
		"subscription_rotations":     "subscription_id = 300",
		"traffic_sessions":           "subscription_id = 300",
		"traffic_usage_ledger":       "subscription_id = 300",
		"rate_limit_ip_entries":      "subscription_id = 300",
		"service_access_policies":    "subscription_id = 300",
		"charge_orders":              "client_id = 7",
		"plan_orders":                "client_id = 7",
		"balance_transactions":       "client_id = 7",
		"client_balances":            "client_id = 7",
		"client_gifts":               "client_id = 7",
		"client_log_packages":        "client_id = 7",
		"client_oauth_bindings":      "client_id = 7",
		"client_self_unban_logs":     "client_id = 7",
		"ip_whitelist_entries":       "client_id = 7",
		"shared_account_repair_logs": "client_id = 7",
		// warp_configs 的 client_id 是 WARP 注册返回的字符串 ID，不能按本地
		// clients.id 删除（PG 会直接报类型错误），因此不在清理范围。
		"referral_withdrawals": "client_id = 7",
		"referral_codes":       "client_id = 7",
		"referral_records":     "referrer_id = 7 OR referee_id = 7",
	}
	for table, cond := range checks {
		var n int64
		if err := db.Table(table).Where(cond).Count(&n).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if n != 0 {
			t.Errorf("%s 仍有 %d 条残留", table, n)
		}
	}

	// 旁证数据必须还在
	var others int64
	for _, q := range []string{
		`SELECT COUNT(*) FROM charge_orders WHERE client_id = 8`,
		`SELECT COUNT(*) FROM subscriptions WHERE client_id = 8`,
		`SELECT COUNT(*) FROM referral_records WHERE referrer_id = 8`,
		`SELECT COUNT(*) FROM clients WHERE id = 8`,
	} {
		var n int64
		if err := db.Raw(q).Scan(&n).Error; err != nil {
			t.Fatalf("count others: %v", err)
		}
		others += n
	}
	if others != 4 {
		t.Errorf("不应影响其他客户的数据，实际匹配 %d 条（期望 4）", others)
	}
}
