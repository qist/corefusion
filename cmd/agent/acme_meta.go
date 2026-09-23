package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// certMetaFile 是"这张证书当初是怎么签出来的"落盘文件名 ✓（0600 ✓ 含 DNS 凭据 ✓）。
const certMetaFile = "meta.json"

// certMeta 记录**自续期所需的全部参数** ✓。
//
// 为什么要落盘 ✗：以前这些参数**只由面板在 `IssuePublicCertCommand` 里下发** ✓，
// 节点签完就把它们忘了 ✗ → 一旦面板不触发续期 ✗，节点**根本不知道该怎么续** ✗
// （邮箱、CA、挑战方式、DNS 凭据全丢了 ✗）。
//
// 按新的架构口径（**面板只管触发 ✓、证书由节点持有并由节点自己续 ✓、面板不落地 ✗**）：
// 节点必须自己记住这些 ✓ —— 所以签发成功后立刻写一份 ✓，续期时读回来 ✓，行为与首次签发完全一致 ✓。
type certMeta struct {
	// CertID 是面板侧的证书 id ✓（上报/对账用 ✓；节点自签时可能为 0 ✓）。
	CertID      int64    `json:"cert_id,omitempty"`
	Identifiers []string `json:"identifiers"` // 首次申请时的 SAN（域名或 IP ✓）
	Email       string   `json:"email"`
	CA          string   `json:"ca"`
	Challenge   string   `json:"challenge"`
	Profile     string   `json:"profile,omitempty"`
	Webroot     string   `json:"webroot,omitempty"`
	// DNS 凭据只在 dns-01 时有 ✓（wildcard 必须 dns-01 ✓）——
	// 节点要能自己续 ✓ 就必须留着 ✓（文件权限 0600 ✓、绝不写日志 ✓）。
	DNSProvider string            `json:"dns_provider,omitempty"`
	DNSEnvVars  map[string]string `json:"dns_env_vars,omitempty"`

	IssuedAt    time.Time `json:"issued_at,omitempty"`
	LastRenewAt time.Time `json:"last_renew_at,omitempty"`
}

// certMetaPath 返回某张证书的 meta 路径 ✓。
func certMetaPath(dataDir, name string) string {
	return filepath.Join(dataDir, "certs", name, certMetaFile)
}

// writeCertMeta 原子写入 meta ✓（先写临时文件再 rename ✓，避免半截文件 ✗）。
func writeCertMeta(dataDir, name string, m *certMeta) error {
	dir := filepath.Dir(certMetaPath(dataDir, name))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := certMetaPath(dataDir, name) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, certMetaPath(dataDir, name))
}

// readCertMeta 读回 meta ✓；**不存在 = 这张证书不归节点自管** ✓（例如面板推下来的 ✓）
// → 调用方应当跳过 ✓（返回 os.ErrNotExist ✓ 便于区分"读坏了"和"没有" ✓）。
func readCertMeta(dataDir, name string) (*certMeta, error) {
	data, err := os.ReadFile(certMetaPath(dataDir, name))
	if err != nil {
		return nil, err
	}
	var m certMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", certMetaPath(dataDir, name), err)
	}
	return &m, nil
}

// listLocalCertDirs 列出本机 ACME 目录下已签发的证书目录名 ✓（`<dataDir>/certs/<name>/` ✓）。
func listLocalCertDirs(dataDir string) ([]string, error) {
	root := filepath.Join(dataDir, "certs")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // 还没签过任何证书 ✓ 不是错误 ✓
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, serr := os.Stat(filepath.Join(root, e.Name(), "cert.pem")); serr != nil {
			continue // 没有 cert.pem 的目录跳过 ✓
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out, nil
}

// issueParams 依据 meta 还原"签发参数" ✓ —— 自续期就是**用与首次签发完全相同的一套参数**再签一次 ✓
// （这样 CA/邮箱/挑战方式/EAB 行为都不会漂移 ✗，这是之前踩过的坑 ✓）。
func (m *certMeta) issueParams(a *AgentApp, dataDir string) acmeIssueParams {
	ids := make([]string, 0, len(m.Identifiers))
	for _, s := range m.Identifiers {
		if s = strings.TrimSpace(s); s != "" {
			ids = append(ids, s)
		}
	}
	env := map[string]string{}
	for k, v := range m.DNSEnvVars {
		env[k] = v
	}
	return acmeIssueParams{
		Identifiers: ids,
		Email:       strings.TrimSpace(m.Email),
		Challenge:   strings.TrimSpace(m.Challenge),
		CA:          strings.TrimSpace(m.CA),
		Profile:     strings.TrimSpace(m.Profile),
		Webroot:     strings.TrimSpace(m.Webroot),
		DataDir:     dataDir,
		DNSProvider: strings.TrimSpace(m.DNSProvider),
		DNSEnvVars:  env,
		CertID:      m.CertID,
		// ⚠ 刻意**不带 EAB** ✗：账号早已在节点本地注册好 ✓（`<dataDir>/accounts/<ca>.json` ✓），
		// 续期只用账号密钥 ✓ → 再也不会消耗新的 EAB ✓（Google 的 EAB 是有限/易变的 ✓）。
		Timeout: 5 * time.Minute,
	}
}
