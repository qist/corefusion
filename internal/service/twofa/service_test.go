package twofa

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"corefusion/internal/model/entity"
)

// TestValidateTOTP 用同一库生成动态码再验证：时间窗口 ±1 内必须通过，错误码必须失败。
func TestValidateTOTP(t *testing.T) {
	svc := &Service{issuer: "test"}
	secret := "JBSWY3DPEHPK3PXP"
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}
	if !svc.validateTOTP(secret, code) {
		t.Fatal("正确动态码应通过（同一时刻生成）")
	}
	if svc.validateTOTP(secret, "000000") {
		t.Fatal("错误动态码不应通过")
	}
	if svc.validateTOTP(secret, "12345") {
		t.Fatal("长度不足 6 位不应通过")
	}
	if svc.validateTOTP(secret, "1234567") {
		t.Fatal("超过 6 位不应通过")
	}
	// 相邻时间步长（±30s）也应通过（Skew=1）
	codeNext, err := totp.GenerateCode(secret, time.Now().Add(30*time.Second))
	if err != nil {
		t.Fatalf("generate next code: %v", err)
	}
	if !svc.validateTOTP(secret, codeNext) {
		t.Fatal("相邻时间窗口的动态码应通过（Skew=1）")
	}
}

// TestBeginSetup 验证 BeginSetup 生成可用的 secret 与 otpauth URI（无 DB 依赖）。
func TestBeginSetup(t *testing.T) {
	svc := &Service{issuer: "CoreFusion Panel", pending: newPendingLoginStore()}
	user := &entity.User{Username: "admin", TOTPEnabled: false}
	secret, uri, err := svc.BeginSetup(context.Background(), user)
	if err != nil {
		t.Fatalf("begin setup: %v", err)
	}
	if len(secret) < 16 {
		t.Fatalf("secret 长度异常: %q", secret)
	}
	if !strings.Contains(uri, "otpauth://totp/") || !strings.Contains(uri, "secret="+secret) {
		t.Fatalf("otpauth URI 异常: %q", uri)
	}
	// 生成的 secret 能通过自我校验
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}
	if !svc.validateTOTP(secret, code) {
		t.Fatal("BeginSetup 生成的 secret 应能通过 validateTOTP")
	}
	// 已启用时拒绝重复 setup
	enabled := &entity.User{Username: "admin", TOTPEnabled: true}
	if _, _, err := svc.BeginSetup(context.Background(), enabled); err == nil {
		t.Fatal("已启用 2FA 时应拒绝再次 setup")
	}
}

// TestRecoveryCodeFormat 验证恢复码格式（XXXX-XXXX-XXXX）与 hash 一致性。
func TestRecoveryCodeFormat(t *testing.T) {
	for i := 0; i < 50; i++ {
		code := randomRecoveryCode()
		parts := strings.Split(code, "-")
		if len(parts) != 3 || len(parts[0]) != 4 || len(parts[1]) != 4 || len(parts[2]) != 4 {
			t.Fatalf("恢复码格式异常: %q", code)
		}
		if hashCode(code) != hashCode(strings.ToLower(code)) {
			t.Fatal("hashCode 应大小写不敏感（登录时先统一大写再 hash）")
		}
	}
}

// TestPendingLoginStore 验证 pending login 票据的一次性与过期。
func TestPendingLoginStore(t *testing.T) {
	store := newPendingLoginStore()
	tok := store.create(1, "admin", 5*time.Minute)
	if tok == "" {
		t.Fatal("create 应返回 token")
	}
	p, ok := store.take(tok)
	if !ok || p.UserID != 1 || p.Username != "admin" {
		t.Fatalf("take 应返回票据，got %+v ok=%v", p, ok)
	}
	// 一次性：重复 take 必须失败
	if _, ok := store.take(tok); ok {
		t.Fatal("票据应只能取一次")
	}

	// 过期票据
	store2 := newPendingLoginStore()
	tok2 := store2.create(2, "ops", -time.Second)
	if _, ok := store2.take(tok2); ok {
		t.Fatal("过期票据不应可取")
	}

	// 随机 token 不应碰撞
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		tok := store.create(1, "admin", time.Minute)
		if seen[tok] {
			t.Fatal("token 碰撞")
		}
		seen[tok] = true
	}
}