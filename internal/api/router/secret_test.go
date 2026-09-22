package router

import (
	"encoding/hex"
	"testing"
)

// 审计 v5 P-3：`router.New` 以前硬编码可猜的 `"corefusion-dev-secret"` ✗ —— 知道这个字符串
// 就能伪造通过鉴权的管理端 token。改为每次随机生成（库代码不该带全世界都知道的默认密钥 ✗）。
func TestRandomEphemeralSecretIsRandomAndStrong(t *testing.T) {
	a := RandomEphemeralSecret()
	b := RandomEphemeralSecret()

	if len(a) != 64 {
		t.Errorf("应为 32 字节 hex（64 字符），实际 %d 字符", len(a))
	}
	if _, err := hex.DecodeString(a); err != nil {
		t.Errorf("不是合法 hex：%v", err)
	}
	if a == b {
		t.Error("两次生成必须不同（否则等于固定密钥）")
	}
	if a == "corefusion-dev-secret" || b == "corefusion-dev-secret" {
		t.Fatal("不得再回退到可猜的默认密钥")
	}
}
