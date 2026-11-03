package twofa

import (
	"strings"
	"testing"
)

// TestRecoveryCodeFormatAndEntropy 锁定安全审计 v6 M-8：
// 新恢复码必须 5 组 4 位（10 字节 = 80 bit 熵），且不能重复。
func TestRecoveryCodeFormatAndEntropy(t *testing.T) {
	seen := make(map[string]bool, 200)
	for i := 0; i < 200; i++ {
		code := randomRecoveryCode()
		parts := strings.Split(code, "-")
		if len(parts) != 5 {
			t.Fatalf("recovery code %q should have 5 groups", code)
		}
		for _, p := range parts {
			if len(p) != 4 {
				t.Fatalf("recovery code %q group %q should be 4 hex chars", code, p)
			}
			for _, r := range p {
				if !strings.ContainsRune("0123456789ABCDEF", r) {
					t.Fatalf("recovery code %q contains non-hex char %q", code, r)
				}
			}
		}
		if seen[code] {
			t.Fatalf("duplicate recovery code generated: %q", code)
		}
		seen[code] = true
	}
}

// 旧格式（3 组，48 bit）仍必须能被 hashCode/FindUnusedByHash 校验，保证升级兼容。
func TestLegacyRecoveryCodeHashCompatible(t *testing.T) {
	h := hashCode("AAAA-BBBB-CCCC")
	if len(h) != 64 {
		t.Fatalf("legacy code hash should still be 64 hex chars, got %d", len(h))
	}
	if h != hashCode("aaaa-bbbb-cccc") {
		t.Fatal("hash should be case-insensitive")
	}
}
