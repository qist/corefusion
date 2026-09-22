package main

import (
	"strings"
	"testing"

	"corefusion/internal/auth/password"
)

// 安全审计 v5 S4：新部署不允许再出现可预测的固定初始口令。
func TestResolveInitialAdminPassword(t *testing.T) {
	// ① 环境变量优先（运维可控）
	got, generated := resolveInitialAdminPassword(func(k string) string {
		if k == initialAdminPasswordEnv {
			return "  my-own-strong-pass  "
		}
		return ""
	})
	if generated {
		t.Error("显式指定时不应标记为随机生成")
	}
	if got != "my-own-strong-pass" {
		t.Errorf("应使用环境变量里的口令（去掉首尾空白），得到 %q", got)
	}

	// ② 未指定 → 随机生成，足够长、且两次不同
	p1, gen1 := resolveInitialAdminPassword(func(string) string { return "" })
	p2, _ := resolveInitialAdminPassword(func(string) string { return "" })
	if !gen1 {
		t.Error("未指定时应标记为随机生成")
	}
	if len(p1) < 20 {
		t.Errorf("随机口令太短：%d", len(p1))
	}
	if p1 == p2 {
		t.Error("两次生成的随机口令不应相同")
	}
	if p1 == weakDefaultAdminPassword || p2 == weakDefaultAdminPassword {
		t.Error("随机口令绝不能等于历史弱默认口令")
	}
	// ③ nil getenv 也不能 panic（防御调用方）
	if pw, _ := resolveInitialAdminPassword(nil); pw == "" {
		t.Error("getenv 为 nil 时仍应生成口令")
	}
}

func TestAdminPasswordIsWeak(t *testing.T) {
	weakHash, err := password.Hash(weakDefaultAdminPassword)
	if err != nil {
		t.Fatal(err)
	}
	if !adminPasswordIsWeak(weakHash) {
		t.Error("默认弱口令必须被识别（否则启动告警失效）")
	}

	strongHash, err := password.Hash(generateAdminPassword(24))
	if err != nil {
		t.Fatal(err)
	}
	if adminPasswordIsWeak(strongHash) {
		t.Error("强口令不应被判为弱口令")
	}
	if adminPasswordIsWeak("not-a-bcrypt-hash") {
		t.Error("非法哈希不应误判为弱口令（bcrypt 比较失败即为 false）")
	}
}

func TestGenerateAdminPasswordCharset(t *testing.T) {
	pw := generateAdminPassword(64)
	if len(pw) != 64 {
		t.Fatalf("长度不符：%d", len(pw))
	}
	for _, r := range pw {
		if !strings.ContainsRune(passwordAlphabet, r) {
			t.Fatalf("出现字符集之外的字符 %q", r)
		}
	}
	// 易混淆字符必须被排除
	for _, bad := range []string{"0", "O", "1", "l", "I"} {
		if strings.Contains(pw, bad) {
			t.Errorf("字符集不应包含易混淆字符 %q（可能误读）", bad)
		}
	}
	// 零/负长度走默认值
	if len(generateAdminPassword(0)) != 24 {
		t.Error("长度 <=0 时应回退到默认长度")
	}
}
