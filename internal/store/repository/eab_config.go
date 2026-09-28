package repository

import (
	"context"
	"fmt"
	"log"
	"strings"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	secretcrypto "corefusion/internal/security/crypto"
)

// EABConfigRepository handles EAB config database operations.
//
// ── 凭据加密 ✓ ──
// `MACKey`（EAB-HMAC-Key ✓）与 `GoogleSAJSON`（GCP 服务账号密钥 ✓）都是**凭据** ✗：
// 明文落库后，任何能读到库文件 / 备份的人都能拿它们去注册 ACME 账号 ✓，
// 或以该服务账号的身份调 GCP 接口 ✗（现网真机上这两项原先就是明文 ✗）。
//
// 加解密**只在这一层**做 ✓（AES-256-GCM ✓，见 `internal/security/crypto` ✓，
// 与**充值卡 / DNS 渠道凭据共用同一把密钥** ✓ —— `CF_SECRET_ENCRYPTION_KEY` ✓）→
//   - 上层（service / router / 节点下发）拿到的**永远是明文** ✓ → 调用点一行都不用改 ✓；
//   - 存进库的**永远是密文** ✓ → 不会再有人"顺手"写出明文 ✗。
//
// 兼容 ✓：历史行是明文（不带 `enc:` 前缀 ✓）→ `DecryptSecret` 原样返回 ✓
// → 老数据不迁移也能继续用 ✓（启动时的幂等迁移会就地转成密文 ✓）。
type EABConfigRepository struct {
	db *gorm.DB
}

// NewEABConfigRepository creates a new EABConfigRepository.
func NewEABConfigRepository(db *gorm.DB) *EABConfigRepository {
	return &EABConfigRepository{db: db}
}

func (r *EABConfigRepository) Create(ctx context.Context, cfg *entity.EABConfig) error {
	stored, err := encryptEABSecrets(cfg)
	if err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Create(stored).Error; err != nil {
		return err
	}
	// ⚠ 自增主键必须**回填给调用方** ✗：Create 写的是密文副本，GORM 只把 ID 填在副本上 ✓，
	// 而 service 要把 cfg.ID 返回给前端 ✓、`FetchGoogleEAB` 之后还要按 ID 更新它 ✓ ——
	// 漏了这一步就会出现"新建成功但 id=0" → 前端拿不到 ID ✗（既有测试 `TestEABConfigCRUD` 正是这么抓到它的 ✓）。
	cfg.ID = stored.ID
	cfg.CreatedAt = stored.CreatedAt
	cfg.UpdatedAt = stored.UpdatedAt
	return nil
}

func (r *EABConfigRepository) GetByID(ctx context.Context, id int64) (*entity.EABConfig, error) {
	var cfg entity.EABConfig
	if err := r.db.WithContext(ctx).First(&cfg, id).Error; err != nil {
		return nil, err
	}
	decryptEABSecrets(&cfg)
	return &cfg, nil
}

func (r *EABConfigRepository) List(ctx context.Context) ([]entity.EABConfig, error) {
	var list []entity.EABConfig
	if err := r.db.WithContext(ctx).Order("id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	for i := range list {
		decryptEABSecrets(&list[i])
	}
	return list, nil
}

func (r *EABConfigRepository) Update(ctx context.Context, cfg *entity.EABConfig) error {
	stored, err := encryptEABSecrets(cfg)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Save(stored).Error
}

func (r *EABConfigRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.EABConfig{}, id).Error
}

// encryptEABSecrets 返回**密文版副本** ✓ —— 绝不改写调用方实体 ✗。
//
// 为什么必须是副本 ✗：`MintGoogleEAB` 保存完会**立刻**把同一份 cfg 交给 ACME 注册
// （`router.go` 里直接取 `cfg.KeyID, cfg.MACKey` 下发 ✓）——
// 若就地改写入参，调用方拿到的就是密文 ✗ → 注册端只会说"EAB 无效" ✗，很难联想到加密 ✗。
func encryptEABSecrets(cfg *entity.EABConfig) (*entity.EABConfig, error) {
	out := *cfg // 全是值类型字段 ✓ 浅拷贝足够 ✓

	mac, err := encryptCredential(out.MACKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt eab mac key: %w", err)
	}
	sa, err := encryptCredential(out.GoogleSAJSON)
	if err != nil {
		return nil, fmt.Errorf("encrypt google service account json: %w", err)
	}
	out.MACKey = mac
	out.GoogleSAJSON = sa
	return &out, nil
}

// encryptCredential 加密单个凭据 ✓：空值保持空 ✓、**已加密的不再包一层** ✓。
func encryptCredential(s string) (string, error) {
	if strings.TrimSpace(s) == "" {
		return s, nil
	}
	if secretcrypto.IsEncrypted(s) {
		return s, nil
	}
	return secretcrypto.EncryptSecret(s)
}

// decryptEABSecrets 就地把密文换回明文 ✓（读路径 ✓）。
//
// 解不开时**原样返回**并打日志 ✓：宁可让调用方拿到一个"明显不对劲的值"
// （注册时报 EAB 无效 ✓，日志里能查到原因 ✓），也不要把证书页整个打成 500 ✗。
func decryptEABSecrets(cfg *entity.EABConfig) {
	cfg.MACKey = decryptCredential("mac_key", cfg.ID, cfg.MACKey)
	cfg.GoogleSAJSON = decryptCredential("google_sa_json", cfg.ID, cfg.GoogleSAJSON)
}

func decryptCredential(field string, id int64, s string) string {
	if s == "" {
		return s
	}
	plain, err := secretcrypto.DecryptSecret(s)
	if err != nil {
		log.Printf("[eab_configs] WARNING: decrypt %s for id=%d failed, returning raw value: %v", field, id, err)
		return s
	}
	return plain
}
