package proxy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// TestNginxConfigGlobalTLS 锁定全局 TLS 指令的生成：ssl_dhparam / ssl_session_ticket_key
// 由全局开关控制，写入 http 上下文（各站点共用 data/ssl/ 文件）。
func TestNginxConfigGlobalTLS(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "tls.example.com", ListenPort: "443", CertPath: "certs/tls.example.com/cert.pem", KeyPath: "certs/tls.example.com/key.pem", Enabled: true, Path: "/", Upstream: "127.0.0.1:8080"},
	}

	// 关闭：不生成任何全局 TLS 指令
	cfg := generateFor(t, newNginxTestManager(t, sites, `{}`))
	if strings.Contains(cfg, "ssl_dhparam") || strings.Contains(cfg, "ssl_session_ticket_key") {
		t.Fatalf("开关关闭时不应生成全局 TLS 指令：\n%s", cfg)
	}

	// 开启：生成 dhparam 与两份会话票据密钥
	cfg = generateFor(t, newNginxTestManager(t, sites, `{"ssl_dhparam":true,"ssl_session_ticket":true}`))
	for _, want := range []string{
		"ssl_dhparam ../ssl/dhparam.pem;",
		"ssl_session_ticket_key ../ssl/tls_session_ticket.key;",
		"ssl_session_ticket_key ../ssl/tlsb_session_ticket.key;",
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("缺少 %q：\n%s", want, cfg)
		}
	}
}

// TestNginxConfigOCSPStapling 锁定 OCSP stapling 生成：除 on/verify 外，还必须给出
// ssl_trusted_certificate（fullchain 链）与 resolver，否则 nginx 无法拉取/校验 OCSP。
func TestNginxConfigOCSPStapling(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "tls.example.com", ListenPort: "443", CertPath: "certs/tls.example.com/cert.pem", KeyPath: "certs/tls.example.com/key.pem", EnableStapling: true, Enabled: true, Path: "/", Upstream: "127.0.0.1:8080"},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{"ssl_resolver":"8.8.8.8 valid=600s"}`))
	for _, want := range []string{
		"ssl_stapling on;",
		"ssl_stapling_verify on;",
		"ssl_trusted_certificate ../certs/tls.example.com/fullchain.pem;",
		"resolver 8.8.8.8 valid=600s;",
		"resolver_timeout 10s;",
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("缺少 %q：\n%s", want, cfg)
		}
	}
}

// TestNginxConfigOCSPResolverOptional 锁定 resolver 的「可关闭」语义：
//   - 留空 → 不生成 resolver / resolver_timeout（改用系统 resolver）；
//   - 非空 → 按配置生成。
func TestNginxConfigOCSPResolverOptional(t *testing.T) {
	sites := func() []entity.NginxSite {
		return []entity.NginxSite{
			{ServerID: 2, Domain: "tls.example.com", ListenPort: "443", CertPath: "certs/tls.example.com/cert.pem", KeyPath: "certs/tls.example.com/key.pem", EnableStapling: true, Enabled: true, Path: "/", Upstream: "127.0.0.1:8080"},
		}
	}

	// 留空 = 关闭：不生成 resolver，但保留 stapling 本体
	cfg := generateFor(t, newNginxTestManager(t, sites(), `{"ssl_resolver":""}`))
	if strings.Contains(cfg, "resolver ") || strings.Contains(cfg, "resolver_timeout") {
		t.Fatalf("resolver 留空时不应生成 resolver 指令：\n%s", cfg)
	}
	if !strings.Contains(cfg, "ssl_stapling on;") {
		t.Fatalf("留空只关 resolver，stapling 本体应保留：\n%s", cfg)
	}

	// 非空 = 生成
	cfg = generateFor(t, newNginxTestManager(t, sites(), `{"ssl_resolver":"8.8.8.8 1.1.1.1"}`))
	if !strings.Contains(cfg, "resolver 8.8.8.8 1.1.1.1;") {
		t.Fatalf("应生成配置的 resolver：\n%s", cfg)
	}
}

// TestWriteGlobalSslFilesSessionTicket 验证会话票据密钥从 base64 还原落地：
// 两份、各 48 字节、内容一致时幂等不重写。
func TestWriteGlobalSslFilesSessionTicket(t *testing.T) {
	dir := t.TempDir()
	b64, err := GenerateSessionTicketB64()
	if err != nil {
		t.Fatalf("GenerateSessionTicketB64 失败：%v", err)
	}
	if err := WriteGlobalSslFiles(dir, "", b64, b64); err != nil {
		t.Fatalf("WriteGlobalSslFiles 失败：%v", err)
	}
	for _, name := range []string{sslTicketKeyName, sslTicketKeyBName} {
		b, err := os.ReadFile(filepath.Join(dir, sslDirRel, name))
		if err != nil {
			t.Fatalf("读取 %s 失败：%v", name, err)
		}
		if len(b) != sslTicketKeyLength {
			t.Fatalf("%s 长度=%d，期望 %d", name, len(b), sslTicketKeyLength)
		}
	}
	// 再次写入相同内容不应改动文件（会话票据不因重复同步失效）
	first, _ := os.ReadFile(filepath.Join(dir, sslDirRel, sslTicketKeyName))
	if err := WriteGlobalSslFiles(dir, "", b64, b64); err != nil {
		t.Fatalf("二次 WriteGlobalSslFiles 失败：%v", err)
	}
	again, _ := os.ReadFile(filepath.Join(dir, sslDirRel, sslTicketKeyName))
	if string(first) != string(again) {
		t.Fatal("内容未变时会话票据密钥不应被重写")
	}
}
