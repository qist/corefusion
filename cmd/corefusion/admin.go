package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"corefusion/internal/auth/password"
	"corefusion/internal/store/db"
	"corefusion/internal/store/repository"
)

// 离线管理命令（= 恢复路径）。
//
// 为什么必须有它：2FA / 通行密钥（Passkey）一旦启用，唯一的超级管理员一旦丢了
// 手机或安全密钥，就会被**永久锁在门外**。所以"恢复能力"必须是与登录方式
// **相互独立**的一条通道 —— 这也是后续 2FA / Passkey 的硬前置。
//
// 设计约束：
//   - 只读写数据库，不启动 HTTP 服务，也不依赖面板能否登录；
//   - **不跑 AutoMigrate**：只想改个口令时不应该顺手动表结构；
//   - 改口令**同时** BumpTokenVersion → 立即吊销该账号所有已签发 token
//     （middleware.AuthRequired 每请求校验 token_version）；
//   - 随机生成的口令只打印一次，且不写入任何日志。

// userStatusActive 与 entity.User.Status 的取值对应（default:1）。
const userStatusActive = 1

// minAdminPasswordLen 是"建议下限"（与面板改密接口的 min=8 对齐）。
// CLI 是恢复通道，**只告警不拦截** —— 万一运维就是要设个临时短口令进门，也不该被挡住。
const minAdminPasswordLen = 8

func printAdminUsage() {
	fmt.Print(`corefusion admin - 离线管理命令（面板无法登录时使用）

Usage:
  corefusion admin reset-password [-u <用户名>] [-p <新口令>]
      重置指定账号的口令。省略 -p 时随机生成 24 位强口令并打印一次。
      执行后会立即吊销该账号的全部现有会话（token_version +1）。

  corefusion admin list
      列出账号（角色 / 状态 / 是否已设置口令 / 令牌版本），用于确认用户名。

Examples:
  corefusion admin reset-password
  corefusion admin reset-password -u admin
  corefusion admin reset-password -u ops -p 'my-strong-passphrase'

说明：面板运行中也可以执行，改动立即生效（无需重启）。
`)
}

// openAdminRepos 打开数据库并返回用户仓储与清理函数（不启动服务、不做迁移）。
func openAdminRepos() (*repository.UserRepository, func()) {
	env := loadEnv()
	dataDir := env["SAFE_PATHS"]
	if dataDir == "" {
		dataDir = "data"
	}
	if !filepath.IsAbs(dataDir) {
		if abs, err := filepath.Abs(dataDir); err == nil {
			dataDir = abs
		}
	}
	dbCfg := db.ConfigFromEnv(env, filepath.Join(dataDir, "sqlite", "app.db"))
	// 防呆：不在面板目录下执行时，SQLite 会**静默新建**一个空库 ✗，
	// 随后报 "no such table: users" 让人摸不着头脑。宁可在这里直接说清楚。
	if strings.TrimSpace(env["CF_DB_DSN"]) == "" {
		sqlitePath := filepath.Join(dataDir, "sqlite", "app.db")
		if _, err := os.Stat(sqlitePath); err != nil {
			fmt.Fprintf(os.Stderr, "✗ 找不到数据库：%s\n", sqlitePath)
			fmt.Fprintln(os.Stderr, "  请在面板目录（含 .env）下执行本命令，或设置 SAFE_PATHS / CF_DB_DSN 指向正确位置。")
			os.Exit(1)
		}
	}
	if err := db.Init(dbCfg); err != nil {
		fmt.Fprintf(os.Stderr, "✗ 无法打开数据库：%v\n", err)
		os.Exit(1)
	}
	return repository.NewUserRepository(db.DB), func() { db.Close() }
}

// runAdmin 是 `corefusion admin <subcommand>` 的入口。
func runAdmin(args []string) {
	if len(args) == 0 {
		printAdminUsage()
		os.Exit(2)
	}
	switch args[0] {
	case "reset-password":
		adminResetPassword(args[1:])
	case "list", "ls":
		adminList(args[1:])
	case "help", "--help", "-h":
		printAdminUsage()
	default:
		fmt.Fprintf(os.Stderr, "✗ 未知子命令：%s\n\n", args[0])
		printAdminUsage()
		os.Exit(2)
	}
}

