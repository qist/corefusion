package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// ============================================================
// Referral
// ============================================================

type ReferralCodeRepository struct{ db *gorm.DB }

func NewReferralCodeRepository(db *gorm.DB) *ReferralCodeRepository {
	return &ReferralCodeRepository{db: db}
}

func (r *ReferralCodeRepository) GetByClientID(ctx context.Context, clientID int64) (*entity.ReferralCode, error) {
	var item entity.ReferralCode
	err := r.db.WithContext(ctx).Where("client_id = ?", clientID).First(&item).Error
	return &item, err
}

func (r *ReferralCodeRepository) GetByCode(ctx context.Context, code string) (*entity.ReferralCode, error) {
	var item entity.ReferralCode
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&item).Error
	return &item, err
}

func (r *ReferralCodeRepository) Create(ctx context.Context, item *entity.ReferralCode) error {
	return r.db.WithContext(ctx).Create(item).Error
}

type ReferralRecordRepository struct{ db *gorm.DB }

func NewReferralRecordRepository(db *gorm.DB) *ReferralRecordRepository {
	return &ReferralRecordRepository{db: db}
}

func (r *ReferralRecordRepository) ListByReferrerID(ctx context.Context, referrerID int64, limit, offset int) ([]entity.ReferralRecord, int64, error) {
	var items []entity.ReferralRecord
	var total int64
	q := r.db.WithContext(ctx).Model(&entity.ReferralRecord{}).Where("referrer_id = ?", referrerID)
	q.Count(&total)
	err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (r *ReferralRecordRepository) Create(ctx context.Context, item *entity.ReferralRecord) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *ReferralRecordRepository) GetByRefereeID(ctx context.Context, refereeID int64) (*entity.ReferralRecord, error) {
	var item entity.ReferralRecord
	err := r.db.WithContext(ctx).Where("referee_id = ?", refereeID).First(&item).Error
	return &item, err
}

func (r *ReferralRecordRepository) Update(ctx context.Context, item *entity.ReferralRecord) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *ReferralRecordRepository) SumAvailableCommission(ctx context.Context, referrerID int64) (int64, error) {
	var sum int64
	err := r.db.WithContext(ctx).Model(&entity.ReferralRecord{}).
		Where("referrer_id = ? AND commission_status = ?", referrerID, entity.CommissionAvailable).
		Select("COALESCE(SUM(commission_amount),0)").Scan(&sum).Error
	return sum, err
}

type ReferralWithdrawalRepository struct{ db *gorm.DB }

func NewReferralWithdrawalRepository(db *gorm.DB) *ReferralWithdrawalRepository {
	return &ReferralWithdrawalRepository{db: db}
}

