package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"corefusion/internal/constants"
)

// EnsureCert loads an existing mTLS certificate or bootstraps a new one from the Panel.
func (a *AgentApp) EnsureCert() error {
	if a.hasValidCert() {
		log.Println("[Agent] using existing mTLS certificate")
		return nil
	}

	if a.cfg.Bootstrap.Token == "" {
		return fmt.Errorf("no bootstrap token and no valid cert — configure bootstrap.token in config.yaml")
	}

	log.Println("[Agent] bootstrapping with panel...")

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "agent-" + strconv.FormatInt(a.cfg.Panel.ServerID, 10)
	}

	scheme := "http"
	if a.cfg.Panel.TLS {
		scheme = "https"
	}
	bootstrapURL := fmt.Sprintf("%s://%s:%d/api/v1/agent/bootstrap", scheme, a.cfg.Panel.Host, a.cfg.Panel.Port)
	reqBody, _ := json.Marshal(map[string]interface{}{
		"server_id":       a.cfg.Panel.ServerID,
		"bootstrap_token": a.cfg.Bootstrap.Token,
		"hostname":        hostname,
		"agent_version":   constants.Version,
	})

	// ⚠ 不再 `InsecureSkipVerify: true` ✗（安全审计 v5 M6）：引导阶段同样要校验面板身份，
	// 否则任何能 MITM 的人都能冒充面板、把 Agent 引导到自己的假面板上（拿到 Agent 的私钥 ✗）。
	// 两条路径**都不需要人工填写**任何东西：
	//   ① 安装脚本会自动把面板 CA 证书写到 certs/ca-cert.pem（面板自动附带）→ 用它做正常校验；
	//   ② 万一没有 CA（老脚本安装的节点）→ TOFU：把首次看到的证书指纹自动记到
	//      certs/panel-tls-pin，之后每次引导都比对它（不一致即拒绝）。
	tlsCfg, err := a.bootstrapTLSConfig()
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	if tlsCfg != nil {
		client.Transport = &http.Transport{TLSClientConfig: tlsCfg}
	}
	resp, err := client.Post(bootstrapURL, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("bootstrap request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("bootstrap failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Code string `json:"code"`
		Data struct {
			CertPEM   string `json:"cert_pem"`
			KeyPEM    string `json:"key_pem"`
			CACertPEM string `json:"ca_cert_pem"`
			ExpiresAt int64  `json:"cert_expires_at"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("parse bootstrap response: %w", err)
	}
	if apiResp.Code != "" && apiResp.Code != "OK" {
		return fmt.Errorf("bootstrap error: %s - %s", apiResp.Code, apiResp.Message)
	}

	if err := os.WriteFile(a.certPath, []byte(apiResp.Data.CertPEM), 0640); err != nil {
		return fmt.Errorf("write cert: %w", err)
	}
	if err := os.WriteFile(a.keyPath, []byte(apiResp.Data.KeyPEM), 0600); err != nil {
		return fmt.Errorf("write key: %w", err)
	}
	if err := os.WriteFile(a.caPath, []byte(apiResp.Data.CACertPEM), 0640); err != nil {
		return fmt.Errorf("write CA cert: %w", err)
	}

	log.Printf("[Agent] bootstrap successful, cert expires at %v", time.Unix(apiResp.Data.ExpiresAt, 0))
	return nil
}

// bootstrapTLSConfig 组装引导阶段的 TLS 配置（返回 nil 表示面板走 http，无需 TLS）。
//
// 优先级：① 有面板 CA → 标准链校验（ServerName 用面板证书名，因为面板证书是自签 CA 签发、
// SAN 只有 corefusion-panel/localhost，而 Agent 连的是 IP/任意主机名）；
// ② 没 CA → TOFU：首次接受并把证书指纹落盘，之后必须一致（自动，不需要人工填指纹）。
func (a *AgentApp) bootstrapTLSConfig() (*tls.Config, error) {
	if !a.cfg.Panel.TLS {
		return nil, nil
	}

	cfg := &tls.Config{MinVersion: tls.VersionTLS13, ServerName: panelCertServerName}

	if caPEM, err := os.ReadFile(a.caPath); err == nil && len(caPEM) > 0 {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("parse panel CA %s failed", a.caPath)
		}
		cfg.RootCAs = pool
		log.Printf("[Agent] bootstrap: verifying panel against CA %s", a.caPath)
		return cfg, nil
	}

	// TOFU 路径：没有 CA 可依赖，就把首次看到的证书指纹固定下来。
	pinPath := a.panelTLSPinPath()
	log.Printf("[Agent] bootstrap: no CA available, using trust-on-first-use pin %s", pinPath)
	cfg.InsecureSkipVerify = true // 仅关闭"内置根证书校验"，下面自己做严格校验
	cfg.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		fp := certChainFingerprint(rawCerts)
		if fp == "" {
			return fmt.Errorf("panel presented no certificate")
		}
		// ⚠ 每次校验都**重新读**固定值 ✗：不能在构建配置时读一次就捕获 ——
		// 那样首次固定后，本次进程内的后续校验仍拿旧值（空），等于没固定（单元测试抓到过 ✓）。
		pinned := readPinnedFingerprint(pinPath)
		if pinned == "" {
			if err := writePinnedFingerprint(pinPath, fp); err != nil {
				return fmt.Errorf("record panel certificate pin: %w", err)
			}
			log.Printf("[Agent] bootstrap: pinned panel certificate %s… (%s)", fp[:16], pinPath)
			return nil
		}
		if !strings.EqualFold(fp, pinned) {
			return fmt.Errorf("panel certificate changed (pinned %s…, got %s…); refusing — "+
				"remove %s to re-pin deliberately", pinned[:16], fp[:16], pinPath)
		}
		return nil
	}
	return cfg, nil
}

// panelTLSPinPath 是 TOFU 指纹的落盘位置（数据目录内，Agent 独占）。
func (a *AgentApp) panelTLSPinPath() string {
	return filepath.Join(filepath.Dir(a.caPath), "panel-tls-pin")
}

// certChainFingerprint 返回证书链里**最后一环**（根/签发者）的 SHA-256，用于固定面板身份。
// 用根而不是叶子：面板的服务端证书是每次启动在内存里重新签发的，叶子指纹每次都会变。
func certChainFingerprint(rawCerts [][]byte) string {
	if len(rawCerts) == 0 {
		return ""
	}
	sum := sha256.Sum256(rawCerts[len(rawCerts)-1])
	return hex.EncodeToString(sum[:])
}

func readPinnedFingerprint(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func writePinnedFingerprint(path, fp string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(fp+"\n"), 0600)
}

// hasValidCert checks whether the on-disk mTLS certificate is present and not expired.
func (a *AgentApp) hasValidCert() bool {
	cert, err := tls.LoadX509KeyPair(a.certPath, a.keyPath)
	if err != nil {
		return false
	}
	if _, err := os.ReadFile(a.caPath); err != nil {
		return false
	}
	for _, c := range cert.Certificate {
		parsed, err := x509.ParseCertificate(c)
		if err != nil {
			continue
		}
		if time.Now().After(parsed.NotAfter) {
			return false
		}
	}
	return true
}
