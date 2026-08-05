package main

import (
	"fmt"
	"os"
	"strings"

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
	case strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://"):
		dial = postgres.Open(dsn)
	case strings.HasPrefix(dsn, "mysql://"):
		dial = mysql.Open(strings.TrimPrefix(dsn, "mysql://"))
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
