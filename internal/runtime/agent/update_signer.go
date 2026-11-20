package agent

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	secretcrypto "corefusion/internal/security/crypto"
)

// AgentUpdateSigner 用 ed25519 给 Agent 更新包签名（安全审计 v7 L-6）。
//
// 为什么需要：更新包的 `.sha256` 与二进制来自同一个主机，控制下载域名/DNS 的人
// 可以把两者一起换掉。签名必须由**mTLS 控制面**（PanelCommand 流）下发，并且
// Agent 端内置/预置公钥，才能形成独立的信任锚点。
//
// 私钥**存数据库**（system_settings `agent.update_signing_key`，AES-GCM 加密），
// 与 CA 私钥（ca_roots.key_pem_enc）同一口径：备份只含数据库，密钥才不会随
// 数据目录丢失；启动时再物化到 `agent/update-signing.key`（0600）供运维查看。
// 公钥由安装脚本写入节点 `config.yaml` 的 `update.public_key`。
const (
	updateSigningKeyFile = "update-signing.key"
	// UpdateSigningKeySettingKey 是数据库里保存加密私钥的键。
	UpdateSigningKeySettingKey = "agent.update_signing_key"
	// UpdatePublicKeyConfigKey 是 Agent 配置里的公钥路径名（仅用于报错提示）。
	UpdatePublicKeyConfigKey = "update.public_key"
)

// UpdateSigningKeyStore 是保存签名私钥所需的设置存储（*repository.SystemSettingRepository）。
type UpdateSigningKeyStore interface {
	GetByKey(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

// AgentUpdateSigner 持有面板的更新签名私钥。
type AgentUpdateSigner struct {
	mu   sync.RWMutex
	priv ed25519.PrivateKey
}

// NewAgentUpdateSigner 用给定私钥创建签名器。
func NewAgentUpdateSigner(priv ed25519.PrivateKey) *AgentUpdateSigner {
	return &AgentUpdateSigner{priv: priv}
}

// decodeSignerSeed 解析 base64 seed。
func decodeSignerSeed(raw string) ([]byte, error) {
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("invalid update signing key: expected base64 %d-byte seed", ed25519.SeedSize)
	}
	return seed, nil
}

// LoadOrCreateAgentUpdateSigner 读取面板更新签名私钥：优先数据库（加密），
// 其次兼容旧版本的文件，最后才生成新密钥。store 为 nil 时只走文件（测试/独立使用）。
func LoadOrCreateAgentUpdateSigner(ctx context.Context, dataDir string, store UpdateSigningKeyStore) (*AgentUpdateSigner, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	path := filepath.Join(dataDir, "agent", updateSigningKeyFile)

	// ① 数据库（权威来源，随数据库备份迁移）
	if store != nil {
		if enc, err := store.GetByKey(ctx, UpdateSigningKeySettingKey); err == nil && strings.TrimSpace(enc) != "" {
			plain, derr := secretcrypto.DecryptSecret(enc)
			if derr != nil {
				return nil, fmt.Errorf("decrypt %s: %w", UpdateSigningKeySettingKey, derr)
			}
			seed, serr := decodeSignerSeed(plain)
			if serr != nil {
				return nil, fmt.Errorf("%s: %w", UpdateSigningKeySettingKey, serr)
			}
			return NewAgentUpdateSigner(ed25519.NewKeyFromSeed(seed)), nil
		}
	}

	// ② 旧版本文件（升级迁移）：读到就写回数据库
	if b, err := os.ReadFile(path); err == nil {
		seed, serr := decodeSignerSeed(string(b))
		if serr != nil {
			return nil, fmt.Errorf("%s: %w", path, serr)
		}
		if store != nil {
			if perr := persistSignerSeed(ctx, store, path, seed); perr != nil {
				// 文件仍在，密钥不会变；只影响「备份是否包含它」，记日志继续。
				log.Printf("[AgentUpdateSigner] migrate file key to database failed: %v", perr)
			} else {
				log.Printf("[AgentUpdateSigner] migrated update signing key from %s to database", path)
			}
		}
		return NewAgentUpdateSigner(ed25519.NewKeyFromSeed(seed)), nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	// ③ 首次：生成并持久化
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return nil, fmt.Errorf("generate update signing key: %w", err)
	}
	if store != nil {
		if err := persistSignerSeed(ctx, store, path, seed); err != nil {
			return nil, err
		}
	} else if err := writeSignerSeedFile(path, seed); err != nil {
		return nil, err
	}
	log.Printf("[AgentUpdateSigner] generated update signing key (db=%v, file=%s)", store != nil, path)
	return NewAgentUpdateSigner(ed25519.NewKeyFromSeed(seed)), nil
}

// persistSignerSeed 把 seed 加密写入数据库，并物化到文件（权限 0600）。
func persistSignerSeed(ctx context.Context, store UpdateSigningKeyStore, path string, seed []byte) error {
	enc, err := secretcrypto.EncryptSecret(base64.StdEncoding.EncodeToString(seed))
	if err != nil {
		return fmt.Errorf("encrypt update signing key: %w", err)
	}
	if err := store.Set(ctx, UpdateSigningKeySettingKey, enc); err != nil {
		return fmt.Errorf("save update signing key: %w", err)
	}
	if err := writeSignerSeedFile(path, seed); err != nil {
		// 数据库已保存，文件只是方便运维；失败不影响功能。
		log.Printf("[AgentUpdateSigner] write key file %s failed: %v", path, err)
	}
	return nil
}

func writeSignerSeedFile(path string, seed []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(seed)+"\n"), 0600)
}

// SignChecksum 对规范化后的 SHA-256 校验和签名，返回 base64。
// 没有私钥时返回空串（调用方应视为「无法签名」）。
func (s *AgentUpdateSigner) SignChecksum(checksum string) string {
	if s == nil {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.priv) == 0 {
		return ""
	}
	sum := normalizeChecksum(checksum)
	if sum == "" {
		return ""
	}
	return base64.StdEncoding.EncodeToString(ed25519.Sign(s.priv, []byte(sum)))
}

// PublicKeyPEM 返回 PKIX 编码的公钥 PEM（可安全公开，随安装脚本下发）。
func (s *AgentUpdateSigner) PublicKeyPEM() []byte {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.priv) == 0 {
		return nil
	}
	pub := s.priv.Public().(ed25519.PublicKey)
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
}

// ParseUpdatePublicKey 解析 PEM 公钥（只接受 ed25519）。
func ParseUpdatePublicKey(pemBytes []byte) (ed25519.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("invalid update public key PEM")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse update public key: %w", err)
	}
	pub, ok := key.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("update public key must be ed25519")
	}
	return pub, nil
}

// VerifyAgentUpdateSignature 校验更新包签名：signature 是 checksum 的 ed25519 签名（base64）。
func VerifyAgentUpdateSignature(pubPEM []byte, checksum, signature string) error {
	pub, err := ParseUpdatePublicKey(pubPEM)
	if err != nil {
		return err
	}
	sum := normalizeChecksum(checksum)
	if sum == "" {
		return fmt.Errorf("empty checksum")
	}
	if strings.TrimSpace(signature) == "" {
		return fmt.Errorf("update is not signed")
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(signature))
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("invalid signature length %d", len(sig))
	}
	if !ed25519.Verify(pub, []byte(sum), sig) {
		return fmt.Errorf("update signature mismatch")
	}
	return nil
}
