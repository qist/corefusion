package main

import (
	"context"
	"crypto/rand"
	"log"
	"math/big"
	"strings"

	"corefusion/internal/auth/password"
	"corefusion/internal/store/repository"
)

// weakDefaultAdminPassword 是历史上写死的初始口令（`admin/admin123`）。
// 安全审计 v5 S4：新部署即存在**可预测**的管理员口令，且无强制改密。
const weakDefaultAdminPassword = "admin123"

// initialAdminPasswordEnv 允许运维显式指定初始口令（对接密钥管理/自动化部署）。
const initialAdminPasswordEnv = "CF_INITIAL_ADMIN_PASSWORD"

// passwordAlphabet 去掉了容易看错的字符（0/O、1/l/I）。
const passwordAlphabet = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#%^*-_=+"

// resolveInitialAdminPassword 决定**新建**部署的初始管理员口令：
//  1. 环境变量 `CF_INITIAL_ADMIN_PASSWORD` 优先（运维可控）；
//  2. 否则用 crypto/rand 随机生成（不再使用可预测的固定口令）。
//
// 返回 (口令, 是否为随机生成)。
func resolveInitialAdminPassword(getenv func(string) string) (string, bool) {
	if getenv != nil {
		if v := strings.TrimSpace(getenv(initialAdminPasswordEnv)); v != "" {
			return v, false
		}
	}
	return generateAdminPassword(24), true
}

// generateAdminPassword 生成 n 个字符的随机口令（crypto/rand，均匀取值）。
func generateAdminPassword(n int) string {
	if n <= 0 {
		n = 24
	}
	var b strings.Builder
	b.Grow(n)
	max := big.NewInt(int64(len(passwordAlphabet)))
	for i := 0; i < n; i++ {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			// crypto/rand 失败极罕见；此处不静默降级为弱随机，直接 panic 由启动流程暴露。
			panic("crypto/rand unavailable: " + err.Error())
		}
		b.WriteByte(passwordAlphabet[idx.Int64()])
	}
	return b.String()
}

// adminPasswordIsWeak 判断某条口令哈希是否仍是历史弱默认口令。
func adminPasswordIsWeak(passwordHash string) bool {
	return password.Check(weakDefaultAdminPassword, passwordHash)
}

// warnIfWeakAdminPassword 启动时检查现有管理员是否仍在使用历史弱默认口令，是则打印醒目告警。
//
// ⚠ 这里**只告警、绝不修改**现有口令 ✗ —— 静默改口令会把已有部署直接挡在门外（运维不知道新口令）。
// 新部署的口令由 resolveInitialAdminPassword 决定，根本不经过这里。
func warnIfWeakAdminPassword(ctx context.Context, userRepo *repository.UserRepository, username string) {
	if userRepo == nil {
		return
	}
	user, err := userRepo.GetByUsername(ctx, username)
	if err != nil || user == nil {
		return
	}
	if !adminPasswordIsWeak(user.PasswordHash) {
		return
	}
	log.Printf("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
	log.Printf("[security] 管理员账号 %q 仍在使用**默认弱口令** %q —— 面板一旦暴露在公网即可被接管。", username, weakDefaultAdminPassword)
	log.Printf("[security] 请立即在 面板 → 用户 → 修改密码 中改掉；本程序不会自动修改你的口令。")
	log.Printf("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
}