// adminResetPassword 重置某个账号的口令并吊销其所有会话。
func adminResetPassword(args []string) {
	fs := flag.NewFlagSet("admin reset-password", flag.ExitOnError)
	username := "admin"
	fs.StringVar(&username, "u", "admin", "要重置的用户名")
	fs.StringVar(&username, "username", "admin", "要重置的用户名（等价于 -u）")
	newPwd := ""
	fs.StringVar(&newPwd, "p", "", "新口令；留空则随机生成 24 位强口令并打印一次")
	fs.StringVar(&newPwd, "password", "", "新口令（等价于 -p）")
	_ = fs.Parse(args) // ExitOnError：解析失败会自行退出

	repo, cleanup := openAdminRepos()
	defer cleanup()
	ctx := context.Background()

	user, err := repo.GetByUsername(ctx, strings.TrimSpace(username))
	if err != nil || user == nil {
		fmt.Fprintf(os.Stderr, "✗ 找不到用户 %q。\n", username)
		if users, _, lerr := repo.List(ctx, 20, 0); lerr == nil && len(users) > 0 {
			fmt.Fprintln(os.Stderr, "  现有账号（用 -u <用户名> 指定）：")
			for i := range users {
				fmt.Fprintf(os.Stderr, "    - %s（角色 %s，状态 %d）\n", users[i].Username, users[i].Role, users[i].Status)
			}
		}
		os.Exit(1)
	}

	plain := newPwd
	generated := false
	if strings.TrimSpace(plain) == "" {
		plain = generateAdminPassword(24)
		generated = true
	}

	hash, err := password.Hash(plain)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ 口令不符合要求：%v\n", err)
		os.Exit(1)
	}

	// 只更新 password_hash：不用 Save，避免把内存里的旧 token_version 写回去，
	// 覆盖掉期间别人 bump 出的新版本（那会让已吊销的 token 重新生效）。
	if err := repo.UpdatePasswordHash(ctx, user.ID, hash); err != nil {
		fmt.Fprintf(os.Stderr, "✗ 更新口令失败：%v\n", err)
		os.Exit(1)
	}
	// 立即吊销该账号所有已签发 token（对所有登录入口一视同仁）。
	if err := repo.BumpTokenVersion(ctx, user.ID); err != nil {
		fmt.Fprintf(os.Stderr, "⚠ 口令已更新，但吊销旧会话失败：%v\n", err)
		fmt.Fprintln(os.Stderr, "  请重启面板以确保旧 token 失效。")
	}

	fmt.Printf("✓ 已重置账号 %q（id=%d）的口令。\n", user.Username, user.ID)
	if generated {
		fmt.Printf("\n  新口令（仅显示这一次，请立即保存到密码管理器）：\n\n      %s\n\n", plain)
	}
	fmt.Println("  ✓ 该账号的全部现有会话已吊销（token_version +1），旧 token 立即失效。")
	// 注意：adminPasswordIsWeak 收的是**哈希**（内部做 bcrypt.Check），不是明文。
	if adminPasswordIsWeak(hash) {
		fmt.Fprintf(os.Stderr, "⚠ 你设置的是历史弱默认口令 %q —— 面板一旦暴露在公网即可被接管，请尽快改掉。\n", plain)
	}
	if len(plain) < minAdminPasswordLen {
		fmt.Fprintf(os.Stderr, "⚠ 新口令只有 %d 个字符（建议 ≥ %d）：这是恢复通道，允许你设短口令，但请登录后立即改强。\n",
			len(plain), minAdminPasswordLen)
	}
	if user.Status != userStatusActive {
		fmt.Fprintf(os.Stderr, "⚠ 该账号状态为 %d（非 %d = 停用），改完口令仍需把状态改回 %d 才能登录。\n",
			user.Status, userStatusActive, userStatusActive)
	}
	fmt.Println("  提示：登录后在 面板 → 用户 里改掉本次口令。")
}

// adminList 列出账号，便于确认用户名（改口令失败时最常用）。
func adminList(args []string) {
	repo, cleanup := openAdminRepos()
	defer cleanup()

	users, total, err := repo.List(context.Background(), 100, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ 读取账号失败：%v\n", err)
		os.Exit(1)
	}
	fmt.Printf("共 %d 个账号：\n\n", total)
	fmt.Printf("  %-5s %-24s %-14s %-6s %-12s %s\n", "ID", "用户名", "角色", "状态", "口令", "令牌版本")
	for i := range users {
		u := users[i]
		pwdState := "未设置"
		if strings.TrimSpace(u.PasswordHash) != "" {
			pwdState = "已设置"
		}
		status := fmt.Sprintf("%d", u.Status)
		if u.Status != userStatusActive {
			status += "（停用）"
		}
		fmt.Printf("  %-5d %-24s %-14s %-6s %-12s %d\n", u.ID, u.Username, u.Role, status, pwdState, u.TokenVersion)
	}
}
