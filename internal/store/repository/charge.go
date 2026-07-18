package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/security/crypto"
)

// ============================================================
// ChargeSiteRepository
// ============================================================

type ChargeSiteRepository struct {
	db *gorm.DB
}

func NewChargeSiteRepository(db *gorm.DB) *ChargeSiteRepository {
	return &ChargeSiteRepository{db: db}
}

func (r *ChargeSiteRepository) ListEnabled(ctx context.Context) ([]entity.ChargeSite, error) {
	var items []entity.ChargeSite
	err := r.db.WithContext(ctx).Where("enabled = ?", true).Order("sort_order ASC, id ASC").Find(&items).Error
	return items, err
}

func (r *ChargeSiteRepository) ListAll(ctx context.Context) ([]entity.ChargeSite, error) {
	var items []entity.ChargeSite
	err := r.db.WithContext(ctx).Order("sort_order ASC, id ASC").Find(&items).Error
	return items, err
}

func (r *ChargeSiteRepository) GetByID(ctx context.Context, id int64) (*entity.ChargeSite, error) {
	var item entity.ChargeSite
	err := r.db.WithContext(ctx).First(&item, id).Error
	return &item, err
}

func (r *ChargeSiteRepository) Create(ctx context.Context, item *entity.ChargeSite) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *ChargeSiteRepository) Update(ctx context.Context, item *entity.ChargeSite) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *ChargeSiteRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.ChargeSite{}, id).Error
}

// ============================================================
// ChargeOrderRepository
// ============================================================

type ChargeOrderRepository struct {
	db *gorm.DB
}

func NewChargeOrderRepository(db *gorm.DB) *ChargeOrderRepository {
	return &ChargeOrderRepository{db: db}
}

func (r *ChargeOrderRepository) Create(ctx context.Context, item *entity.ChargeOrder) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *ChargeOrderRepository) GetByID(ctx context.Context, id int64) (*entity.ChargeOrder, error) {
	var item entity.ChargeOrder
	err := r.db.WithContext(ctx).First(&item, id).Error
	return &item, err
}

func (r *ChargeOrderRepository) GetByTradeNo(ctx context.Context, tradeNo string) (*entity.ChargeOrder, error) {
	var item entity.ChargeOrder
	err := r.db.WithContext(ctx).Where("trade_no = ?", tradeNo).First(&item).Error
	return &item, err
}

