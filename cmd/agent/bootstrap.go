package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net"
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
		a.logCertExpiryWarning()
		return nil
	}

	// 过期/损坏要先说清楚，否则用户只会看到 mTLS 握手失败的一行错误。
	if exp, ok := a.agentCertExpiry(); ok {
		if left := time.Until(exp); left <= 0 {
			log.Printf("[Agent] local mTLS certificate EXPIRED at %s (%.1f days ago) — the panel cannot authenticate this node",
				exp.Format(time.RFC3339), -left.Hours()/24)
		} else {
			log.Printf("[Agent] local mTLS certificate is not usable (expires %s)", exp.Format(time.RFC3339))
		}
	}

	if a.cfg.Bootstrap.Token == "" {
		return fmt.Errorf("no bootstrap token and no valid cert — configure bootstrap.token in config.yaml")
	}

	log.Println("[Agent] bootstrapping with panel...")

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "agent-" + strconv.FormatInt(a.cfg.Panel.ServerID, 10)
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"server_id":       a.cfg.Panel.ServerID,
		"bootstrap_token": a.cfg.Bootstrap.Token,
		"hostname":        hostname,
		"agent_version":   constants.Version,
	})

	endpoints, err := a.bootstrapEndpoints()
	if err != nil {
		return err
	}
	var lastErr error
	for _, ep := range endpoints {
		body, status, err := a.postBootstrap(ep, reqBody)
		if err != nil {
			lastErr = fmt.Errorf("%s endpoint: %w", ep.label, err)
			log.Printf("[Agent] bootstrap via %s failed: %v", ep.label, err)
			continue
		}
		if status != http.StatusOK {
			lastErr = fmt.Errorf("%s endpoint: HTTP %d: %s", ep.label, status, strings.TrimSpace(string(body)))
			log.Printf("[Agent] bootstrap via %s failed: HTTP %d", ep.label, status)
			continue
		}

		var apiResp struct {
			Code string `json:"code"`
			Data struct {
				ServerID  int64  `json:"server_id"`
				CertPEM   string `json:"cert_pem"`
				KeyPEM    string `json:"key_pem"`
				CACertPEM string `json:"ca_cert_pem"`
				ExpiresAt int64  `json:"cert_expires_at"`
			} `json:"data"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(body, &apiResp); err != nil {
			lastErr = fmt.Errorf("parse bootstrap response from %s: %w", ep.label, err)
			log.Printf("[Agent] bootstrap via %s returned invalid JSON: %v", ep.label, err)
			continue
		}
		if apiResp.Code != "" && apiResp.Code != "OK" {
			lastErr = fmt.Errorf("bootstrap error from %s: %s - %s", ep.label, apiResp.Code, apiResp.Message)
			log.Printf("[Agent] bootstrap via %s rejected: %s", ep.label, apiResp.Message)
			continue
		}
		if apiResp.Data.ServerID != 0 && apiResp.Data.ServerID != a.cfg.Panel.ServerID {
			lastErr = fmt.Errorf("bootstrap response server_id=%d does not match config server_id=%d",
				apiResp.Data.ServerID, a.cfg.Panel.ServerID)
			log.Printf("[Agent] bootstrap via %s returned server_id=%d, want %d", ep.label, apiResp.Data.ServerID, a.cfg.Panel.ServerID)
			continue
		}

		result := bootstrapResult{
			CertPEM:   apiResp.Data.CertPEM,
			KeyPEM:    apiResp.Data.KeyPEM,
			CACertPEM: apiResp.Data.CACertPEM,
			ExpiresAt: apiResp.Data.ExpiresAt,
		}
		if err := a.installBootstrapResult(result, ep.label); err != nil {
			lastErr = err
			log.Printf("[Agent] bootstrap via %s rejected: %v", ep.label, err)
			continue
		}
		log.Printf("[Agent] bootstrap successful via %s, cert expires at %v", ep.label, time.Unix(result.ExpiresAt, 0))
		return nil
	}
	return fmt.Errorf("bootstrap request failed: %w", lastErr)
}

type bootstrapEndpoint struct {
	url     string
	tlsCfg  *tls.Config
	label   string
	timeout time.Duration
}

type bootstrapResult struct {
	CertPEM   string
	KeyPEM    string
	CACertPEM string
	ExpiresAt int64
}

// bootstrapEndpoints returns the endpoints to try.
//
// The public CDN/reverse-proxy endpoint (PublicHost/Port/TLS) is preferred: the
// origin's direct listener is often a bare HTTP port that gets closed for
// safety, while the public entry is the same address that served the install
// script and is guaranteed to reach the panel API. The direct listener stays as
// a fallback.
//
// If the direct attempt would be cleartext HTTP, it is dropped entirely when a
// TLS public endpoint exists: the one-time bootstrap token must never travel in
// cleartext over the internet.
//
// Trust differs per hop: the public endpoint verifies the proxy's certificate
// with the system roots, the direct endpoint uses the panel CA. The issued
// Agent certificate is verified against the panel CA in both cases.
func (a *AgentApp) bootstrapEndpoints() ([]bootstrapEndpoint, error) {
	tlsCfg, directErr := a.bootstrapTLSConfig()
	if a.cfg.Panel.TLS && directErr != nil {
		// 安全审计 v7 L-5：直连 TLS 没有信任锚点（无 CA、无指纹、未开 TOFU）→ 直连不可用。
		// 如果还有公共 HTTPS 入口，仍可只走公共入口；否则 fail closed。
		log.Printf("[Agent] bootstrap: direct TLS endpoint unavailable: %v", directErr)
	}
	direct := bootstrapEndpoint{
		url:     bootstrapURL(a.cfg.Panel.Host, a.cfg.Panel.Port, a.cfg.Panel.TLS),
		tlsCfg:  tlsCfg,
		label:   "direct",
		timeout: 8 * time.Second,
	}

	publicHost := strings.TrimSpace(a.cfg.Panel.PublicHost)
	if publicHost == "" {
		if directErr != nil {
			return nil, directErr
		}
		return []bootstrapEndpoint{direct}, nil
	}
	publicPort := a.cfg.Panel.PublicPort
	if publicPort <= 0 {
		publicPort = a.cfg.Panel.Port
	}
	if publicHost == a.cfg.Panel.Host &&
		publicPort == a.cfg.Panel.Port &&
		a.cfg.Panel.PublicTLS == a.cfg.Panel.TLS {
		if directErr != nil {
			return nil, directErr
		}
		return []bootstrapEndpoint{direct}, nil
	}

	var publicTLS *tls.Config
	if a.cfg.Panel.PublicTLS {
		// Trust the CDN/proxy's public certificate. The Agent certificate in the
		// response is verified separately against the panel CA, so terminating
		// TLS at the CDN cannot substitute its own Agent certificate.
		publicTLS = &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: publicHost,
		}
	}
	publicEndpoint := bootstrapEndpoint{
		url:     bootstrapURL(publicHost, publicPort, a.cfg.Panel.PublicTLS),
		tlsCfg:  publicTLS,
		label:   "public",
		timeout: 30 * time.Second,
	}

	// 直连 TLS 不可用（缺信任锚点）：只能走公共 HTTPS；公共入口若不加密则 fail closed。
	if directErr != nil {
		if publicEndpoint.tlsCfg == nil {
			return nil, directErr
		}
		return []bootstrapEndpoint{publicEndpoint}, nil
	}

	switch {
	case publicEndpoint.tlsCfg != nil && direct.tlsCfg == nil:
		// 公共入口走 HTTPS 而直连是明文：只走公共入口，绝不把 token 裸奔。
		return []bootstrapEndpoint{publicEndpoint}, nil
	case direct.tlsCfg == nil && publicEndpoint.tlsCfg == nil:
		// 两端都是明文（纯内网）：直连优先，公共兜底。
		return []bootstrapEndpoint{direct, publicEndpoint}, nil
	default:
		// 公共 HTTPS 入口优先（源站端口可能被关闭），直连仅作兜底。
		return []bootstrapEndpoint{publicEndpoint, direct}, nil
	}
}

func bootstrapURL(host string, port int, useTLS bool) string {
	scheme := "http"
	if useTLS {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/api/v1/agent/bootstrap", scheme, net.JoinHostPort(host, strconv.Itoa(port)))
}

func (a *AgentApp) postBootstrap(ep bootstrapEndpoint, reqBody []byte) ([]byte, int, error) {
	client := &http.Client{Timeout: ep.timeout}
	if ep.tlsCfg != nil {
		client.Transport = &http.Transport{TLSClientConfig: ep.tlsCfg}
	}
	resp, err := client.Post(ep.url, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, resp.StatusCode, readErr
	}
	return body, resp.StatusCode, nil
}

func (a *AgentApp) installBootstrapResult(result bootstrapResult, source string) error {
	if strings.TrimSpace(result.CertPEM) == "" || strings.TrimSpace(result.KeyPEM) == "" {
		return fmt.Errorf("bootstrap response from %s has empty cert/key", source)
	}
	if err := verifyBootstrapCert(result.CertPEM, result.KeyPEM, a.caPath, a.cfg.Panel.ServerID); err != nil {
		return fmt.Errorf("verify bootstrap certificate from %s: %w", source, err)
	}
	if err := os.WriteFile(a.certPath, []byte(result.CertPEM), 0640); err != nil {
		return fmt.Errorf("write cert: %w", err)
	}
	if err := os.WriteFile(a.keyPath, []byte(result.KeyPEM), 0600); err != nil {
		return fmt.Errorf("write key: %w", err)
	}
	if result.CACertPEM != "" {
		info, statErr := os.Stat(a.caPath)
		if os.IsNotExist(statErr) || (statErr == nil && info.Size() == 0) {
			if err := os.MkdirAll(filepath.Dir(a.caPath), 0750); err != nil {
				return fmt.Errorf("create CA dir: %w", err)
			}
			if err := os.WriteFile(a.caPath, []byte(result.CACertPEM), 0640); err != nil {
				return fmt.Errorf("write CA cert: %w", err)
			}
		} else {
			// Never overwrite an existing trust anchor with a bundle supplied by
			// the remote endpoint; a public TLS endpoint could be a CDN.
			log.Printf("[Agent] keeping existing CA %s (bootstrap response also carried a CA bundle)", a.caPath)
		}
	}
	return nil
}

// verifyBootstrapCert checks that the issued certificate belongs to this
// server and that its private key matches. If the panel CA is present, the
// certificate must also chain to it — this is what makes the public/CDN
// fallback safe even though that hop uses a public TLS certificate.
func verifyBootstrapCert(certPEM, keyPEM, caPath string, serverID int64) error {
	certBlock, _ := pem.Decode([]byte(certPEM))
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		return fmt.Errorf("invalid certificate PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return fmt.Errorf("parse certificate: %w", err)
	}
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("certificate is not valid before %v", cert.NotBefore)
	}
	if now.After(cert.NotAfter) {
		return fmt.Errorf("certificate expired at %v", cert.NotAfter)
	}
	if cert.IsCA {
		return fmt.Errorf("bootstrap response returned a CA certificate instead of an Agent certificate")
	}
	if serverID > 0 {
		wantOU := fmt.Sprintf("server-%d", serverID)
		found := false
		for _, ou := range cert.Subject.OrganizationalUnit {
			if ou == wantOU {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("certificate OU does not identify %s", wantOU)
		}
	}

	keyBlock, _ := pem.Decode([]byte(keyPEM))
	if keyBlock == nil {
		return fmt.Errorf("invalid private key PEM")
	}
	var key any
	switch keyBlock.Type {
	case "EC PRIVATE KEY":
		key, err = x509.ParseECPrivateKey(keyBlock.Bytes)
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	case "PRIVATE KEY":
		key, err = x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	default:
		return fmt.Errorf("unsupported private key type %q", keyBlock.Type)
	}
	if err != nil {
		return fmt.Errorf("parse private key: %w", err)
	}

	certPub, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		return fmt.Errorf("marshal certificate public key: %w", err)
	}
	keyPub, err := x509.MarshalPKIXPublicKey(publicKeyOf(key))
	if err != nil {
		return fmt.Errorf("marshal private key public part: %w", err)
	}
	if !bytes.Equal(certPub, keyPub) {
		return fmt.Errorf("certificate public key does not match returned private key")
	}

	caPEM, err := os.ReadFile(caPath)
	if err != nil || len(caPEM) == 0 {
		return nil // legacy/TOFU path: no local CA to verify against
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return fmt.Errorf("parse panel CA %s failed", caPath)
	}
	if _, err := cert.Verify(x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err != nil {
		return fmt.Errorf("certificate does not chain to panel CA: %w", err)
	}
	return nil
}

func publicKeyOf(key any) any {
	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	case *rsa.PrivateKey:
		return &k.PublicKey
	default:
		return key
	}
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

	// 没有面板 CA：**不再自动信任首连证书**（安全审计 v7 L-5）。
	// 首装时处于网络中间位置的人原本可以固定自己的证书并截获 bootstrap token，
	// 进而冒用节点身份。现在信任锚点只能来自：
	//   ① 配置里显式填写的 bootstrap.panel_cert_fingerprint（带外核对）；
	//   ② 历史版本已经固定过的 pin 文件（本地信任锚点，不会被远端改写）；
	//   ③ 显式打开 bootstrap.allow_tofu（首连自动固定，仍有 MITM 风险）。
	pinPath := a.panelTLSPinPath()
	trustAnchor := normalizeCertFingerprint(a.cfg.Bootstrap.PanelCertFingerprint)
	if trustAnchor == "" {
		trustAnchor = readPinnedFingerprint(pinPath)
	}
	if trustAnchor == "" && !a.cfg.Bootstrap.AllowTOFU {
		return nil, fmt.Errorf("bootstrap: 面板未提供 CA，也没有已固定的证书指纹 —— "+
			"请使用面板生成的安装脚本（脚本会随带面板 CA）；或在本机核对面板证书后把 SHA-256 "+
			"写入 bootstrap.panel_cert_fingerprint（openssl s_client -connect %s:%d -showcerts | "+
			"openssl x509 -fingerprint -sha256 -noout）；或显式设置 bootstrap.allow_tofu: true（不安全）",
			a.cfg.Panel.Host, a.cfg.Panel.Port)
	}
	if trustAnchor == "" {
		log.Printf("[Agent] bootstrap: WARNING bootstrap.allow_tofu=true 且没有已固定的指纹，首次连接的面板证书将被信任")
	}
	cfg.InsecureSkipVerify = true // 仅关闭"内置根证书校验"，下面自己做严格校验
	cfg.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		fp := certChainFingerprint(rawCerts)
		if fp == "" {
			return fmt.Errorf("panel presented no certificate")
		}
		// ⚠ 每次校验都**重新读**固定值 ✗：不能在构建配置时读一次就捕获 ——
		// 那样首次固定后，本次进程内的后续校验仍拿旧值（空），等于没固定（单元测试抓到过 ✓）。
		want := normalizeCertFingerprint(a.cfg.Bootstrap.PanelCertFingerprint)
		if want == "" {
			want = readPinnedFingerprint(pinPath)
		}
		if want == "" {
			if !a.cfg.Bootstrap.AllowTOFU {
				return fmt.Errorf("panel certificate is not pinned (fingerprint %s); "+
					"set bootstrap.panel_cert_fingerprint to trust it", fp)
			}
			if err := writePinnedFingerprint(pinPath, fp); err != nil {
				return fmt.Errorf("record panel certificate pin: %w", err)
			}
			log.Printf("[Agent] bootstrap: pinned panel certificate %s… (%s)", fp[:16], pinPath)
			return nil
		}
		if !strings.EqualFold(fp, want) {
			got, pinned := fp, want
			if len(got) > 16 {
				got = got[:16]
			}
			if len(pinned) > 16 {
				pinned = pinned[:16]
			}
			return fmt.Errorf("panel certificate changed (pinned %s…, got %s…); refusing — "+
				"if the panel was legitimately re-keyed, update bootstrap.panel_cert_fingerprint "+
				"or remove %s to re-pin deliberately", pinned, got, pinPath)
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
	return normalizeCertFingerprint(string(b))
}

func writePinnedFingerprint(path, fp string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(fp+"\n"), 0600)
}

// hasValidCert checks whether the on-disk mTLS certificate is present and not expired.
// agentCertExpiry 返回本地 mTLS 证书的过期时间（读不到证书时 ok=false）。
func (a *AgentApp) agentCertExpiry() (time.Time, bool) {
	cert, err := tls.LoadX509KeyPair(a.certPath, a.keyPath)
	if err != nil || len(cert.Certificate) == 0 {
		return time.Time{}, false
	}
	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return time.Time{}, false
	}
	return parsed.NotAfter, true
}

// HasCertFiles 表示本地已经有证书文件（不判断是否过期）。
// 用于区分"首次安装、压根没证书"和"有证书但过期需要重新引导"。
func (a *AgentApp) HasCertFiles() bool {
	if _, err := os.Stat(a.certPath); err != nil {
		return false
	}
	if _, err := os.Stat(a.keyPath); err != nil {
		return false
	}
	_, err := os.Stat(a.caPath)
	return err == nil
}

// logCertExpiryWarning 在证书临近过期时给出一条可操作的提醒。
// 面板长期关停会导致续签推送不到位 —— 这里提前把时间点喊出来。
func (a *AgentApp) logCertExpiryWarning() {
	exp, ok := a.agentCertExpiry()
	if !ok {
		return
	}
	left := time.Until(exp)
	if left <= 0 {
		log.Printf("[Agent] mTLS certificate expired at %s", exp.Format(time.RFC3339))
		return
	}
	if left < 7*24*time.Hour {
		log.Printf("[Agent] mTLS certificate expires in %.1f days (%s); the panel renews it while connected",
			left.Hours()/24, exp.Format(time.RFC3339))
	}
}

// retryBootstrapLoop 在证书不可用且引导失败后周期性重试。
//
// 为什么不直接退出：面板长期关停（周末/假期）后证书可能过期，此时引导令牌通常
// 也已用过或过期 → 引导必失败。以前的实现是 log.Fatalf + systemd 无限重启，
// 节点只会留下"连不上"的假象。现在保持进程存活（核心守护/本地配置照常运行），
// 日志给出明确的修复步骤，并在管理员重新下发令牌后自动恢复。
func (a *AgentApp) retryBootstrapLoop(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.EnsureCert(); err != nil {
				log.Printf("[Agent] bootstrap retry failed: %v", err)
				continue
			}
			log.Println("[Agent] bootstrap succeeded on retry, mTLS connection will resume")
			return
		}
	}
}

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
