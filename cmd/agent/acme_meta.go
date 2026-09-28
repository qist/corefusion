package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"corefusion/internal/runtime/cert"
	"corefusion/internal/security/crypto"
)

// certMetaFile 是"这张证书当初是怎么签出来的"落盘文件名 ✓（0600 ✓ DNS 凭据 AES-256-GCM 加密 ✓）。
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
	// 节点要能自己续 ✓ 就必须留着 ✓。落盘时**值经 AES-256-GCM 加密** ✓（`enc:` 前缀 ✓，
	// 与面板侧 DNS 通道/EAB/ACME 账号同口径 ✓），文件权限 0600 ✓、绝不写日志 ✓。
	// ⚠ 加密密钥在节点本地派生（CF_SECRET_ENCRYPTION_KEY 或主机名兜底 ✓），
	// 所以**续期完全不依赖面板在线** ✓（面板挂了节点照样自续 ✓）；
	// 防的是备份/磁盘镜像/误 cat 这类"整目录泄露" ✓，防不了已获 root 的攻击者 ✓。
	// ⚠ 换主机名/换机器会解不开 → 证书退化为"无法自续" ✓（续期循环会记日志 ✓，现有证书不受影响 ✓）。
	DNSProvider string            `json:"dns_provider,omitempty"`
	DNSEnvVars  map[string]string `json:"dns_env_vars,omitempty"`

	// DNSChannelID 是**面板侧那条 DNS 通道的 id** ✓（`certificates.dns_channel_id` ✓）。
	// ⚠ 节点签发本身用不到它 ✗（凭据已在 DNSProvider/DNSEnvVars ✓），
	// 但**上报时必须原样带回面板** ✗：不回传 → 面板落库的证书 `dns_channel_id = 0` ✗
	// → 面板自动续期按 dns-01 找不到通道 → 只能**跳过续期**（不降级，但也永远不续 ✗）。
	// 老 meta 没有这个字段 → 反序列化为 0 ✓ = 历史数据 ✓（面板按通配符兜底 ✓）。
	DNSChannelID int64 `json:"dns_channel_id,omitempty"`

	// KeyType / KeyBits 是**这张证书的密钥规格** ✓（ecc|rsa + 位数 ✓）。
	// ⚠ 节点自续期=照这套参数重签一遍 ✗ → 不记住就会从 rsa 退回 EC256 ✗
	// （续期顺手把密钥类型换掉 ✗，真机就是"记录 rsa、文件却是 EC"的来源之一 ✓）。
	// 老 meta 没有这两个字段 → 反序列化为空 ✓ = 历史默认 EC256 ✓，兼容 ✓。
	KeyType string `json:"key_type,omitempty"`
	KeyBits int    `json:"key_bits,omitempty"`

	IssuedAt    time.Time `json:"issued_at,omitempty"`
	LastRenewAt time.Time `json:"last_renew_at,omitempty"`
}

// certMetaPath 返回某张证书的 meta 路径 ✓。
//
// ⚠ meta **永远**在 `certs/<name>/meta.json` ✓（不进类型子目录 ✓）——
// 它是"这张证书归节点自管"的标记 ✓，与密钥类型无关 ✓；
// 证书文件本身才按类型分层 ✓（见 cert.LocalCertRelPath ✓）。
func certMetaPath(dataDir, name string) string {
	return filepath.Join(dataDir, "certs", name, certMetaFile)
}

// certNameFromRelPath 从 `CertRelPaths` 给出的相对路径里取出**证书名** ✓。
//
// 布局（写死在 `cert.CertRelPaths` ✓）：
//
//	ecc  → certs/<name>/cert.pem
//	其他 → certs/<name>/<keytype>/cert.pem      ← 例如 rsa
//
// ⚠ 不能直接用 `filepath.Base(filepath.Dir(rel))` ✗：RSA 时那会得到 "rsa" ✗
// → meta 落到 `certs/rsa/meta.json` ✗（与 certMetaPath 不一致 ✗ → 自续期失效 ✗）。
func certNameFromRelPath(rel string) string {
	parts := strings.Split(filepath.ToSlash(filepath.Dir(rel)), "/")
	if len(parts) >= 2 && parts[0] == "certs" {
		return parts[1]
	}
	return filepath.Base(filepath.Dir(rel))
}

// localCertPathFor 返回本机某张证书**按密钥类型**定位的 cert.pem 绝对路径 ✓。
func localCertPathFor(dataDir, name, keyType string) string {
	certRel, _, _ := cert.CertRelPaths(name, keyType)
	return filepath.Join(dataDir, certRel)
}