func (r *ChargeOrderRepository) Update(ctx context.Context, item *entity.ChargeOrder) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *ChargeOrderRepository) ListByClientID(ctx context.Context, clientID int64, status string, limit, offset int) ([]entity.ChargeOrder, int64, error) {
	var items []entity.ChargeOrder
	var total int64
	q := r.db.WithContext(ctx).Model(&entity.ChargeOrder{}).Where("client_id = ?", clientID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	q.Count(&total)
	err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

// MarkExpired marks all pending orders past their expiry time as expired.
func (r *ChargeOrderRepository) MarkExpired(ctx context.Context) error {
	return r.db.WithContext(ctx).
		Model(&entity.ChargeOrder{}).
		Where("status = ? AND expires_at IS NOT NULL AND expires_at < ?", entity.ChargeStatusPending, time.Now()).
		Update("status", entity.ChargeStatusExpired).Error
}

// ============================================================
// ChargeCardRepository
// ============================================================

type ChargeCardRepository struct {
	db *gorm.DB
}

func NewChargeCardRepository(db *gorm.DB) *ChargeCardRepository {
	return &ChargeCardRepository{db: db}
}

func (r *ChargeCardRepository) GetByCardCode(ctx context.Context, cardCode string) (*entity.ChargeCard, error) {
	var item entity.ChargeCard
	// v3.3: 使用 hash 查找，不存储明文 card_code
	cardHash := crypto.HashSecret(cardCode)
	err := r.db.WithContext(ctx).Where("card_code_hash = ?", cardHash).First(&item).Error
	if err != nil {
		return &item, err
	}
	// 解密填充明文
	if item.CardCodeEncrypted != "" {
		if plain, err := crypto.DecryptSecret(item.CardCodeEncrypted); err == nil {
			item.CardCode = plain
		}
	}
	return &item, err
}

func (r *ChargeCardRepository) Create(ctx context.Context, item *entity.ChargeCard) error {
	// v3.3: 加密存储卡号
	if item.CardCode != "" {
		item.CardCodeHash = crypto.HashSecret(item.CardCode)
		enc, err := crypto.EncryptSecret(item.CardCode)
		if err != nil {
			return err
		}
		item.CardCodeEncrypted = enc
	}
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *ChargeCardRepository) CreateBatch(ctx context.Context, items []entity.ChargeCard) error {
	// v3.3: 加密存储卡号
	for i := range items {
		if items[i].CardCode != "" {
			items[i].CardCodeHash = crypto.HashSecret(items[i].CardCode)
			enc, err := crypto.EncryptSecret(items[i].CardCode)
			if err != nil {
				return err
			}
			items[i].CardCodeEncrypted = enc
		}
	}
	return r.db.WithContext(ctx).Create(&items).Error
}

func (r *ChargeCardRepository) Update(ctx context.Context, item *entity.ChargeCard) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *ChargeCardRepository) ListAll(ctx context.Context, status, cardType string, limit, offset int) ([]entity.ChargeCard, int64, error) {
	var items []entity.ChargeCard
	var total int64
	q := r.db.WithContext(ctx).Model(&entity.ChargeCard{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if cardType != "" {
		q = q.Where("card_type = ?", cardType)
	}
	q.Count(&total)
	err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	if err != nil {
		return items, total, err
	}
	// v3.3: 解密填充明文 + 脱敏
	for i := range items {
		if items[i].CardCodeEncrypted != "" {
			if plain, err := crypto.DecryptSecret(items[i].CardCodeEncrypted); err == nil {
				items[i].CardCode = plain
				items[i].CardCodeMasked = crypto.MaskCardCode(plain)
			}
		}
	}
	return items, total, err
}

func (r *ChargeCardRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.ChargeCard{}, id).Error
}

// IncrementUsedCount atomically increments the used_count.
func (r *ChargeCardRepository) IncrementUsedCount(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Model(&entity.ChargeCard{}).
		Where("id = ?", id).
		UpdateColumn("used_count", gorm.Expr("used_count + 1")).Error
}

// ============================================================
// ClientBalanceRepository
// ============================================================

type ClientBalanceRepository struct {
	db *gorm.DB
}

func NewClientBalanceRepository(db *gorm.DB) *ClientBalanceRepository {
	return &ClientBalanceRepository{db: db}
}

func (r *ClientBalanceRepository) GetByClientID(ctx context.Context, clientID int64) (*entity.ClientBalance, error) {
	var item entity.ClientBalance
	err := r.db.WithContext(ctx).Where("client_id = ?", clientID).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// GetOrCreate returns the client's balance, creating a zero-balance record if none exists.
func (r *ClientBalanceRepository) GetOrCreate(ctx context.Context, clientID int64) (*entity.ClientBalance, error) {
	bal, err := r.GetByClientID(ctx, clientID)
	if err == nil {
		return bal, nil
	}
	// Create zero-balance record
	bal = &entity.ClientBalance{
		ClientID: clientID,
		Balance:  0,
		Frozen:   0,
	}
	if err := r.db.WithContext(ctx).Create(bal).Error; err != nil {
		return nil, err
	}
	return bal, nil
}

// AddBalance atomically adds amount to the client's balance.
// Returns the updated balance record.
func (r *ClientBalanceRepository) AddBalance(ctx context.Context, clientID int64, amount int64) (*entity.ClientBalance, error) {
	err := r.db.WithContext(ctx).Model(&entity.ClientBalance{}).
		Where("client_id = ?", clientID).
		UpdateColumn("balance", gorm.Expr("balance + ?", amount)).Error
	if err != nil {
		return nil, err
	}
	return r.GetByClientID(ctx, clientID)
}

// DeductBalance atomically deducts amount from the client's balance.
// Returns error if insufficient balance.
func (r *ClientBalanceRepository) DeductBalance(ctx context.Context, clientID int64, amount int64) (*entity.ClientBalance, error) {
	result := r.db.WithContext(ctx).Model(&entity.ClientBalance{}).
		Where("client_id = ? AND balance >= ?", clientID, amount).
		UpdateColumn("balance", gorm.Expr("balance - ?", amount))
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound // insufficient balance
	}
	return r.GetByClientID(ctx, clientID)
}

func (r *ClientBalanceRepository) Update(ctx context.Context, item *entity.ClientBalance) error {
	return r.db.WithContext(ctx).Save(item).Error
}

// ============================================================
// BalanceTransactionRepository
// ============================================================

type BalanceTransactionRepository struct {
	db *gorm.DB
}

func NewBalanceTransactionRepository(db *gorm.DB) *BalanceTransactionRepository {
	return &BalanceTransactionRepository{db: db}
}

func (r *BalanceTransactionRepository) Create(ctx context.Context, item *entity.BalanceTransaction) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *BalanceTransactionRepository) ListByClientID(ctx context.Context, clientID int64, txType string, limit, offset int) ([]entity.BalanceTransaction, int64, error) {
	var items []entity.BalanceTransaction
	var total int64
	q := r.db.WithContext(ctx).Model(&entity.BalanceTransaction{}).Where("client_id = ?", clientID)
	if txType != "" {
		q = q.Where("type = ?", txType)
	}
	q.Count(&total)
	err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}
