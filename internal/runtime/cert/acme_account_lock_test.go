package cert

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// 同一账号文件的注册动作必须**串行**：Google / ZeroSSL 的 EAB 是一次性的，
// 并发签发（ECC + RSA）时两个 goroutine 都看到"未注册"→ 各自用同一把 EAB
// 去 new-account → 后一个必然 403（生产实测 2026-11-05：ECC 成功、RSA 失败）。
func TestAccountLockIsPerAccountPath(t *testing.T) {
	const p = "/tmp/acme/accounts/google.json"

	a := accountLock(p)
	b := accountLock(p)
	if a != b {
		t.Fatalf("same account path must share one mutex")
	}
	if other := accountLock(p + ".other"); other == a {
		t.Fatalf("different account paths must not share a mutex")
	}

	// 并发拿到的也必须是同一把（不能被 LoadOrStore 造出第二把）
	var wg sync.WaitGroup
	got := make([]*sync.Mutex, 20)
	for i := range got {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got[i] = accountLock(p)
		}(i)
	}
	wg.Wait()
	for i, m := range got {
		if m != a {
			t.Fatalf("concurrent accountLock[%d] returned a different mutex", i)
		}
	}
}

// 展示名必须归一到同一个账号文件：否则 "Let's Encrypt" 会另建一份
// `Let's Encrypt.json`，判断不出"账号已注册"，还会重复注册。
func TestNewACMEClientNormalizesCAForAccountPath(t *testing.T) {
	c := NewACMEClient("/data", "ops@example.com", CAProvider("Let's Encrypt"))
	if got, want := c.accountPath(), filepath.Join("/data", "acme", "accounts", "lets_encrypt.json"); got != want {
		t.Fatalf("accountPath = %q, want %q", got, want)
	}
	if c.caURL != CALetsEncrypt {
		t.Fatalf("caURL = %q, want %q", c.caURL, CALetsEncrypt)
	}

	g := NewACMEClient("/data", "ops@example.com", CAProvider("Google Trust Services LLC"))
	if got, want := g.accountPath(), filepath.Join("/data", "acme", "accounts", "google.json"); got != want {
		t.Fatalf("google accountPath = %q, want %q", got, want)
	}
	if g.caURL != CAGooglePublicCA {
		t.Fatalf("google caURL = %q, want %q", g.caURL, CAGooglePublicCA)
	}
}

// 账号已注册时 prepareClient 必须**直接复用**，不再走注册（离线可验证：
// 该路径不访问网络；若还去注册，这里会因目录 URL 不可达而失败）。
func TestPrepareClientReusesRegisteredAccount(t *testing.T) {
	dir := t.TempDir()
	accountDir := filepath.Join(dir, "acme", "accounts")
	if err := os.MkdirAll(accountDir, 0o750); err != nil {
		t.Fatal(err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})

	acct := AccountData{
		Email:           "ops@example.com",
		CAURL:           CAGooglePublicCA,
		KeyPEM:          string(keyPEM),
		RegistrationURI: "https://acme.example/account/1",
	}
	raw, err := json.Marshal(acct)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(accountDir, "google.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	c := NewACMEClient(dir, "ops@example.com", CAProviderGoogle)
	client, err := c.prepareClient(context.Background(), nil)
	if err != nil {
		t.Fatalf("prepareClient with registered account should not hit the network: %v", err)
	}
	if client == nil {
		t.Fatalf("prepareClient returned nil client")
	}
}
