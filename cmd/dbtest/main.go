package main

import (
	"log"
	"os"

	"corefusion/internal/migration"
	"corefusion/internal/store/db"
)

// dbtest: 验证 CoreFusion 在 SQLite / MySQL / PostgreSQL 之间的数据库相互迁移。
// 用法:
//   CF_DB_DSN=<源DSN> go run ./cmd/dbtest <目标DSN> [<目标DSN2> ...]
// 源 DSN 默认取环境变量 CF_DB_DSN(当前运行的库),目标 DSN 作为位置参数。
// 每个目标: TestConnection -> db.MigrateTo(建表+拷贝) -> migration.AutoMigrate(补全索引/修复)
func main() {
	srcDSN := os.Getenv("CF_DB_DSN")
	if srcDSN == "" {
		srcDSN = "data/sqlite/app.db"
	}
	srcCfg := db.ParseDSN(srcDSN)
	if err := db.Init(srcCfg); err != nil {
		log.Fatalf("init source (%s): %v", srcCfg.Type, err)
	}
	db.RecordStartup(srcCfg)
	defer db.Close()
	log.Printf("源库已连接: type=%s", srcCfg.Type)

	targets := os.Args[1:]
	if len(targets) == 0 {
		log.Fatal("用法: go run ./cmd/dbtest <目标DSN> [<目标DSN2> ...]")
	}

	for _, t := range targets {
		if err := migrateToTarget(srcCfg, t); err != nil {
			log.Printf("!!! 迁移到 %s 失败: %v", t, err)
			os.Exit(1)
		}
	}
	log.Println("全部目标迁移完成")
}

func migrateToTarget(srcCfg db.Config, targetDSN string) error {
	dstCfg := db.ParseDSN(targetDSN)
	log.Printf(">>> 目标: type=%s", dstCfg.Type)

	// 1. 连通性测试
	if err := db.TestConnection(dstCfg); err != nil {
		return err
	}
	log.Printf("    连通性 OK")

	// 2. 建表 + 拷贝数据(内置 AutoMigrate + 逐表拷贝)
	tables, rows, err := db.MigrateTo(dstCfg)
	if err != nil {
		return err
	}
	log.Printf("    已拷贝 %d 表 / %d 行", tables, rows)

	// 3. 补全索引/数据修复(幂等,不破坏已写入数据)
	if err := db.Init(dstCfg); err != nil {
		return err
	}
	if err := migration.AutoMigrate(db.DB); err != nil {
		return err
	}
	defer db.Close()
	log.Printf("<<< 完成: %s", dstCfg.Type)
	return nil
}
