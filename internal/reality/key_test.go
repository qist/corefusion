package reality

import (
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex %q: %v", s, err)
	}
	return b
}

// RFC 7748 §6.1 的经典 x25519 测试向量（Alice）。
func TestPublicKeyFromPrivate(t *testing.T) {
	priv := base64.RawURLEncoding.EncodeToString(mustHex(t, "77076d0a7318a57d3c16c17251b26645df4c2f87ebc0992ab177fba51db92c2a"))
	want := base64.RawURLEncoding.EncodeToString(mustHex(t, "8520f0098930a754748b7ddcb43ef75a0dbf3a0d26381af4eba4a98eaa9b4e6a"))

	if got := PublicKeyFromPrivate(priv); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	// 前后空白要容忍（用户从终端复制常带空格/换行）
	if got := PublicKeyFromPrivate("  " + priv + "\n"); got != want {
		t.Fatalf("trim failed: got %q", got)
	}
}

func TestPublicKeyFromPrivateInvalid(t *testing.T) {
	cases := []string{
		"",
		"not-base64!!",
		base64.RawURLEncoding.EncodeToString([]byte("too-short")),
	}
	for _, c := range cases {
		if got := PublicKeyFromPrivate(c); got != "" {
			t.Fatalf("input %q should be invalid, got %q", c, got)
		}
	}
}
