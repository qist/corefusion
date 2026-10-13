package proxy

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// 全局 TLS 文件（相对 dataDir）——所有 TLS server 共用，落盘在 data/ssl/。
// 素材以 base64 存库（NginxSettings.Ssl*B64），开启后由面板/各节点还原到磁盘，
// 保证多节点一致（会话票据密钥跨节点共享，客户端可在不同节点复用会话）。
const (
	sslDirRel          = "ssl"
	sslDhparamName     = "dhparam.pem"
	sslTicketKeyName   = "tls_session_ticket.key"
	sslTicketKeyBName  = "tlsb_session_ticket.key"
	sslTicketKeyLength = 48 // openssl rand 48
)

// nginxSslPath 返回 nginx 配置中引用 data/ssl/<name> 的路径。
// nginx 配置前缀是 data/proxy（证书也写作 ../certs/...），故用 "../ssl/..."。
func nginxSslPath(name string) string {
	return "../" + sslDirRel + "/" + name
}

// trustChainRel 依据站点证书路径推断同目录下的 fullchain 链文件（OCSP 校验用）。
func trustChainRel(certRel string) string {
	if filepath.Base(certRel) == "fullchain.pem" {
		return certRel
	}
	return filepath.ToSlash(filepath.Join(filepath.Dir(certRel), "fullchain.pem"))
}

// WriteGlobalSslFiles 把 base64 素材还原到 data/ssl/（幂等：内容未变则不重写）。
func WriteGlobalSslFiles(dataDir, dhparamB64, ticketB64, ticketBB64 string) error {
	dir := filepath.Join(dataDir, sslDirRel)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	items := []struct {
		name string
		b64  string
		mode os.FileMode
	}{
		{sslDhparamName, dhparamB64, 0600},
		{sslTicketKeyName, ticketB64, 0600},
		{sslTicketKeyBName, ticketBB64, 0600},
	}
	for _, it := range items {
		if strings.TrimSpace(it.b64) == "" {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(it.b64))
		if err != nil {
			return fmt.Errorf("解码 %s 失败：%w", it.name, err)
		}
		p := filepath.Join(dir, it.name)
		if b, rerr := os.ReadFile(p); rerr == nil && string(b) == string(raw) {
			continue
		}
		if err := os.WriteFile(p, raw, it.mode); err != nil {
			return fmt.Errorf("写入 %s 失败：%w", it.name, err)
		}
		log.Printf("[proxy] 已从数据库还原全局 TLS 文件：%s", p)
	}
	return nil
}

// GenerateDhparamB64 生成 2048 位 DH 参数并返回 base64（openssl dhparam，较慢）。
func GenerateDhparamB64() (string, error) {
	if _, err := exec.LookPath("openssl"); err != nil {
		return "", fmt.Errorf("生成 dhparam 需要 openssl：%w", err)
	}
	f, err := os.CreateTemp("", "cf-dhparam-*.pem")
	if err != nil {
		return "", err
	}
	tmpPath := f.Name()
	f.Close()
	defer os.Remove(tmpPath)
	if out, err := exec.Command("openssl", "dhparam", "-out", tmpPath, "2048").CombinedOutput(); err != nil {
		return "", fmt.Errorf("openssl dhparam 生成失败：%v：%s", err, string(out))
	}
	b, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// GenerateSessionTicketB64 生成一份 48 字节随机会话票据密钥并返回 base64。
func GenerateSessionTicketB64() (string, error) {
	buf := make([]byte, sslTicketKeyLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}

// EnsureGlobalSSLMaterial 确保当前节点开启的全局 TLS 素材已在库（base64），
// 缺失则生成并写回；本机再还原到 data/ssl/。用于「保存开关」时自动补齐，
// 避免在生成配置（可能高频）时触发耗时的 dhparam 生成。
func (m *NginxManager) EnsureGlobalSSLMaterial(ctx context.Context) (*NginxSettings, error) {
	s, err := m.GetNginxSettings(ctx)
	if err != nil {
		return nil, err
	}
	changed := false
	if s.SslDhparam && s.SslDhparamB64 == "" {
		if b64, err := GenerateDhparamB64(); err != nil {
			log.Printf("[NginxManager] 生成 dhparam 失败：%v", err)
		} else {
			s.SslDhparamB64 = b64
			changed = true
		}
	}
	// 两份密钥必须互不相同，任一缺失就重新生成一对
	if s.SslSessionTicket && (s.SslSessionTicketB64 == "" || s.SslSessionTicketBB64 == "") {
		a, err1 := GenerateSessionTicketB64()
		b, err2 := GenerateSessionTicketB64()
		if err1 != nil || err2 != nil {
			log.Printf("[NginxManager] 生成会话票据密钥失败：%v / %v", err1, err2)
		} else {
			s.SslSessionTicketB64, s.SslSessionTicketBB64 = a, b
			changed = true
		}
	}
	if changed {
		if err := m.SaveNginxSettings(ctx, s); err != nil {
			return nil, err
		}
	}
	if m.isLocal && (s.SslDhparam || s.SslSessionTicket) {
		if err := WriteGlobalSslFiles(m.dataDir, s.SslDhparamB64, s.SslSessionTicketB64, s.SslSessionTicketBB64); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// RegenerateGlobalSSLMaterial 重新生成素材（供前端「生成/替换」按钮），存库并还原到本机磁盘。
// target: dhparam | session_ticket。
func (m *NginxManager) RegenerateGlobalSSLMaterial(ctx context.Context, target string) (*NginxSettings, error) {
	s, err := m.GetNginxSettings(ctx)
	if err != nil {
		return nil, err
	}
	switch target {
	case "dhparam":
		b64, err := GenerateDhparamB64()
		if err != nil {
			return nil, err
		}
		s.SslDhparamB64 = b64
	case "session_ticket":
		a, err := GenerateSessionTicketB64()
		if err != nil {
			return nil, err
		}
		b, err := GenerateSessionTicketB64()
		if err != nil {
			return nil, err
		}
		s.SslSessionTicketB64, s.SslSessionTicketBB64 = a, b
	default:
		return nil, fmt.Errorf("未知 target：%s", target)
	}
	if err := m.SaveNginxSettings(ctx, s); err != nil {
		return nil, err
	}
	if m.isLocal {
		if err := WriteGlobalSslFiles(m.dataDir, s.SslDhparamB64, s.SslSessionTicketB64, s.SslSessionTicketBB64); err != nil {
			return nil, err
		}
	}
	return s, nil
}