// encryptDNSecrets 返回一份**加密后**的 dns_env_vars 副本 ✓。
//
// ⚠ 幂等 ✓：已经是 `enc:` 前缀的值**不再包一层** ✓（双层密文解一次还是密文 ✗，
// 与 crypto.IsEncrypted 的设计口径一致 ✓）——老明文 meta 就地自愈重写时靠这一点 ✓。
// 不改传入的 map ✓（调用方后续还要用明文值 ✓）。
func encryptDNSecrets(env map[string]string) (map[string]string, error) {
	if len(env) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(env))
	for k, v := range env {
		if v == "" {
			out[k] = v
			continue
		}
		if crypto.IsEncrypted(v) {
			out[k] = v
			continue
		}
		enc, err := crypto.EncryptSecret(v)
		if err != nil {
			return nil, fmt.Errorf("encrypt dns credential %s: %w", k, err)
		}
		out[k] = enc
	}
	return out, nil
}

// writeCertMeta 原子写入 meta ✓（先写临时文件再 rename ✓，避免半截文件 ✗）。
// DNS 凭据值**加密后**才落盘 ✓（密钥节点本地派生 ✓，续期不依赖面板 ✓）。
func writeCertMeta(dataDir, name string, m *certMeta) error {
	dir := filepath.Dir(certMetaPath(dataDir, name))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	stored := *m
	encEnv, err := encryptDNSecrets(m.DNSEnvVars)
	if err != nil {
		return err
	}
	stored.DNSEnvVars = encEnv
	data, err := json.MarshalIndent(&stored, "", "  ")
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
//
// DNS 凭据值解密后返回 ✓（`enc:` 前缀才解 ✓，老明文 meta 原样透传 ✓ = 向后兼容 ✓）。
// ⚠ 自愈：读到**明文凭据**（升级前落盘的老 meta ✓）会**就地重写成密文** ✓
// （下一次 6h 自续检查扫到即洗掉 ✓，无需人工迁移 ✓）。
func readCertMeta(dataDir, name string) (*certMeta, error) {
	data, err := os.ReadFile(certMetaPath(dataDir, name))
	if err != nil {
		return nil, err
	}
	var m certMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", certMetaPath(dataDir, name), err)
	}
	if len(m.DNSEnvVars) > 0 {
		needsHeal := false
		for _, v := range m.DNSEnvVars {
			if v != "" && !crypto.IsEncrypted(v) {
				needsHeal = true
				break
			}
		}
		if needsHeal {
			if werr := writeCertMeta(dataDir, name, &m); werr != nil {
				// 自愈失败不影响本次读取 ✓（凭据已解密可用 ✓；下一轮再试 ✓）
				log.Printf("[acme] 就地加密 %s 的 DNS 凭据失败（下轮重试）: %v", certMetaPath(dataDir, name), werr)
			} else {
				log.Printf("[acme] 已就地加密 %s 的 DNS 凭据（老明文 meta 自愈 ✓）", certMetaPath(dataDir, name))
			}
		}
		for k, v := range m.DNSEnvVars {
			plain, derr := crypto.DecryptSecret(v)
			if derr != nil {
				// 解不开 = 密钥变了（换主机名/换机器 ✗）→ 这张证书无法自续 ✓，明说 ✓
				log.Printf("[acme] 解密 %s 的 DNS 凭据 %s 失败（密钥不匹配？该证书将无法自续）: %v",
					certMetaPath(dataDir, name), k, derr)
				return nil, fmt.Errorf("decrypt dns credential %s: %w", k, derr)
			}
			m.DNSEnvVars[k] = plain
		}
	}
	return &m, nil
}

// listLocalCertDirs 列出本机 ACME 目录下已签发的证书目录名 ✓（`<dataDir>/certs/<name>/` ✓）。
//
// ⚠ 必须同时认**类型子目录** ✗：RSA 证书在 `certs/<name>/rsa/cert.pem` ✓，
// 只找 `certs/<name>/cert.pem` 的话 RSA 证书**永远发现不了** ✗ → 永不自续 ✗。
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
			// 顶层没有 cert.pem：再看类型子目录（RSA ✓）；都没有才跳过 ✓
			if _, rerr := os.Stat(localCertPathFor(dataDir, e.Name(), cert.KeyTypeRSA)); rerr != nil {
				continue
			}
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
		// 面板通道 id：节点签发不用它 ✓，但上报时要原样带回 ✓（见 certMeta.DNSChannelID ✓）
		DNSChannelID: m.DNSChannelID,
		// ⚠ 钥匙类型也要原样带回去 ✗：不带就退回 EC256 ✗
		// → 每次自续期都**悄悄把 rsa 证书换成 EC** ✗（真机就是"记录 rsa、文件是 EC" ✓）。
		// 老 meta 里没有这两项 → 空/0 ✓ = 历史默认 EC256 ✓，行为与升级前一致 ✓。
		KeyType: strings.TrimSpace(m.KeyType),
		KeyBits: m.KeyBits,
		// ⚠ 刻意**不带 EAB** ✗：账号早已在节点本地注册好 ✓（`<dataDir>/accounts/<ca>.json` ✓），
		// 续期只用账号密钥 ✓ → 再也不会消耗新的 EAB ✓（Google 的 EAB 是有限/易变的 ✓）。
		Timeout: 5 * time.Minute,
	}
}
