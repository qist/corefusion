package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// ACMEAccountConfigRepository handles ACME account config database operations.
//
// ── EAB 凭据加密 ✓ ──
// `EABMACKey`（账号绑定的 EAB-HMAC-Key ✓）是**密钥** ✗：谁能读到库文件 / 备份，
// 谁就能拿它去 CA 侧证明"我就是这个账号" ✓（ZeroSSL、Google Public CA 都靠它 ✓）。
// 实体上那句 `json:"-"` 只挡住了**前端** ✗，库内仍是明文 ✗ —— 这里补上 ✓。
//
// 规则与 `EABConfigRepository` 完全一致 ✓：
//   - AES-256-GCM ✓，**同一把密钥** ✓（`internal/security/crypto` ✓，`CF_SECRET_ENCRYPTION_KEY` ✓）；
//   - 加解密**只在这一层** ✓ → 上层（service / handler / router 下发）永远拿明文 ✓；
//   - 空值保持空 ✓，已加密的不再包一层 ✓（`Update` 时"留空 = 保持原值"会读到明文再存回 ✓）；
//   - 历史明文行原样可用 ✓（`DecryptSecret` 对无前缀输入原样返回 ✓），启动迁移会就地转密文 ✓。
type ACMEAccountConfigRepository struct {
	db *gorm.DB
}

// NewACMEAccountConfigRepository creates a new ACMEAccountConfigRepository.
func NewACMEAccountConfigRepository(db *gorm.DB) *ACMEAccountConfigRepository {
	return &ACMEAccountConfigRepository{db: db}
}

func (r *ACMEAccountConfigRepository) Create(ctx context.Context, cfg *entity.ACMEAccountConfig) error {
	stored, err := encryptACMESecrets(cfg)
	if err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Create(stored).Error; err != nil {
		return err
	}
	// ⚠ 自增主键回填给调用方 ✗：写的是密文副本 ✓，GORM 只会把 ID 填在副本上 ✓，
	// 而 service 要把 cfg 直接返回给前端 ✓ —— 漏了就会出现"新建成功但 id=0" ✗。
	cfg.ID = stored.ID
	cfg.CreatedAt = stored.CreatedAt
	cfg.UpdatedAt = stored.UpdatedAt
	return nil
}

func (r *ACMEAccountConfigRepository) GetByID(ctx context.Context, id int64) (*entity.ACMEAccountConfig, error) {
	var cfg entity.ACMEAccountConfig
	err := r.db.WithContext(ctx).First(&cfg, id).Error
	if err != nil {
		return nil, err
	}
	decryptACMESecrets(&cfg)
	return &cfg, nil
}

func (r *ACMEAccountConfigRepository) GetDefault(ctx context.Context) (*entity.ACMEAccountConfig, error) {
	var cfg entity.ACMEAccountConfig
	err := r.db.WithContext(ctx).Where("is_default = ? AND status = ?", true, "active").First(&cfg).Error
	if err != nil {
		return nil, err
	}
	decryptACMESecrets(&cfg)
	return &cfg, nil
}

func (r *ACMEAccountConfigRepository) List(ctx context.Context) ([]entity.ACMEAccountConfig, error) {
	var list []entity.ACMEAccountConfig
	err := r.db.WithContext(ctx).Order("id ASC").Find(&list).Error
	if err != nil {
		return list, err
	}
	// ⚠ 必须在 repository 里解密 ✗：`ACMEEmailService.List` 靠 `EABMACKey != ""` 判断 has_eab ✓，
	// 若这里返回密文，前端会看到"配过 EAB"的账号全是 true，而没法区分到底填没填 ✗。
	for i := range list {
		decryptACMESecrets(&list[i])
	}
	return list, nil
}

func (r *ACMEAccountConfigRepository) Update(ctx context.Context, cfg *entity.ACMEAccountConfig) error {
	stored, err := encryptACMESecrets(cfg)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Save(stored).Error
}

func (r *ACMEAccountConfigRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.ACMEAccountConfig{}, id).Error
}

// encryptACMESecrets 返回**密文版副本** ✓ —— 不改写调用方实体 ✗
// （service 会把刚 Create/Update 的 cfg 直接回给前端，改成密文就穿帮了 ✗）。
func encryptACMESecrets(cfg *entity.ACMEAccountConfig) (*entity.ACMEAccountConfig, error) {
	out := *cfg
	mac, err := encryptCredential(out.EABMACKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt acme eab mac key: %w", err)
	}
	out.EABMACKey = mac
	return &out, nil
}

// decryptACMESecrets 就地把密文换回明文 ✓（解不开时原样返回并打日志 ✓，不让整个账号列表接口挂掉 ✗）。
func decryptACMESecrets(cfg *entity.ACMEAccountConfig) {
	cfg.EABMACKey = decryptCredential("eabmac_key", cfg.ID, cfg.EABMACKey)
}
