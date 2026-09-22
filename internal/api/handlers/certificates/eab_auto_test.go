package certificates

import "testing"

// 哪些 CA 强制要求 EAB ✓（真机验证过：ZeroSSL 报 externalAccountRequired ✓）
func TestCANeedsEAB(t *testing.T) {
	cases := map[string]bool{
		"google": true, "zerossl": true, " Google ": true, "ZEROSSL": true,
		"lets_encrypt": false, "lets_encrypt_staging": false, "": false,
	}
	for in, want := range cases {
		if got := caNeedsEAB(in); got != want {
			t.Fatalf("caNeedsEAB(%q)=%v 期望 %v", in, got, want)
		}
	}
}