func (r *ReferralWithdrawalRepository) Create(ctx context.Context, item *entity.ReferralWithdrawal) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *ReferralWithdrawalRepository) ListByClientID(ctx context.Context, clientID int64, limit, offset int) ([]entity.ReferralWithdrawal, int64, error) {
	var items []entity.ReferralWithdrawal
	var total int64
	q := r.db.WithContext(ctx).Model(&entity.ReferralWithdrawal{}).Where("client_id = ?", clientID)
	q.Count(&total)
	err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (r *ReferralWithdrawalRepository) ListAll(ctx context.Context, status string, limit, offset int) ([]entity.ReferralWithdrawal, int64, error) {
	var items []entity.ReferralWithdrawal
	var total int64
	q := r.db.WithContext(ctx).Model(&entity.ReferralWithdrawal{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	q.Count(&total)
	err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (r *ReferralWithdrawalRepository) GetByID(ctx context.Context, id int64) (*entity.ReferralWithdrawal, error) {
	var item entity.ReferralWithdrawal
	err := r.db.WithContext(ctx).First(&item, id).Error
	return &item, err
}

func (r *ReferralWithdrawalRepository) Update(ctx context.Context, item *entity.ReferralWithdrawal) error {
	return r.db.WithContext(ctx).Save(item).Error
}

// ============================================================
// Ticket
// ============================================================

type TicketRepository struct{ db *gorm.DB }

func NewTicketRepository(db *gorm.DB) *TicketRepository {
	return &TicketRepository{db: db}
}

func (r *TicketRepository) Create(ctx context.Context, item *entity.Ticket) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *TicketRepository) GetByID(ctx context.Context, id int64) (*entity.Ticket, error) {
	var item entity.Ticket
	err := r.db.WithContext(ctx).First(&item, id).Error
	return &item, err
}

func (r *TicketRepository) ListByClientID(ctx context.Context, clientID int64, status string, limit, offset int) ([]entity.Ticket, int64, error) {
	var items []entity.Ticket
	var total int64
	q := r.db.WithContext(ctx).Model(&entity.Ticket{}).Where("client_id = ?", clientID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	q.Count(&total)
	err := q.Order("updated_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (r *TicketRepository) ListAll(ctx context.Context, status, category string, limit, offset int) ([]entity.Ticket, int64, error) {
	var items []entity.Ticket
	var total int64
	q := r.db.WithContext(ctx).Model(&entity.Ticket{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if category != "" {
		q = q.Where("category = ?", category)
	}
	q.Count(&total)
	err := q.Order("updated_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (r *TicketRepository) Update(ctx context.Context, item *entity.Ticket) error {
	return r.db.WithContext(ctx).Save(item).Error
}

type TicketMessageRepository struct{ db *gorm.DB }

func NewTicketMessageRepository(db *gorm.DB) *TicketMessageRepository {
	return &TicketMessageRepository{db: db}
}

func (r *TicketMessageRepository) Create(ctx context.Context, item *entity.TicketMessage) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *TicketMessageRepository) ListByTicketID(ctx context.Context, ticketID int64) ([]entity.TicketMessage, error) {
	var items []entity.TicketMessage
	err := r.db.WithContext(ctx).Where("ticket_id = ?", ticketID).Order("created_at ASC").Find(&items).Error
	return items, err
}

// ============================================================
// Gift
// ============================================================

type ClientGiftRepository struct{ db *gorm.DB }

func NewClientGiftRepository(db *gorm.DB) *ClientGiftRepository {
	return &ClientGiftRepository{db: db}
}

func (r *ClientGiftRepository) ListAvailableByClientID(ctx context.Context, clientID int64) ([]entity.ClientGift, error) {
	var items []entity.ClientGift
	err := r.db.WithContext(ctx).
		Where("client_id = ? AND status = ? AND (expires_at IS NULL OR expires_at > ?)", clientID, entity.GiftStatusAvailable, time.Now()).
		Order("created_at DESC").Find(&items).Error
	return items, err
}

func (r *ClientGiftRepository) ListByClientID(ctx context.Context, clientID int64, limit, offset int) ([]entity.ClientGift, int64, error) {
	var items []entity.ClientGift
	var total int64
	q := r.db.WithContext(ctx).Model(&entity.ClientGift{}).Where("client_id = ?", clientID)
	q.Count(&total)
	err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (r *ClientGiftRepository) GetByID(ctx context.Context, id int64) (*entity.ClientGift, error) {
	var item entity.ClientGift
	err := r.db.WithContext(ctx).First(&item, id).Error
	return &item, err
}

func (r *ClientGiftRepository) Create(ctx context.Context, item *entity.ClientGift) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *ClientGiftRepository) CreateBatch(ctx context.Context, items []entity.ClientGift) error {
	return r.db.WithContext(ctx).Create(&items).Error
}

func (r *ClientGiftRepository) Update(ctx context.Context, item *entity.ClientGift) error {
	return r.db.WithContext(ctx).Save(item).Error
}

// ============================================================
// Article
// ============================================================

type ArticleRepository struct{ db *gorm.DB }

func NewArticleRepository(db *gorm.DB) *ArticleRepository {
	return &ArticleRepository{db: db}
}

func (r *ArticleRepository) ListPublished(ctx context.Context, category, platform string) ([]entity.Article, error) {
	var items []entity.Article
	q := r.db.WithContext(ctx).Where("is_published = ?", true)
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if platform != "" {
		q = q.Where("platform = ? OR platform = '' OR platform = 'other'", platform)
	}
	err := q.Order("sort_order ASC, created_at DESC").Find(&items).Error
	return items, err
}

func (r *ArticleRepository) GetByID(ctx context.Context, id int64) (*entity.Article, error) {
	var item entity.Article
	err := r.db.WithContext(ctx).First(&item, id).Error
	return &item, err
}

func (r *ArticleRepository) GetBySlug(ctx context.Context, slug string) (*entity.Article, error) {
	var item entity.Article
	err := r.db.WithContext(ctx).Where("slug = ?", slug).First(&item).Error
	return &item, err
}

func (r *ArticleRepository) ListAll(ctx context.Context, category string, limit, offset int) ([]entity.Article, int64, error) {
	var items []entity.Article
	var total int64
	q := r.db.WithContext(ctx).Model(&entity.Article{})
	if category != "" {
		q = q.Where("category = ?", category)
	}
	q.Count(&total)
	err := q.Order("sort_order ASC, created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (r *ArticleRepository) Create(ctx context.Context, item *entity.Article) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *ArticleRepository) Update(ctx context.Context, item *entity.Article) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *ArticleRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.Article{}, id).Error
}

// ============================================================
// Shared Account
// ============================================================

type SharedAccountRepository struct{ db *gorm.DB }

func NewSharedAccountRepository(db *gorm.DB) *SharedAccountRepository {
	return &SharedAccountRepository{db: db}
}

func (r *SharedAccountRepository) ListEnabled(ctx context.Context) ([]entity.SharedAccount, error) {
	var items []entity.SharedAccount
	err := r.db.WithContext(ctx).Where("enabled = ?", true).Order("sort_order ASC").Find(&items).Error
	return items, err
}

func (r *SharedAccountRepository) ListAll(ctx context.Context, limit, offset int) ([]entity.SharedAccount, int64, error) {
	var items []entity.SharedAccount
	var total int64
	q := r.db.WithContext(ctx).Model(&entity.SharedAccount{})
	q.Count(&total)
	err := q.Order("sort_order ASC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (r *SharedAccountRepository) GetByID(ctx context.Context, id int64) (*entity.SharedAccount, error) {
	var item entity.SharedAccount
	err := r.db.WithContext(ctx).First(&item, id).Error
	return &item, err
}

func (r *SharedAccountRepository) Create(ctx context.Context, item *entity.SharedAccount) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *SharedAccountRepository) Update(ctx context.Context, item *entity.SharedAccount) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *SharedAccountRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.SharedAccount{}, id).Error
}

type SharedAccountRepairLogRepository struct{ db *gorm.DB }

func NewSharedAccountRepairLogRepository(db *gorm.DB) *SharedAccountRepairLogRepository {
	return &SharedAccountRepairLogRepository{db: db}
}

func (r *SharedAccountRepairLogRepository) Create(ctx context.Context, item *entity.SharedAccountRepairLog) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *SharedAccountRepairLogRepository) ListByAccountID(ctx context.Context, accountID int64, limit, offset int) ([]entity.SharedAccountRepairLog, int64, error) {
	var items []entity.SharedAccountRepairLog
	var total int64
	q := r.db.WithContext(ctx).Model(&entity.SharedAccountRepairLog{}).Where("account_id = ?", accountID)
	q.Count(&total)
	err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (r *SharedAccountRepairLogRepository) GetLastByClientAndAccount(ctx context.Context, clientID, accountID int64) (*entity.SharedAccountRepairLog, error) {
	var item entity.SharedAccountRepairLog
	err := r.db.WithContext(ctx).
		Where("client_id = ? AND account_id = ?", clientID, accountID).
		Order("created_at DESC").First(&item).Error
	return &item, err
}

func (r *SharedAccountRepairLogRepository) Update(ctx context.Context, item *entity.SharedAccountRepairLog) error {
	return r.db.WithContext(ctx).Save(item).Error
}

// ============================================================
// OAuth Binding
// ============================================================

type ClientOAuthBindingRepository struct{ db *gorm.DB }

func NewClientOAuthBindingRepository(db *gorm.DB) *ClientOAuthBindingRepository {
	return &ClientOAuthBindingRepository{db: db}
}

func (r *ClientOAuthBindingRepository) ListByClientID(ctx context.Context, clientID int64) ([]entity.ClientOAuthBinding, error) {
	var items []entity.ClientOAuthBinding
	err := r.db.WithContext(ctx).Where("client_id = ?", clientID).Find(&items).Error
	return items, err
}

func (r *ClientOAuthBindingRepository) GetByProvider(ctx context.Context, clientID int64, provider string) (*entity.ClientOAuthBinding, error) {
	var item entity.ClientOAuthBinding
	err := r.db.WithContext(ctx).Where("client_id = ? AND provider = ?", clientID, provider).First(&item).Error
	return &item, err
}

func (r *ClientOAuthBindingRepository) GetByProviderUserID(ctx context.Context, provider, providerUserID string) (*entity.ClientOAuthBinding, error) {
	var item entity.ClientOAuthBinding
	err := r.db.WithContext(ctx).Where("provider = ? AND provider_user_id = ?", provider, providerUserID).First(&item).Error
	return &item, err
}

func (r *ClientOAuthBindingRepository) Create(ctx context.Context, item *entity.ClientOAuthBinding) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *ClientOAuthBindingRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.ClientOAuthBinding{}, id).Error
}

// ============================================================
// Self-Unban
// ============================================================

type ClientSelfUnbanLogRepository struct{ db *gorm.DB }

func NewClientSelfUnbanLogRepository(db *gorm.DB) *ClientSelfUnbanLogRepository {
	return &ClientSelfUnbanLogRepository{db: db}
}

func (r *ClientSelfUnbanLogRepository) Create(ctx context.Context, item *entity.ClientSelfUnbanLog) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *ClientSelfUnbanLogRepository) GetLastByClientID(ctx context.Context, clientID int64) (*entity.ClientSelfUnbanLog, error) {
	var item entity.ClientSelfUnbanLog
	err := r.db.WithContext(ctx).Where("client_id = ?", clientID).Order("created_at DESC").First(&item).Error
	return &item, err
}

// ============================================================
// IP Whitelist Entry
// ============================================================

type IPWhitelistEntryRepository struct{ db *gorm.DB }

func NewIPWhitelistEntryRepository(db *gorm.DB) *IPWhitelistEntryRepository {
	return &IPWhitelistEntryRepository{db: db}
}

func (r *IPWhitelistEntryRepository) ListByClientID(ctx context.Context, clientID int64) ([]entity.IPWhitelistEntry, error) {
	var items []entity.IPWhitelistEntry
	err := r.db.WithContext(ctx).Where("client_id = ?", clientID).Order("created_at DESC").Find(&items).Error
	return items, err
}

func (r *IPWhitelistEntryRepository) Create(ctx context.Context, item *entity.IPWhitelistEntry) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *IPWhitelistEntryRepository) DeleteByID(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.IPWhitelistEntry{}, id).Error
}

func (r *IPWhitelistEntryRepository) GetByClientAndIP(ctx context.Context, clientID int64, ip string) (*entity.IPWhitelistEntry, error) {
	var item entity.IPWhitelistEntry
	err := r.db.WithContext(ctx).Where("client_id = ? AND ip_address = ?", clientID, ip).First(&item).Error
	return &item, err
}
