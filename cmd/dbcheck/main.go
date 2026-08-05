package main

import (
	"fmt"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/driver/mysql"
	"github.com/qist/sqlite"
	"gorm.io/gorm"
)

// dbcheck: 打印给定 DSN 各表行数,用于迁移后一致性核对。
// 用法: go run ./cmd/dbcheck <DSN>
func main() {
	dsn := os.Args[1]
	var dial gorm.Dialector
	switch {
	case len(dsn) > 8 && dsn[:8] == "postgres":
		dial = postgres.Open(dsn)
	case len(dsn) > 5 && dsn[:5] == "mysql":
		dial = mysql.Open(dsn)
	default:
		dial = sqlite.Open(dsn)
	}
	db, err := gorm.Open(dial, &gorm.Config{})
	if err != nil {
		panic(err)
	}
	tables := []string{"users", "node_servers", "inbound_configs", "subscription_access_logs", "proxy_sites", "portal_templates", "server_metrics"}
	for _, t := range tables {
		var n int64
		db.Table(t).Count(&n)
		fmt.Printf("%-26s = %d\n", t, n)
	}
}
