package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"corefusion/internal/store/repository"
)

// 面板 / 订阅 / 门户共用同一份 TLS 材料，但**必须能热加载** ✓：
//
//   - 证书中心续期/重签后，`certs/<host>/cert.pem` 已经换成新文件 ✓，
//     旧实现只在启动时 `tls.LoadX509KeyPair` 一次 ✗ → 必须重启面板才生效 ✗。
//     浏览器的旧连接继续用旧证书可以接受 ✓，但**新连接必须立刻用新证书** ✓。
//   - mTLS 的客户端 CA（`panel.mtls_ca_path`）换掉/续期后同样要即时生效 ✓。
//   - 在设置页改证书路径、开关 mTLS 也不该要求重启 ✓（监听地址/端口除外 ✗）。
//
// 做法：TLSConfig 里不放 Certificates ✗，改用 GetCertificate + GetConfigForClient ✓，
// 每次握手按文件 (mtime,size) 判断是否需要重读；没有变化时只 stat 两次 ✓。

// fileStamp 用 (mtime, size) 判断文件是否变过。
type fileStamp struct {
	mod  time.Time
	size int64
}

func statStamp(path string) (fileStamp, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return fileStamp{}, false
	}
	return fileStamp{mod: fi.ModTime(), size: fi.Size()}, true
}

// panelTLS 持有服务器证书与 mTLS 客户端 CA，并按需热加载。
type panelTLS struct {
	mu sync.Mutex

	certPath, keyPath, caPath string
	mtls                      bool

	cert                *tls.Certificate
	certStamp, keyStamp fileStamp

	pool    *x509.CertPool
	caStamp fileStamp

	// lastMsg/lastAt 用于抑制重复日志（每 10s 巡检一次，坏文件不该刷屏）。
	lastMsg string
}

// newPanelTLS 启动时加载一次：证书必须成功（与原 ServeTLS 行为一致，失败即启动失败）；
// mTLS CA 失败只告警（与原行为一致：跳过 mTLS，不阻断面板启动）。
func newPanelTLS(certPath, keyPath, caPath string, mtls bool) (*panelTLS, error) {
	p := &panelTLS{certPath: certPath, keyPath: keyPath, caPath: caPath, mtls: mtls}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.loadCertLocked(); err != nil {
		return nil, err
	}
	if p.mtls {
		p.ensureCALocked()
	}
	return p, nil
}

// Config 为每个 http.Server 生成**独立**的 tls.Config ✓。
//
// ⚠ 不能三个 server 共用同一个 *tls.Config ✗：http.Server.ServeTLS 会往
// NextProtos 里写 h2，多个 goroutine 同时写同一份配置是数据竞争 ✓。
func (p *panelTLS) Config() *tls.Config {
	cfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		// 与 http.Server 默认一致；GetConfigForClient 返回的配置必须显式带上 ✓，
		// 否则 ALPN 只剩 http/1.1（h2 会失效 ✗）。
		NextProtos:     []string{"h2", "http/1.1"},
		GetCertificate: p.getCertificate,
	}
	// base 要传进去：http.Server 可能按自身协议配置调整过 NextProtos，
	// 返回的新配置必须沿用调整后的结果（写死容易和实际协议能力不一致 ✗）。
	cfg.GetConfigForClient = func(*tls.ClientHelloInfo) (*tls.Config, error) {
		return p.getConfigForClient(cfg)
	}
	return cfg
}

// getCertificate 每次握手检查证书文件是否更新（续期后无需重启）。
func (p *panelTLS) getCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.ensureCertLocked(); err != nil {
		return nil, err
	}
	if p.cert == nil {
		return nil, fmt.Errorf("no server certificate available")
	}
	return p.cert, nil
}

// getConfigForClient 只在 mTLS 开启时覆盖配置，并顺带热加载客户端 CA。
// mTLS 关闭时返回 nil → 沿用原配置（行为与改造前一致 ✓）。
func (p *panelTLS) getConfigForClient(base *tls.Config) (*tls.Config, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.mtls {
		return nil, nil
	}
	p.ensureCALocked()
	if p.pool == nil {
		// 与原启动逻辑一致：CA 不可用时跳过 mTLS（并留日志），不阻断面板访问。
		p.logOnce("WARNING: mTLS enabled but client CA is unavailable; serving without client auth")
		return nil, nil
	}
	protos := []string{"h2", "http/1.1"}
	if base != nil && len(base.NextProtos) > 0 {
		protos = append([]string(nil), base.NextProtos...)
	}
	return &tls.Config{
		MinVersion:     tls.VersionTLS12,
		NextProtos:     protos,
		GetCertificate: p.getCertificate,
		ClientAuth:     tls.RequireAndVerifyClientCert,
		ClientCAs:      p.pool,
	}, nil
}

// apply 在设置巡检发现变更时调用（也可用于设置保存后的即时生效）。
// 证书/密钥路径为空视为"本次没有有效配置" → 保留当前证书，绝不把面板打成无证书状态。
func (p *panelTLS) apply(certPath, keyPath, caPath string, mtls bool) {
	if certPath == "" || keyPath == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if certPath != p.certPath || keyPath != p.keyPath {
		log.Printf("[tls] certificate paths changed: cert=%s key=%s", certPath, keyPath)
		p.certPath, p.keyPath = certPath, keyPath
		p.cert = nil // 强制重读，即使新旧文件时间戳碰巧一致
	}
	if caPath != p.caPath {
		log.Printf("[tls] mTLS client CA path changed: %s", caPath)
		p.caPath = caPath
		p.pool = nil
	}
	if mtls != p.mtls {
		if mtls {
			log.Printf("[tls] mTLS enabled (hot reload)")
		} else {
			log.Printf("[tls] mTLS disabled (hot reload)")
		}
		p.mtls = mtls
	}
	if err := p.ensureCertLocked(); err != nil {
		log.Printf("[tls] hot reload certificate failed: %v", err)
	}
	if p.mtls {
		p.ensureCALocked()
	}
}

// loadCertLocked 读取证书+私钥并记录时间戳。调用方必须持锁。
func (p *panelTLS) loadCertLocked() error {
	cert, err := tls.LoadX509KeyPair(p.certPath, p.keyPath)
	if err != nil {
		return fmt.Errorf("load certificate %s: %w", p.certPath, err)
	}
	cs, _ := statStamp(p.certPath)
	ks, _ := statStamp(p.keyPath)
	p.cert = &cert
	p.certStamp, p.keyStamp = cs, ks
	log.Printf("[tls] loaded server certificate: %s", p.certPath)
	return nil
}

// ensureCertLocked 文件变化时重读；重读失败**保留旧证书**（不能因为一次读取失败让面板不可用）。
func (p *panelTLS) ensureCertLocked() error {
	cs, okC := statStamp(p.certPath)
	ks, okK := statStamp(p.keyPath)
	if p.cert != nil && okC && okK && cs == p.certStamp && ks == p.keyStamp {
		return nil
	}
	if err := p.loadCertLocked(); err != nil {
		if p.cert != nil {
			p.logOnce("WARNING: reload server certificate failed, keeping previous: " + err.Error())
			return nil
		}
		return err
	}
	return nil
}

// ensureCALocked 文件变化时重读客户端 CA；失败时保留旧 pool（mTLS 继续用旧 CA）。
func (p *panelTLS) ensureCALocked() {
	if p.caPath == "" {
		return
	}
	st, ok := statStamp(p.caPath)
	if !ok {
		p.logOnce("WARNING: mTLS client CA not readable: " + p.caPath)
		return
	}
	if p.pool != nil && st == p.caStamp {
		return
	}
	raw, err := os.ReadFile(p.caPath)
	if err != nil {
		p.logOnce("WARNING: read mTLS client CA failed: " + err.Error())
		return
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(raw) {
		p.logOnce("WARNING: parse mTLS client CA failed: " + p.caPath)
		return
	}
	p.pool = pool
	p.caStamp = st
	log.Printf("[tls] loaded mTLS client CA: %s", p.caPath)
}

// logOnce 相同消息只打印一次（巡检/握手都会反复触发）。
func (p *panelTLS) logOnce(msg string) {
	if msg == p.lastMsg {
		return
	}
	p.lastMsg = msg
	log.Print(msg)
}

// watchSettings 每 10s 从库里读一次面板 TLS 相关设置：
// 证书续期靠文件时间戳即时感知 ✓，这里只负责"换路径 / 开关 mTLS"这类配置变更 ✓。
func (p *panelTLS) watchSettings(repo *repository.SystemSettingRepository, dataDir string) {
	if repo == nil {
		return
	}
	get := func(key string) string {
		v, err := repo.GetByKey(context.Background(), key)
		if err != nil {
			return ""
		}
		return v
	}
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		// TLS 开关本身无法热切换（监听器已按 TLS 建好），关闭需重启；
		// 这里只在仍启用 TLS 时同步证书/CA 路径与 mTLS 开关。
		if get("panel.tls_enabled") != "true" {
			continue
		}
		p.apply(
			resolveDataPath(dataDir, get("panel.tls_cert_path")),
			resolveDataPath(dataDir, get("panel.tls_key_path")),
			resolveDataPath(dataDir, get("panel.mtls_ca_path")),
			get("panel.mtls_enabled") == "true",
		)
	}
}
